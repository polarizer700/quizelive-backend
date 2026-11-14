package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Session struct {
	AccessToken      string
	ExpiresIn        time.Duration
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             entity.User
}

type Auth struct {
	users         repository.UserRepository
	tokens        repository.RefreshTokenRepository
	verifications repository.EmailVerificationRepository
	resets        repository.PasswordResetRepository
	hasher        auth.PasswordHasher
	issuer        auth.TokenIssuer
	mailer        auth.Mailer
	refreshTTL    time.Duration
	verifyTTL     time.Duration
	resetTTL      time.Duration
	publicURL     string
	now           func() time.Time
}

func NewAuth(
	users repository.UserRepository,
	tokens repository.RefreshTokenRepository,
	verifications repository.EmailVerificationRepository,
	resets repository.PasswordResetRepository,
	hasher auth.PasswordHasher,
	issuer auth.TokenIssuer,
	mailer auth.Mailer,
	refreshTTL time.Duration,
	verifyTTL time.Duration,
	publicURL string,
) *Auth {
	if verifyTTL <= 0 {
		verifyTTL = 24 * time.Hour
	}

	return &Auth{
		users:         users,
		tokens:        tokens,
		verifications: verifications,
		resets:        resets,
		hasher:        hasher,
		issuer:        issuer,
		mailer:        mailer,
		refreshTTL:    refreshTTL,
		verifyTTL:     verifyTTL,
		resetTTL:      time.Hour,
		publicURL:     strings.TrimRight(publicURL, "/"),
		now:           time.Now,
	}
}

func (a *Auth) Login(ctx context.Context, emailRaw, password string) (Session, error) {
	email, err := entity.NormalizeEmail(emailRaw)
	if err != nil || password == "" {
		return Session{}, domain.ErrUnauthorized
	}

	user, err := a.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return Session{}, err
	}

	hash := ""
	if err == nil {
		hash = user.PasswordHash
	}

	cmpErr := a.hasher.Compare(hash, password)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(cmpErr, domain.ErrUnauthorized) {
		return Session{}, domain.ErrUnauthorized
	}

	if cmpErr != nil {
		return Session{}, cmpErr
	}

	if user.EmailVerifiedAt == nil {
		if err := a.sendVerificationEmail(ctx, user.ID, email); err != nil {
			slog.Error("resend verification on login", "category", "mail", "email", email, "error", err)
		}
		return Session{}, &domain.EmailUnverifiedError{}
	}

	return a.startSession(ctx, user)
}

func (a *Auth) Register(ctx context.Context, emailRaw, password string) error {
	email, err := entity.NormalizeEmail(emailRaw)
	if err != nil {
		return err
	}

	if err = entity.ValidatePassword(password); err != nil {
		return err
	}

	hash, err := a.hasher.Hash(password)
	if err != nil {
		return err
	}

	user, err := a.users.GetByEmail(ctx, email)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		user, err = a.users.Create(ctx, email, hash, entity.RoleEditor, nil)
	case err != nil:
		return err
	case user.EmailVerifiedAt != nil:
		return &domain.ConflictError{Message: "email already exists"}
	default:
		user.PasswordHash = hash
		user, err = a.users.Update(ctx, user)
	}

	if err != nil {
		return err
	}

	return a.sendVerificationEmail(ctx, user.ID, email)
}

func (a *Auth) sendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	raw, tokenHash, err := newSecret()
	if err != nil {
		return err
	}

	if err = a.verifications.ReplacePending(ctx, userID, tokenHash, a.now().Add(a.verifyTTL)); err != nil {
		return err
	}

	link := a.publicURL + "/verify-email?token=" + raw
	text := "Подтвердите почту, чтобы войти в QuizeLive:\n\n" + link + "\n\nСсылка действует 24 часа. Если вы не регистрировались, проигнорируйте письмо."

	return a.mailer.Send(ctx, auth.Message{
		To:      email,
		Subject: "Подтвердите почту QuizeLive",
		Text:    text,
	})
}

func (a *Auth) VerifyEmail(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return domain.FieldErrorf("token", "invalid", "token is invalid or expired")
	}

	if _, err := a.verifications.Consume(ctx, hashRefresh(rawToken), a.now()); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.FieldErrorf("token", "invalid", "token is invalid or expired")
		}

		return err
	}

	return nil
}

