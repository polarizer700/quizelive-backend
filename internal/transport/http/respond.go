package adminhttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

type errorBody struct {
	Error struct {
		Slug    string `json:"slug"`
		Message string `json:"message"`
	} `json:"error"`
	Errors map[string][]fieldError `json:"errors,omitempty"`
}

type fieldError struct {
	Slug    string `json:"slug"`
	Message string `json:"message"`
}

func writeUseCaseError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	status, body := mapError(err)
	if status >= 500 {
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func mapError(err error) (int, errorBody) {
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		body := errorBody{}
		body.Error.Slug = "INVALID"
		body.Error.Message = "validation failed"
		body.Errors = map[string][]fieldError{}

		for _, field := range validation.Fields {
			body.Errors[field.Field] = append(body.Errors[field.Field], fieldError{
				Slug:    field.Slug,
				Message: field.Message,
			})
		}

		return http.StatusBadRequest, body
	}

	var conflict *domain.ConflictError
	if errors.As(err, &conflict) {
		return http.StatusConflict, simpleError("CONFLICT", conflict.Error())
	}

	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, simpleError("UNAUTHORIZED", "unauthorized")
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, simpleError("FORBIDDEN", "forbidden")
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, simpleError("NOT_FOUND", "not found")
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, simpleError("CONFLICT", "conflict")
	default:
		return http.StatusInternalServerError, simpleError("INTERNAL", "internal error")
	}
}

func simpleError(slug, message string) errorBody {
	body := errorBody{}
	body.Error.Slug = slug
	body.Error.Message = message

	return body
}
