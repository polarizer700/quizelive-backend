package di

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	domainrepo "github.com/polarizer700/quizelive-backend/internal/domain/repository"
	infraauth "github.com/polarizer700/quizelive-backend/internal/infrastructure/auth"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository"
	adminhttp "github.com/polarizer700/quizelive-backend/internal/transport/http"
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
		fx.Provide(asUserRepository),
		fx.Provide(asRefreshTokenRepository),
		fx.Provide(asQuizRepository),
		fx.Provide(asQuestionRepository),
		fx.Provide(asPasswordHasher),
		fx.Provide(asTokenIssuer),
		fx.Provide(newAuth),
		fx.Provide(usecase.NewUsers),
		fx.Provide(usecase.NewQuizzes),
		fx.Provide(usecase.NewQuestions),
		fx.Provide(newHTTPServer),
	)
}

func newAuth(
	users domainrepo.UserRepository,
	tokens domainrepo.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	issuer auth.TokenIssuer,
	cfg *config.Config,
) *usecase.Auth {
	return usecase.NewAuth(users, tokens, hasher, issuer, cfg.RefreshTTL)
}

func asUserRepository(repo *repository.UserRepository) domainrepo.UserRepository { return repo }

func asRefreshTokenRepository(repo *repository.RefreshTokenRepository) domainrepo.RefreshTokenRepository {
	return repo
}

func asQuizRepository(repo *repository.QuizRepository) domainrepo.QuizRepository { return repo }

func asQuestionRepository(repo *repository.QuestionRepository) domainrepo.QuestionRepository {
	return repo
}

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
var _ auth.PasswordHasher = (*infraauth.Hasher)(nil)
