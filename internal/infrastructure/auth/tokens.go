package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
)

const (
	issuerName  = "quizelive"
	audienceAPI = "quizelive-admin"
	audienceWS  = "quizelive-game"
	useAccess   = "access"
	useHost     = "host"
)

type claims struct {
	jwt.RegisteredClaims
	Role   string `json:"role"`
	Use    string `json:"use"`
	QuizID string `json:"quiz_id,omitempty"`
}

type TokenManager struct {
	private   ed25519.PrivateKey
	public    ed25519.PublicKey
	accessTTL time.Duration
	hostTTL   time.Duration
	now       func() time.Time
}

func NewTokenManager(cfg *config.Config) (*TokenManager, error) {
	seed, err := base64.StdEncoding.DecodeString(cfg.JWTSeed)
	if err != nil || len(seed) != 32 {
		return nil, fmt.Errorf("invalid jwt seed")
	}

	private := ed25519.NewKeyFromSeed(seed)

	return &TokenManager{
		private:   private,
		public:    private.Public().(ed25519.PublicKey),
		accessTTL: cfg.AccessTTL,
		hostTTL:   cfg.HostTTL,
		now:       time.Now,
	}, nil
}

func (m *TokenManager) IssueAccess(user entity.User) (string, time.Duration, error) {
	token, err := m.sign(user, useAccess, audienceAPI, "", m.accessTTL)
	if err != nil {
		return "", 0, err
	}

	return token, m.accessTTL, nil
}

func (m *TokenManager) IssueHost(user entity.User, quizID uuid.UUID) (string, time.Duration, error) {
	token, err := m.sign(user, useHost, audienceWS, quizID.String(), m.hostTTL)
	if err != nil {
		return "", 0, err
	}

	return token, m.hostTTL, nil
}

func (m *TokenManager) ParseAccess(token string) (identity.Identity, error) {
	parsed, err := m.parse(token, audienceAPI, useAccess)
	if err != nil {
		return identity.Identity{}, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(parsed.Subject)
	if err != nil {
		return identity.Identity{}, domain.ErrUnauthorized
	}

	role := entity.Role(parsed.Role)
	if !role.Valid() {
		return identity.Identity{}, domain.ErrUnauthorized
	}

	return identity.Identity{UserID: userID, Role: role}, nil
}

func (m *TokenManager) sign(user entity.User, use, audience, quizID string, ttl time.Duration) (string, error) {
	now := m.now()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuerName,
			Subject:   user.ID.String(),
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		Role:   string(user.Role),
		Use:    use,
		QuizID: quizID,
	})

	return token.SignedString(m.private)
}

func (m *TokenManager) parse(token, audience, use string) (claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected signing method")
		}

		return m.public, nil
	}, jwt.WithIssuer(issuerName), jwt.WithAudience(audience), jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	if err != nil {
		return claims{}, err
	}

	out, ok := parsed.Claims.(*claims)
	if !ok || out.Use != use {
		return claims{}, fmt.Errorf("unexpected token use")
	}

	return *out, nil
}
