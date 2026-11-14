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
	GameHTTPAddr   string        `env:"GAME_HTTP_ADDR" envDefault:":8087"`
	DatabaseURL    string        `env:"DATABASE_URL,required"`
	JWTSeed        string        `env:"JWT_ED25519_SEED,required"`
	AccessTTL      time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	RefreshTTL     time.Duration `env:"JWT_REFRESH_TTL" envDefault:"720h"`
	HostTTL        time.Duration `env:"JWT_HOST_TTL" envDefault:"2m"`
	CookieSecure   bool          `env:"COOKIE_SECURE" envDefault:"false"`
	CORSOriginsRaw []string      `env:"CORS_ORIGINS" envSeparator:","`
	PublicAppURL   string        `env:"PUBLIC_APP_URL" envDefault:"http://127.0.0.1:5173"`
	VerifyTTL      time.Duration `env:"EMAIL_VERIFY_TTL" envDefault:"24h"`
	SMTPHost       string        `env:"SMTP_HOST"`
	SMTPPort       string        `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername   string        `env:"SMTP_USERNAME"`
	SMTPPassword   string        `env:"SMTP_PASSWORD"`
	MailFrom       string        `env:"MAIL_FROM" envDefault:"QuizeLive <noreply@quizelive.local>"`
	MediaDir       string        `env:"MEDIA_DIR" envDefault:"./.media"`
	MediaPublicURL string        `env:"MEDIA_PUBLIC_URL" envDefault:"http://127.0.0.1:8086/media"`
	TelegramToken  string        `env:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID string        `env:"TELEGRAM_CHAT_ID"`
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

	cfg.PublicAppURL = strings.TrimRight(strings.TrimSpace(cfg.PublicAppURL), "/")
	if cfg.PublicAppURL == "" {
		return nil, fmt.Errorf("PUBLIC_APP_URL is required")
	}

	if cfg.AccessTTL <= 0 || cfg.RefreshTTL <= 0 || cfg.HostTTL <= 0 || cfg.VerifyTTL <= 0 {
		return nil, fmt.Errorf("token ttl must be positive")
	}

	return cfg, nil
}
