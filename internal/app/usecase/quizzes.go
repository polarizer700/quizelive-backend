package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Quizzes struct {
	quizzes repository.QuizRepository
	issuer  auth.TokenIssuer
}

func NewQuizzes(quizzes repository.QuizRepository, issuer auth.TokenIssuer) *Quizzes {
	return &Quizzes{quizzes: quizzes, issuer: issuer}
}

func (q *Quizzes) Create(ctx context.Context, title, description string, gameType entity.QuizGameType, breakAfterRound int) (entity.Quiz, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return entity.Quiz{}, err
	}

	if !actor.Role.CanEditQuizzes() {
		return entity.Quiz{}, domain.ErrForbidden
	}

	title, description, err = entity.NormalizeQuizText(title, description)
	if err != nil {
		return entity.Quiz{}, err
	}

	if gameType == "" {
		gameType = entity.GameTypeQuiz
	}

	if err := entity.NormalizeGameConfig(gameType, breakAfterRound); err != nil {
		return entity.Quiz{}, err
	}

	return q.quizzes.Create(ctx, entity.Quiz{
		Title:           title,
		Description:     description,
		OwnerID:         actor.UserID,
		Status:          entity.QuizStatusDraft,
		GameType:        gameType,
		BreakAfterRound: breakAfterRound,
	})
}

func (q *Quizzes) List(ctx context.Context, statusRaw string, limit, offset int) ([]entity.Quiz, int64, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return nil, 0, err
	}

	page, err := normalizePage(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	filter := repository.QuizFilter{Page: page}

	switch actor.Role {
	case entity.RoleAdmin:
		status, statusErr := optionalStatus(statusRaw)
		if statusErr != nil {
			return nil, 0, statusErr
		}

		filter.Status = status
	case entity.RoleEditor:
		filter.OwnerID = &actor.UserID

		status, statusErr := optionalStatus(statusRaw)
		if statusErr != nil {
			return nil, 0, statusErr
		}

		filter.Status = status
	case entity.RoleHost:
		published := entity.QuizStatusPublished
		filter.Status = &published
	default:
		return nil, 0, domain.ErrForbidden
	}

	return q.quizzes.List(ctx, filter)
}

func (q *Quizzes) Get(ctx context.Context, id uuid.UUID) (entity.Quiz, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return entity.Quiz{}, err
	}

	quiz, err := q.quizzes.Get(ctx, id)
	if err != nil {
		return entity.Quiz{}, err
	}

	if !entity.CanReadQuiz(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return entity.Quiz{}, domain.ErrNotFound
	}

	return quiz, nil
}

type QuizUpdate struct {
	Title           *string
	Description     *string
	Status          *entity.QuizStatus
	GameType        *entity.QuizGameType
	BreakAfterRound *int
}

func (q *Quizzes) Update(ctx context.Context, id uuid.UUID, patch QuizUpdate) (entity.Quiz, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return entity.Quiz{}, err
	}

	quiz, err := q.quizzes.Get(ctx, id)
	if err != nil {
		return entity.Quiz{}, err
	}

	if !entity.CanReadQuiz(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return entity.Quiz{}, domain.ErrNotFound
	}

	if !entity.CanEditQuiz(actor.Role, actor.UserID, quiz.OwnerID) {
		return entity.Quiz{}, domain.ErrForbidden
	}

	title := quiz.Title
	if patch.Title != nil {
		title = *patch.Title
	}

	description := quiz.Description
	if patch.Description != nil {
		description = *patch.Description
	}

	title, description, err = entity.NormalizeQuizText(title, description)
	if err != nil {
		return entity.Quiz{}, err
	}

	quiz.Title = title
	quiz.Description = description

	if patch.Status != nil {
		if !patch.Status.Valid() {
			return entity.Quiz{}, domain.FieldErrorf("status", "invalid", "status must be draft or published")
		}

		quiz.Status = *patch.Status
	}

	gameType := quiz.GameType
	if patch.GameType != nil {
		gameType = *patch.GameType
	}

	breakAfterRound := quiz.BreakAfterRound
	if patch.BreakAfterRound != nil {
		breakAfterRound = *patch.BreakAfterRound
	}

	if err := entity.NormalizeGameConfig(gameType, breakAfterRound); err != nil {
		return entity.Quiz{}, err
	}

	quiz.GameType = gameType
	quiz.BreakAfterRound = breakAfterRound

	return q.quizzes.Update(ctx, quiz)
}

func (q *Quizzes) Delete(ctx context.Context, id uuid.UUID) error {
	actor, err := identity.Require(ctx)
	if err != nil {
		return err
	}

	quiz, err := q.quizzes.Get(ctx, id)
	if err != nil {
		return err
	}

	if !entity.CanReadQuiz(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return domain.ErrNotFound
	}

	if !entity.CanEditQuiz(actor.Role, actor.UserID, quiz.OwnerID) {
		return domain.ErrForbidden
	}

	return q.quizzes.Delete(ctx, id)
}

type HostToken struct {
	Token     string
	ExpiresIn int
}

func (q *Quizzes) IssueHostToken(ctx context.Context, id uuid.UUID) (HostToken, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return HostToken{}, err
	}

	quiz, err := q.quizzes.Get(ctx, id)
	if err != nil {
		return HostToken{}, err
	}

	if !entity.CanReadQuiz(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return HostToken{}, domain.ErrNotFound
	}

	if quiz.Status != entity.QuizStatusPublished {
		return HostToken{}, &domain.ConflictError{Message: "quiz is not published"}
	}

	if !entity.CanIssueHostToken(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return HostToken{}, domain.ErrForbidden
	}

	token, expiresIn, err := q.issuer.IssueHost(entity.User{ID: actor.UserID, Role: actor.Role}, quiz.ID)
	if err != nil {
		return HostToken{}, err
	}

	return HostToken{Token: token, ExpiresIn: int(expiresIn.Seconds())}, nil
}

func optionalStatus(raw string) (*entity.QuizStatus, error) {
	if raw == "" {
		return nil, nil
	}

	status := entity.QuizStatus(raw)
	if !status.Valid() {
		return nil, domain.FieldErrorf("status", "invalid", "status must be draft or published")
	}

	return &status, nil
}

func readableQuiz(ctx context.Context, quizzes repository.QuizRepository, id uuid.UUID) (identity.Identity, entity.Quiz, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return identity.Identity{}, entity.Quiz{}, err
	}

	quiz, err := quizzes.Get(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return identity.Identity{}, entity.Quiz{}, domain.ErrNotFound
	}

	if err != nil {
		return identity.Identity{}, entity.Quiz{}, err
	}

	if !entity.CanReadQuiz(actor.Role, actor.UserID, quiz.OwnerID, quiz.Status) {
		return identity.Identity{}, entity.Quiz{}, domain.ErrNotFound
	}

	return actor, quiz, nil
}
