package repository

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

func mapDB(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "users_email_key" {
				return &domain.ConflictError{Message: "email already exists"}
			}

			if pgErr.ConstraintName == "game_sessions_code_key" {
				return domain.ErrCodeTaken
			}

			return &domain.ConflictError{Message: "conflict"}
		case "23503":
			return &domain.ConflictError{Message: "record is still referenced"}
		}
	}

	return err
}

func pgTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func pgTimePtr(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}

	return pgTime(*value)
}

func timeFrom(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}

	return value.Time
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	stamp := value.Time

	return &stamp
}
