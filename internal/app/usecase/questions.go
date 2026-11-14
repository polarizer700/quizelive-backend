package usecase

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/media"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

const maxQuestionImageBytes = 5 << 20

type Questions struct {
	quizzes   repository.QuizRepository
	questions repository.QuestionRepository
	media     media.Store
}

func NewQuestions(quizzes repository.QuizRepository, questions repository.QuestionRepository, store media.Store) *Questions {
	return &Questions{quizzes: quizzes, questions: questions, media: store}
}

func (s *Questions) List(ctx context.Context, quizID uuid.UUID) ([]entity.Question, error) {
	if _, _, err := s.ensureRead(ctx, quizID); err != nil {
		return nil, err
	}

	rows, err := s.questions.ListByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}

	return s.withURLs(rows), nil
}

func (s *Questions) Get(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, error) {
	if _, _, err := s.ensureRead(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	question, err := s.questions.Get(ctx, quizID, questionID)
	if err != nil {
		return entity.Question{}, err
	}

	return s.withURL(question), nil
}

func (s *Questions) Create(ctx context.Context, quizID uuid.UUID, input entity.Question) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	prepared, err := entity.PrepareQuestion(input)
	if err != nil {
		return entity.Question{}, err
	}

	prepared.QuizID = quizID

	question, err := s.questions.Create(ctx, prepared)
	if err != nil {
		return entity.Question{}, err
	}

	return s.withURL(question), nil
}

func (s *Questions) Update(ctx context.Context, quizID, questionID uuid.UUID, input entity.Question) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	current, err := s.questions.Get(ctx, quizID, questionID)
	if err != nil {
		return entity.Question{}, err
	}

	prepared, err := entity.PrepareQuestion(input)
	if err != nil {
		return entity.Question{}, err
	}

	prepared.ID = current.ID
	prepared.QuizID = current.QuizID
	prepared.Position = current.Position
	prepared.ImageKey = current.ImageKey
	prepared.ImageContentType = current.ImageContentType
	prepared.ImageSizeBytes = current.ImageSizeBytes
	prepared.ImageURL = current.ImageURL

	question, err := s.questions.Update(ctx, prepared)
	if err != nil {
		return entity.Question{}, err
	}

	return s.withURL(question), nil
}

func (s *Questions) UploadImage(ctx context.Context, quizID, questionID uuid.UUID, contentType string, body io.Reader, size int64) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	contentType = strings.ToLower(strings.TrimSpace(contentType))
	ext, ok := imageExt(contentType)
	if !ok {
		return entity.Question{}, domain.FieldErrorf("file", "invalid", "image must be jpeg, png or webp")
	}
	if size <= 0 || size > maxQuestionImageBytes {
		return entity.Question{}, domain.FieldErrorf("file", "invalid", "image must be up to 5 MB")
	}

	current, err := s.questions.Get(ctx, quizID, questionID)
	if err != nil {
		return entity.Question{}, err
	}

	key := path.Join("questions", quizID.String(), questionID.String()+"."+ext)
	if err = s.media.Put(ctx, key, contentType, body, size); err != nil {
		return entity.Question{}, err
	}

	question, err := s.questions.SetImage(ctx, quizID, questionID, key, contentType, size)
	if err != nil {
		_ = s.media.Delete(ctx, key)
		return entity.Question{}, err
	}

	if current.ImageKey != "" && current.ImageKey != key {
		_ = s.media.Delete(ctx, current.ImageKey)
	}

	return s.withURL(question), nil
}

func (s *Questions) DeleteImage(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	question, oldKey, err := s.questions.ClearImage(ctx, quizID, questionID)
	if err != nil {
		return entity.Question{}, err
	}
	if oldKey != "" {
		_ = s.media.Delete(ctx, oldKey)
	}

	return s.withURL(question), nil
}

func (s *Questions) Delete(ctx context.Context, quizID, questionID uuid.UUID) error {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return err
	}

	current, err := s.questions.Get(ctx, quizID, questionID)
	if err != nil {
		return err
	}

	if err = s.questions.Delete(ctx, quizID, questionID); err != nil {
		return err
	}
	if current.ImageKey != "" {
		_ = s.media.Delete(ctx, current.ImageKey)
	}

	return nil
}

func (s *Questions) withURLs(rows []entity.Question) []entity.Question {
	out := make([]entity.Question, len(rows))
	for i, row := range rows {
		out[i] = s.withURL(row)
	}

	return out
}

func (s *Questions) withURL(question entity.Question) entity.Question {
	if question.ImageKey != "" && s.media != nil {
		question.ImageURL = s.media.URL(question.ImageKey)
	}

	return question
}

func imageExt(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/webp":
		return "webp", true
	default:
		return "", false
	}
}

func ImageContentType(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func (s *Questions) Reorder(ctx context.Context, quizID uuid.UUID, ids []uuid.UUID) ([]entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return nil, err
	}

	current, err := s.questions.ListByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}

	if len(ids) != len(current) || len(ids) == 0 && len(current) != 0 {
		return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
	}

	existing := make(map[uuid.UUID]struct{}, len(current))
	for _, question := range current {
		existing[question.ID] = struct{}{}
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
		}

		if _, ok := seen[id]; ok {
			return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
		}

		seen[id] = struct{}{}
	}

	if err = s.questions.Reorder(ctx, quizID, ids); err != nil {
		return nil, err
	}

	rows, err := s.questions.ListByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}

	return s.withURLs(rows), nil
}

func (s *Questions) ensureRead(ctx context.Context, quizID uuid.UUID) (entity.Quiz, bool, error) {
	_, quiz, err := readableQuiz(ctx, s.quizzes, quizID)
	if err != nil {
		return entity.Quiz{}, false, err
	}

	return quiz, true, nil
}

func (s *Questions) ensureEdit(ctx context.Context, quizID uuid.UUID) (entity.Quiz, bool, error) {
	actor, quiz, err := readableQuiz(ctx, s.quizzes, quizID)
	if err != nil {
		return entity.Quiz{}, false, err
	}

	if !entity.CanEditQuiz(actor.Role, actor.UserID, quiz.OwnerID) {
		return entity.Quiz{}, false, domain.ErrForbidden
	}

	return quiz, true, nil
}
