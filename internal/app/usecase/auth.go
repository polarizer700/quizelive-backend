package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

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
	users      repository.UserRepository
	tokens     repository.RefreshTokenRepository
	hasher     auth.PasswordHasher
	issuer     auth.TokenIssuer
	refreshTTL time.Duration
	now        func() time.Time
}

func NewAuth(
	users repository.UserRepository,
	tokens repository.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	issuer auth.TokenIssuer,
	refreshTTL time.Duration,
) *Auth {
	return &Auth{
		users:      users,
		tokens:     tokens,
		hasher:     hasher,
		issuer:     issuer,
		refreshTTL: refreshTTL,
		now:        time.Now,
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

	return a.startSession(ctx, user)
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
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, time.Time{}, err
	}

	raw := base64.RawURLEncoding.EncodeToString(buf)

	return raw, hashRefresh(raw), now.Add(a.refreshTTL), nil
}

func hashRefresh(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	out := make([]byte, len(sum))
	copy(out, sum[:])

	return out
}
