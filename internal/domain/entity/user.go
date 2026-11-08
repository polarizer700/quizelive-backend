package entity

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleHost   Role = "host"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleEditor, RoleHost:
		return true
	default:
		return false
	}
}

func (r Role) CanManageUsers() bool {
	return r == RoleAdmin
}

func (r Role) CanEditQuizzes() bool {
	return r == RoleAdmin || r == RoleEditor
}

type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	Role            Role
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
}

func NormalizeEmail(raw string) (string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", domain.FieldErrorf("email", "invalid", "email is invalid")
	}

	email := strings.ToLower(parsed.Address)
	if utf8.RuneCountInString(email) < 3 || utf8.RuneCountInString(email) > 320 {
		return "", domain.FieldErrorf("email", "invalid", "email is invalid")
	}

	return email, nil
}

func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < 8 || n > 200 {
		return domain.FieldErrorf("password", "invalid", "password must be 8 to 200 characters")
	}

	return nil
}