func (a *Auth) RequestPasswordReset(ctx context.Context, emailRaw string) error {
	email, err := entity.NormalizeEmail(emailRaw)
	if err != nil {
		return err
	}

	user, err := a.users.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	raw, tokenHash, err := newSecret()
	if err != nil {
		return err
	}

	if err = a.resets.ReplacePending(ctx, user.ID, tokenHash, a.now().Add(a.resetTTL)); err != nil {
		return err
	}

	link := a.publicURL + "/reset-password?token=" + raw
	text := "Сброс пароля QuizeLive:\n\n" + link + "\n\nСсылка действует 1 час. Если вы не запрашивали сброс, проигнорируйте письмо."

	return a.mailer.Send(ctx, auth.Message{
		To:      email,
		Subject: "Сброс пароля QuizeLive",
		Text:    text,
	})
}

func (a *Auth) ConfirmPasswordReset(ctx context.Context, rawToken, password string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return domain.FieldErrorf("token", "invalid", "token is invalid or expired")
	}

	if err := entity.ValidatePassword(password); err != nil {
		return err
	}

	now := a.now()
	userID, err := a.resets.Consume(ctx, hashRefresh(rawToken), now)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.FieldErrorf("token", "invalid", "token is invalid or expired")
		}
		return err
	}

	user, err := a.users.GetByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.FieldErrorf("token", "invalid", "token is invalid or expired")
	}
	if err != nil {
		return err
	}

	hash, err := a.hasher.Hash(password)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	if _, err = a.users.Update(ctx, user); err != nil {
		return err
	}

	if user.EmailVerifiedAt == nil {
		if err = a.users.MarkEmailVerified(ctx, user.ID, now); err != nil {
			return err
		}
	}

	return a.tokens.RevokeAllForUser(ctx, user.ID, now)
}

func (a *Auth) Refresh(ctx context.Context, rawToken string) (Session, error) {
	if rawToken == "" {
		return Session{}, domain.ErrUnauthorized
	}

	stored, err := a.tokens.FindByHash(ctx, hashRefresh(rawToken))
	if errors.Is(err, domain.ErrNotFound) {
		return Session{}, domain.ErrUnauthorized
	}

	if err != nil {
		return Session{}, err
	}

	now := a.now()
	if stored.RevokedAt != nil {
		if err = a.tokens.RevokeAllForUser(ctx, stored.UserID, now); err != nil {
			return Session{}, err
		}

		return Session{}, domain.ErrUnauthorized
	}

	if !stored.ExpiresAt.After(now) {
		return Session{}, domain.ErrUnauthorized
	}

	user, err := a.users.GetByID(ctx, stored.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return Session{}, domain.ErrUnauthorized
	}

	if err != nil {
		return Session{}, err
	}

	raw, hash, expiresAt, err := a.newRefresh(now)
	if err != nil {
		return Session{}, err
	}

	revoked, err := a.tokens.RevokeIfActive(ctx, stored.ID, now)
	if err != nil {
		return Session{}, err
	}

	if !revoked {
		return Session{}, domain.ErrUnauthorized
	}

	if _, err = a.tokens.Insert(ctx, user.ID, hash, expiresAt); err != nil {
		return Session{}, err
	}

	return a.accessSession(user, raw, expiresAt)
}

func (a *Auth) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}

	stored, err := a.tokens.FindByHash(ctx, hashRefresh(rawToken))
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	_, err = a.tokens.RevokeIfActive(ctx, stored.ID, a.now())

	return err
}

func (a *Auth) startSession(ctx context.Context, user entity.User) (Session, error) {
	now := a.now()
	raw, hash, expiresAt, err := a.newRefresh(now)
	if err != nil {
		return Session{}, err
	}

	if _, err = a.tokens.Insert(ctx, user.ID, hash, expiresAt); err != nil {
		return Session{}, err
	}

	return a.accessSession(user, raw, expiresAt)
}

func (a *Auth) accessSession(user entity.User, refresh string, refreshExpires time.Time) (Session, error) {
	access, expiresIn, err := a.issuer.IssueAccess(user)
	if err != nil {
		return Session{}, err
	}

	user.PasswordHash = ""

	return Session{
		AccessToken:      access,
		ExpiresIn:        expiresIn,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExpires,
		User:             user,
	}, nil
}

func (a *Auth) newRefresh(now time.Time) (string, []byte, time.Time, error) {
	raw, sum, err := newSecret()
	if err != nil {
		return "", nil, time.Time{}, err
	}

	return raw, sum, now.Add(a.refreshTTL), nil
}

func newSecret() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}

	raw := base64.RawURLEncoding.EncodeToString(buf)

	return raw, hashRefresh(raw), nil
}

func hashRefresh(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	out := make([]byte, len(sum))
	copy(out, sum[:])

	return out
}
