package adminhttp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	domainauth "github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
	"github.com/polarizer700/quizelive-backend/internal/transport/http/openapi"
)

const refreshCookie = "quizelive_refresh"

type Server struct {
	auth         *usecase.Auth
	users        *usecase.Users
	quizzes      *usecase.Quizzes
	questions    *usecase.Questions
	tokens       domainauth.TokenIssuer
	cookieSecure bool
	corsOrigins  []string
}

func NewServer(
	auth *usecase.Auth,
	users *usecase.Users,
	quizzes *usecase.Quizzes,
	questions *usecase.Questions,
	tokens domainauth.TokenIssuer,
	cookieSecure bool,
	corsOrigins []string,
) *Server {
	return &Server{
		auth:         auth,
		users:        users,
		quizzes:      quizzes,
		questions:    questions,
		tokens:       tokens,
		cookieSecure: cookieSecure,
		corsOrigins:  corsOrigins,
	}
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.corsOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(limitBody)
	router.Use(rateLimitAuth)
	router.Use(s.bindHTTP)
	router.Use(s.authenticate)
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	api := openapi.NewStrictHandlerWithOptions(s, nil, openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeUseCaseError(w, r, errInvalidBody())
		},
		ResponseErrorHandlerFunc: writeUseCaseError,
	})
	openapi.HandlerFromMux(api, router)

	return router
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeUseCaseError(w, r, errUnauthorized())
			return
		}

		id, err := s.tokens.ParseAccess(token)
		if err != nil {
			writeUseCaseError(w, r, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), id)))
	})
}

func isPublic(r *http.Request) bool {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		return true
	}

	if r.Method != http.MethodPost {
		return false
	}

	switch r.URL.Path {
	case "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout", "/api/v1/auth/register", "/api/v1/auth/verify-email":
		return true
	default:
		return false
	}
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

func rateLimitAuth(next http.Handler) http.Handler {
	limited := httprate.LimitByIP(10, time.Minute)(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/refresh" || r.URL.Path == "/api/v1/auth/register" || r.URL.Path == "/api/v1/auth/verify-email" {
			limited.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type httpBag struct {
	w http.ResponseWriter
	r *http.Request
}

type httpBagKey struct{}

func (s *Server) bindHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), httpBagKey{}, &httpBag{w: w, r: r})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) setRefreshCookie(ctx context.Context, raw string, expires time.Time) {
	bag, ok := ctx.Value(httpBagKey{}).(*httpBag)
	if !ok {
		return
	}

	http.SetCookie(bag.w, &http.Cookie{
		Name:     refreshCookie,
		Value:    raw,
		Path:     "/api/v1/auth",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) clearRefreshCookie(ctx context.Context) {
	bag, ok := ctx.Value(httpBagKey{}).(*httpBag)
	if !ok {
		return
	}

	http.SetCookie(bag.w, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func refreshFrom(ctx context.Context) string {
	bag, ok := ctx.Value(httpBagKey{}).(*httpBag)
	if !ok {
		return ""
	}

	cookie, err := bag.r.Cookie(refreshCookie)
	if err != nil {
		return ""
	}

	return cookie.Value
}
