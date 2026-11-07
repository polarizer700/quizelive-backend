package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestPrepareSingleQuestion(t *testing.T) {
	t.Parallel()

	got, err := PrepareQuestion(Question{
		Type:         QuestionTypeSingle,
		Text:         "  Столица? ",
		TimeLimitSec: 20,
		Points:       1000,
		Options: []AnswerOption{
			{Text: " Берлин ", IsCorrect: false},
			{Text: "Париж", IsCorrect: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Text != "Столица?" || got.Options[0].Position != 1 || !got.Options[1].IsCorrect {
		t.Fatalf("unexpected question: %+v", got)
	}
}

func TestPrepareSingleQuestionRejectsTwoCorrect(t *testing.T) {
	t.Parallel()

	_, err := PrepareQuestion(Question{
		Type:         QuestionTypeSingle,
		Text:         "Вопрос",
		TimeLimitSec: 20,
		Points:       100,
		Options: []AnswerOption{
			{Text: "A", IsCorrect: true},
			{Text: "B", IsCorrect: true},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPrepareTextQuestionNormalizesAnswers(t *testing.T) {
	t.Parallel()

	got, err := PrepareQuestion(Question{
		Type:         QuestionTypeText,
		Text:         "Год",
		TimeLimitSec: 30,
		Points:       500,
		TextAnswers:  []string{"  Париж ", "париж", "PARIS"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.TextAnswers) != 2 || got.TextAnswers[0] != "париж" || got.TextAnswers[1] != "paris" {
		t.Fatalf("answers: %#v", got.TextAnswers)
	}
}

func TestCanIssueHostToken(t *testing.T) {
	t.Parallel()

	owner := mustID(t, "11111111-1111-1111-1111-111111111111")
	other := mustID(t, "22222222-2222-2222-2222-222222222222")

	if CanIssueHostToken(RoleHost, other, owner, QuizStatusDraft) {
		t.Fatal("host must not open a draft")
	}

	if !CanIssueHostToken(RoleHost, other, owner, QuizStatusPublished) {
		t.Fatal("host can open a published quiz")
	}

	if CanIssueHostToken(RoleEditor, other, owner, QuizStatusPublished) {
		t.Fatal("editor cannot host someone else's quiz")
	}
}

func mustID(t *testing.T, raw string) uuid.UUID {
	t.Helper()

	parsed, err := uuid.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	return parsed
}
