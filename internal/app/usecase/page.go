package usecase

import (
	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

func normalizePage(limit, offset int) (repository.Page, error) {
	if limit == 0 {
		limit = 20
	}

	if limit < 1 || limit > 100 {
		return repository.Page{}, domain.FieldErrorf("limit", "invalid", "limit must be 1 to 100")
	}

	if offset < 0 {
		return repository.Page{}, domain.FieldErrorf("offset", "invalid", "offset must be zero or positive")
	}

	return repository.Page{Limit: int32(limit), Offset: int32(offset)}, nil
}
