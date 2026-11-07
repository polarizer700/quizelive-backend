package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager(&config.Config{
		JWTSeed:   "Jeb0jFvxV6LN0pEN6anTCBMb1Qb9O0XwuUOgnIrDub8=",
		AccessTTL: time.Minute,
		HostTTL:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	user := entity.User{ID: uuid.New(), Role: entity.RoleEditor}
	token, expiresIn, err := manager.IssueAccess(user)
	if err != nil {
		t.Fatal(err)
	}

	if expiresIn != time.Minute {
		t.Fatalf("ttl: %s", expiresIn)
	}

	id, err := manager.ParseAccess(token)
	if err != nil {
		t.Fatal(err)
	}

	if id.UserID != user.ID || id.Role != user.Role {
		t.Fatalf("identity: %+v", id)
	}

	host, _, err := manager.IssueHost(user, uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	if _, err = manager.ParseAccess(host); err != domain.ErrUnauthorized {
		t.Fatalf("host token must not pass as access: %v", err)
	}
}
