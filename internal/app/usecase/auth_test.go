package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

func TestRegisterThenVerify(t *testing.T) {
	t.Parallel()

	users := &fakeUsers{byEmail: map[string]entity.User{}, byID: map[uuid.UUID]entity.User{}}
	verifications := &fakeVerifications{users: users}
	mailbox := &fakeMailer{}
	authn := NewAuth(users, &fakeTokens{}, verifications, fakeHasher{}, fakeIssuer{}, mailbox, time.Hour, time.Hour, "http://localhost:5173")
	now := time.Unix(1_700_000_000, 0)
	authn.now = func() time.Time { return now }

	if err := authn.Register(context.Background(), "Editor@QuizeLive.test", "password123"); err != nil {
		t.Fatal(err)
	}

	if _, err := authn.Login(context.Background(), "editor@quizelive.test", "password123"); !errors.As(err, new(*domain.EmailUnverifiedError)) {
		t.Fatalf("unverified login: %v", err)
	}

	_, token, ok := strings.Cut(mailbox.text, "token=")
	if !ok {
		t.Fatalf("mail has no token: %s", mailbox.text)
	}
	token, _, _ = strings.Cut(token, "\n")

	if err := authn.VerifyEmail(context.Background(), token); err != nil {
		t.Fatal(err)
	}

	if _, err := authn.Login(context.Background(), "editor@quizelive.test", "password123"); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshReusesRevokedToken(t *testing.T) {
	t.Parallel()

	verified := time.Unix(1_700_000_000, 0)
	user := entity.User{
		ID:              uuid.New(),
		Email:           "host@quizelive.test",
		Role:            entity.RoleHost,
		PasswordHash:    "hash:password123",
		EmailVerifiedAt: &verified,
	}
	store := &fakeTokens{}
	users := &fakeUsers{byEmail: map[string]entity.User{user.Email: user}, byID: map[uuid.UUID]entity.User{user.ID: user}}
	authn := NewAuth(users, store, &fakeVerifications{}, fakeHasher{}, fakeIssuer{}, &fakeMailer{}, time.Hour, time.Hour, "http://localhost:5173")
	authn.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	session, err := authn.Login(context.Background(), user.Email, "password123")
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := authn.Refresh(context.Background(), session.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = authn.Refresh(context.Background(), session.RefreshToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("reuse: got %v", err)
	}

	if !store.allRevoked {
		t.Fatal("reuse must revoke the token family")
	}

	if rotated.AccessToken == "" || rotated.User.PasswordHash != "" {
		t.Fatalf("rotated session leaked or empty: %+v", rotated)
	}
}

type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) { return "hash:" + password, nil }

func (fakeHasher) Compare(hash, password string) error {
	if hash == "hash:"+password {
		return nil
	}

	return domain.ErrUnauthorized
}

type fakeIssuer struct{}

func (fakeIssuer) IssueAccess(user entity.User) (string, time.Duration, error) {
	return "access:" + user.ID.String(), time.Minute, nil
}

func (fakeIssuer) ParseAccess(string) (identity.Identity, error) {
	return identity.Identity{}, domain.ErrUnauthorized
}

func (fakeIssuer) IssueHost(entity.User, uuid.UUID) (string, time.Duration, error) {
	return "host", time.Minute, nil
}

type fakeUsers struct {
	byEmail map[string]entity.User
	byID    map[uuid.UUID]entity.User
}

func (f *fakeUsers) Create(_ context.Context, email, hash string, role entity.Role, verifiedAt *time.Time) (entity.User, error) {
	if _, ok := f.byEmail[email]; ok {
		return entity.User{}, &domain.ConflictError{Message: "email already exists"}
	}

	user := entity.User{
		ID:              uuid.New(),
		Email:           email,
		PasswordHash:    hash,
		Role:            role,
		EmailVerifiedAt: verifiedAt,
	}
	f.byEmail[email] = user
	f.byID[user.ID] = user

	return user, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (entity.User, error) {
	user, ok := f.byID[id]
	if !ok {
		return entity.User{}, domain.ErrNotFound
	}

	return user, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (entity.User, error) {
	user, ok := f.byEmail[email]
	if !ok {
		return entity.User{}, domain.ErrNotFound
	}

	return user, nil
}

func (f *fakeUsers) List(context.Context, repository.Page) ([]entity.User, int64, error) {
	return nil, 0, nil
}

func (f *fakeUsers) Update(_ context.Context, user entity.User) (entity.User, error) {
	f.byEmail[user.Email] = user
	f.byID[user.ID] = user

	return user, nil
}

func (f *fakeUsers) Delete(context.Context, uuid.UUID) error { return nil }

type fakeTokens struct {
	items      []repository.RefreshToken
	hashes     map[string]int
	allRevoked bool
}

func (f *fakeTokens) Insert(_ context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (repository.RefreshToken, error) {
	if f.hashes == nil {
		f.hashes = map[string]int{}
	}

	item := repository.RefreshToken{ID: uuid.New(), UserID: userID, ExpiresAt: expiresAt}
	f.items = append(f.items, item)
	f.hashes[string(tokenHash)] = len(f.items) - 1

	return item, nil
}

func (f *fakeTokens) FindByHash(_ context.Context, tokenHash []byte) (repository.RefreshToken, error) {
	idx, ok := f.hashes[string(tokenHash)]
	if !ok {
		return repository.RefreshToken{}, domain.ErrNotFound
	}

	return f.items[idx], nil
}

func (f *fakeTokens) RevokeIfActive(_ context.Context, id uuid.UUID, at time.Time) (bool, error) {
	for i := range f.items {
		if f.items[i].ID == id && f.items[i].RevokedAt == nil {
			f.items[i].RevokedAt = &at
			return true, nil
		}
	}

	return false, nil
}

func (f *fakeTokens) RevokeAllForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	f.allRevoked = true

	for i := range f.items {
		if f.items[i].UserID == userID {
			f.items[i].RevokedAt = &at
		}
	}

	return nil
}

type fakeVerifications struct {
	users   *fakeUsers
	userID  uuid.UUID
	hash    []byte
	expires time.Time
	used    bool
}

func (f *fakeVerifications) ReplacePending(_ context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	f.userID = userID
	f.hash = append([]byte(nil), tokenHash...)
	f.expires = expiresAt
	f.used = false

	return nil
}

func (f *fakeVerifications) Consume(_ context.Context, tokenHash []byte, at time.Time) (uuid.UUID, error) {
	if f.used || string(f.hash) != string(tokenHash) || !f.expires.After(at) {
		return uuid.Nil, domain.ErrNotFound
	}

	f.used = true
	user := f.users.byID[f.userID]
	user.EmailVerifiedAt = &at
	f.users.byID[user.ID] = user
	f.users.byEmail[user.Email] = user

	return f.userID, nil
}

type fakeMailer struct {
	text string
}

func (f *fakeMailer) Send(_ context.Context, message auth.Message) error {
	f.text = message.Text

	return nil
}
