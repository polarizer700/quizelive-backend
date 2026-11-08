package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid")
	ErrCodeTaken    = errors.New("room code taken")
)

// ConflictError сохраняет errors.Is(..., ErrConflict) и несёт текст для API.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	if e.Message == "" {
		return ErrConflict.Error()
	}

	return e.Message
}

func (e *ConflictError) Unwrap() error {
	return ErrConflict
}

// EmailUnverifiedError означает, что пароль верный, но почта ещё не подтверждена.
type EmailUnverifiedError struct{}

func (e *EmailUnverifiedError) Error() string {
	return "email is not verified"
}

func (e *EmailUnverifiedError) Unwrap() error {
	return ErrForbidden
}

// FieldError — ошибка одного поля.
type FieldError struct {
	Field   string
	Slug    string
	Message string
}

// ValidationError — набор полевых ошибок. errors.Is(..., ErrInvalid) истинно.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return ErrInvalid.Error()
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalid
}

func FieldErrorf(field, slug, message string) *ValidationError {
	return &ValidationError{Fields: []FieldError{{
		Field:   field,
		Slug:    slug,
		Message: message,
	}}}
}
