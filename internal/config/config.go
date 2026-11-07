package config

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	AppEnv         string        `env:"APP_ENV" envDefault:"local"`
	HTTPAddr       string        `env:"HTTP_ADDR" envDefault:":8080"`
	DatabaseURL    string        `env:"DATABASE_URL,required"`
	JWTSeed        string        `env:"JWT_ED25519_SEED,required"`
	AccessTTL      time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	RefreshTTL     time.Duration `env:"JWT_REFRESH_TTL" envDefault:"720h"`
	HostTTL        time.Duration `env:"JWT_HOST_TTL" envDefault:"2m"`
	CookieSecure   bool          `env:"COOKIE_SECURE" envDefault:"false"`
	CORSOriginsRaw []string      `env:"CORS_ORIGINS" envSeparator:","`
}

func (c Config) CORSOrigins() []string {
	out := make([]string, 0, len(c.CORSOriginsRaw))
	for _, origin := range c.CORSOriginsRaw {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			out = append(out, origin)
		}
	}

	return out
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	seed, err := base64.StdEncoding.DecodeString(cfg.JWTSeed)
	if err != nil || len(seed) != 32 {
		return nil, fmt.Errorf("JWT_ED25519_SEED must be 32 bytes encoded as standard base64")
	}

	if cfg.AccessTTL <= 0 || cfg.RefreshTTL <= 0 || cfg.HostTTL <= 0 {
		return nil, fmt.Errorf("token ttl must be positive")
	}

	return cfg, nil
}
