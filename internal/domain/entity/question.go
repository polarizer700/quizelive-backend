package entity

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

type QuestionType string

const (
	QuestionTypeSingle   QuestionType = "single"
	QuestionTypeMultiple QuestionType = "multiple"
	QuestionTypeText     QuestionType = "text"
)

func (t QuestionType) Valid() bool {
	switch t {
	case QuestionTypeSingle, QuestionTypeMultiple, QuestionTypeText:
		return true
	default:
		return false
	}
}

type AnswerOption struct {
	ID        uuid.UUID
	Position  int
	Text      string
	IsCorrect bool
}

type Question struct {
	ID           uuid.UUID
	QuizID       uuid.UUID
	Position     int
	Type         QuestionType
	Text         string
	TimeLimitSec int
	Points       int
	Options      []AnswerOption
	TextAnswers  []string
}

// NormalizeTextAnswer приводит вписываемый ответ к виду, в котором он хранится и сравнивается.
func NormalizeTextAnswer(raw string) string {
	fields := strings.FieldsFunc(strings.TrimSpace(raw), unicode.IsSpace)
	return strings.ToLower(strings.Join(fields, " "))
}

// PrepareQuestion проверяет агрегат и заполняет позиции вариантов и нормализованные ответы.
func PrepareQuestion(q Question) (Question, error) {
	q.Text = strings.TrimSpace(q.Text)
	if n := utf8.RuneCountInString(q.Text); n < 1 || n > 2000 {
		return Question{}, domain.FieldErrorf("text", "invalid", "text must be 1 to 2000 characters")
	}

	if !q.Type.Valid() {
		return Question{}, domain.FieldErrorf("type", "invalid", "type must be single, multiple or text")
	}

	if q.TimeLimitSec < 5 || q.TimeLimitSec > 300 {
		return Question{}, domain.FieldErrorf("time_limit_sec", "invalid", "time limit must be 5 to 300 seconds")
	}

	if q.Points < 1 || q.Points > 10000 {
		return Question{}, domain.FieldErrorf("points", "invalid", "points must be 1 to 10000")
	}

	switch q.Type {
	case QuestionTypeText:
		return prepareTextQuestion(q)
	case QuestionTypeSingle, QuestionTypeMultiple:
		return prepareChoiceQuestion(q)
	default:
		return Question{}, domain.FieldErrorf("type", "invalid", "type must be single, multiple or text")
	}
}

func prepareTextQuestion(q Question) (Question, error) {
	if len(q.Options) > 0 {
		return Question{}, domain.FieldErrorf("options", "invalid", "text question does not use options")
	}

	seen := make(map[string]struct{}, len(q.TextAnswers))
	normalized := make([]string, 0, len(q.TextAnswers))

	for _, raw := range q.TextAnswers {
		value := NormalizeTextAnswer(raw)
		if value == "" || utf8.RuneCountInString(value) > 200 {
			return Question{}, domain.FieldErrorf("text_answers", "invalid", "each text answer must be 1 to 200 characters")
		}

		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	if len(normalized) < 1 || len(normalized) > 20 {
		return Question{}, domain.FieldErrorf("text_answers", "invalid", "text question needs 1 to 20 answers")
	}

	q.TextAnswers = normalized
	q.Options = nil

	return q, nil
}

func prepareChoiceQuestion(q Question) (Question, error) {
	if len(q.TextAnswers) > 0 {
		return Question{}, domain.FieldErrorf("text_answers", "invalid", "choice question does not use text answers")
	}

	if len(q.Options) < 2 || len(q.Options) > 8 {
		return Question{}, domain.FieldErrorf("options", "invalid", "choice question needs 2 to 8 options")
	}

	correct := 0

	for i := range q.Options {
		q.Options[i].Text = strings.TrimSpace(q.Options[i].Text)
		if n := utf8.RuneCountInString(q.Options[i].Text); n < 1 || n > 500 {
			return Question{}, domain.FieldErrorf("options", "invalid", "each option must be 1 to 500 characters")
		}

		q.Options[i].Position = i + 1
		if q.Options[i].IsCorrect {
			correct++
		}
	}

	if q.Type == QuestionTypeSingle && correct != 1 {
		return Question{}, domain.FieldErrorf("options", "invalid", "single question needs exactly one correct option")
	}

	if q.Type == QuestionTypeMultiple && correct < 1 {
		return Question{}, domain.FieldErrorf("options", "invalid", "multiple question needs at least one correct option")
	}

	q.TextAnswers = nil

	return q, nil
}
