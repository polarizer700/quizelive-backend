package di

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	gameapp "github.com/polarizer700/quizelive-backend/internal/app/game"
	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	domainrepo "github.com/polarizer700/quizelive-backend/internal/domain/repository"
	infraauth "github.com/polarizer700/quizelive-backend/internal/infrastructure/auth"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/mail"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository"
	adminhttp "github.com/polarizer700/quizelive-backend/internal/transport/http"
	wsgame "github.com/polarizer700/quizelive-backend/internal/transport/ws"
)

func Module() fx.Option {
	return fx.Options(
		fx.Provide(config.Load),
		fx.Provide(newPool),
		fx.Provide(infraauth.NewHasher),
		fx.Provide(infraauth.NewTokenManager),
		fx.Provide(repository.NewUserRepository),
		fx.Provide(repository.NewRefreshTokenRepository),
		fx.Provide(repository.NewQuizRepository),
		fx.Provide(repository.NewQuestionRepository),
		fx.Provide(repository.NewEmailVerificationRepository),
		fx.Provide(asUserRepository),
		fx.Provide(asRefreshTokenRepository),
		fx.Provide(asEmailVerificationRepository),
		fx.Provide(asQuizRepository),
		fx.Provide(asQuestionRepository),
		fx.Provide(asPasswordHasher),
		fx.Provide(asTokenIssuer),
		fx.Provide(newMailer),
		fx.Provide(newAuth),
		fx.Provide(usecase.NewUsers),
		fx.Provide(usecase.NewQuizzes),
		fx.Provide(usecase.NewQuestions),
		fx.Provide(newHTTPServer),
	)
}

func GameModule() fx.Option {
	return fx.Options(
		fx.Provide(config.Load),
		fx.Provide(newPool),
		fx.Provide(infraauth.NewTokenManager),
		fx.Provide(repository.NewQuizRepository),
		fx.Provide(repository.NewQuestionRepository),
		fx.Provide(repository.NewSessionRepository),
		fx.Provide(asQuizRepository),
		fx.Provide(asQuestionRepository),
		fx.Provide(asSessionRepository),
		fx.Provide(asHostParser),
		fx.Provide(gameapp.NewLoader),
		fx.Provide(gameapp.NewHub),
		fx.Provide(wsgame.NewServer),
		fx.Invoke(StartGame),
	)
}

func newAuth(
	users domainrepo.UserRepository,
	tokens domainrepo.RefreshTokenRepository,
	verifications domainrepo.EmailVerificationRepository,
	hasher auth.PasswordHasher,
	issuer auth.TokenIssuer,
	mailer auth.Mailer,
	cfg *config.Config,
) *usecase.Auth {
	return usecase.NewAuth(users, tokens, verifications, hasher, issuer, mailer, cfg.RefreshTTL, cfg.VerifyTTL, cfg.PublicAppURL)
}

func newMailer(cfg *config.Config) auth.Mailer {
	if cfg.SMTPHost == "" {
		return mail.NewLogSender()
	}

	return mail.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.MailFrom)
}

func asUserRepository(repo *repository.UserRepository) domainrepo.UserRepository { return repo }

func asRefreshTokenRepository(repo *repository.RefreshTokenRepository) domainrepo.RefreshTokenRepository {
	return repo
}

func asEmailVerificationRepository(repo *repository.EmailVerificationRepository) domainrepo.EmailVerificationRepository {
	return repo
}

func asQuizRepository(repo *repository.QuizRepository) domainrepo.QuizRepository { return repo }

func asQuestionRepository(repo *repository.QuestionRepository) domainrepo.QuestionRepository {
	return repo
}

func asSessionRepository(repo *repository.SessionRepository) domainrepo.GameSessionRepository {
	return repo
}

func asHostParser(manager *infraauth.TokenManager) auth.HostTokenParser { return manager }

func asPasswordHasher(hasher *infraauth.Hasher) auth.PasswordHasher { return hasher }

func asTokenIssuer(issuer *infraauth.TokenManager) auth.TokenIssuer { return issuer }

func newHTTPServer(
	cfg *config.Config,
	authn *usecase.Auth,
	users *usecase.Users,
	quizzes *usecase.Quizzes,
	questions *usecase.Questions,
	tokens auth.TokenIssuer,
) *http.Server {
	handler := adminhttp.NewServer(authn, users, quizzes, questions, tokens, cfg.CookieSecure, cfg.CORSOrigins()).Handler()

	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func newPool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return pool.Ping(ctx)
		},
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

func StartGame(lc fx.Lifecycle, server *wsgame.Server, cfg *config.Config) {
	httpServer := &http.Server{
		Addr:              cfg.GameHTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				slog.Info("game-server listening", "addr", httpServer.Addr)
				if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("game-server stopped", "error", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return httpServer.Shutdown(ctx)
		},
	})
}

func StartHTTP(lc fx.Lifecycle, server *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				slog.Info("admin-api listening", "addr", server.Addr)
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("admin-api stopped", "error", err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}

var _ auth.TokenIssuer = (*infraauth.TokenManager)(nil)
var _ auth.HostTokenParser = (*infraauth.TokenManager)(nil)
var _ auth.PasswordHasher = (*infraauth.Hasher)(nil)
var _ auth.Mailer = (*mail.LogSender)(nil)
var _ auth.Mailer = (*mail.SMTPSender)(nil)
