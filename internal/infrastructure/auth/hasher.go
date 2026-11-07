package auth

import (
	"github.com/alexedwards/argon2id"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

type Hasher struct {
	dummy string
}

func NewHasher() (*Hasher, error) {
	dummy, err := argon2id.CreateHash("quizelive-timing-pad", argon2id.DefaultParams)
	if err != nil {
		return nil, err
	}

	return &Hasher{dummy: dummy}, nil
}

func (h *Hasher) Hash(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func (h *Hasher) Compare(hash, password string) error {
	if hash == "" {
		hash = h.dummy
	}

	ok, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil || !ok {
		return domain.ErrUnauthorized
	}

	return nil
}
