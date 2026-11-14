package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain/media"
	infraauth "github.com/polarizer700/quizelive-backend/internal/infrastructure/auth"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/mail"
	inframedia "github.com/polarizer700/quizelive-backend/internal/infrastructure/media"
	pgdb "github.com/polarizer700/quizelive-backend/internal/infrastructure/postgres"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository"
	adminhttp "github.com/polarizer700/quizelive-backend/internal/transport/http"
)

func TestPublishedQuizHostToken(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker is not available")
	}

	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("quizelive"),
		postgres.WithUsername("quizelive"),
		postgres.WithPassword("quizelive"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = pg.Terminate(context.Background())
	})

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	if err = pgdb.Up(dsn); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	server := newAPI(t, pool)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	var login struct {
		AccessToken string `json:"access_token"`
	}
	status := doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/auth/login", "", map[string]string{
		"email":    "admin@quizelive.test",
		"password": "Password123",
	}, &login)
	if status != http.StatusOK || login.AccessToken == "" {
		t.Fatalf("login status %d token %q", status, login.AccessToken)
	}

	var quiz struct {
		ID string `json:"id"`
	}
	status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes", login.AccessToken, map[string]string{
		"title": "Город",
	}, &quiz)
	if status != http.StatusCreated || quiz.ID == "" {
		t.Fatalf("create quiz status %d id %q", status, quiz.ID)
	}

	status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes/"+quiz.ID+"/questions", login.AccessToken, map[string]any{
		"type":           "single",
		"text":           "Столица",
		"time_limit_sec": 20,
		"points":         1000,
		"options": []map[string]any{
			{"text": "Берлин", "is_correct": true},
			{"text": "Рим", "is_correct": true},
		},
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid question status %d", status)
	}

	status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes/"+quiz.ID+"/questions", login.AccessToken, map[string]any{
		"type":           "single",
		"text":           "Столица",
		"time_limit_sec": 20,
		"points":         1000,
		"options": []map[string]any{
			{"text": "Берлин", "is_correct": false},
			{"text": "Париж", "is_correct": true},
		},
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("question status %d", status)
	}

	published := "published"
	status = doJSON(t, client, http.MethodPatch, httpServer.URL+"/api/v1/quizzes/"+quiz.ID, login.AccessToken, map[string]string{
		"status": published,
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("publish status %d", status)
	}

	var hostToken struct {
		Token string `json:"token"`
	}
	status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes/"+quiz.ID+"/host-token", login.AccessToken, nil, &hostToken)
	if status != http.StatusOK || hostToken.Token == "" {
		t.Fatalf("host token status %d", status)
	}

	status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/auth/refresh", "", nil, &login)
	if status != http.StatusOK || login.AccessToken == "" {
		t.Fatalf("refresh status %d", status)
	}
}

func newAPI(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()

	cfg := &config.Config{
		JWTSeed:    "Jeb0jFvxV6LN0pEN6anTCBMb1Qb9O0XwuUOgnIrDub8=",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: time.Hour,
		HostTTL:    2 * time.Minute,
	}

	hasher, err := infraauth.NewHasher()
	if err != nil {
		t.Fatal(err)
	}

	tokens, err := infraauth.NewTokenManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	users := usecase.NewUsers(repository.NewUserRepository(pool), hasher)
	if _, err = users.CreateAdmin(context.Background(), "admin@quizelive.test", "Password123"); err != nil {
		t.Fatal(err)
	}

	return adminhttp.NewServer(
		usecase.NewAuth(
			repository.NewUserRepository(pool),
			repository.NewRefreshTokenRepository(pool),
			repository.NewEmailVerificationRepository(pool),
			repository.NewPasswordResetRepository(pool),
			hasher,
			tokens,
			mail.NewLogSender(),
			cfg.RefreshTTL,
			24*time.Hour,
			"http://localhost:5173",
		),
		users,
		usecase.NewQuizzes(repository.NewQuizRepository(pool), tokens),
		usecase.NewQuestions(
			repository.NewQuizRepository(pool),
			repository.NewQuestionRepository(pool),
			mustDiskStore(t),
		),
		tokens,
		false,
		[]string{"http://localhost"},
		t.TempDir(),
		true,
	).Handler()
}

func mustDiskStore(t *testing.T) media.Store {
	t.Helper()
	store, err := inframedia.NewDiskStore(t.TempDir(), "http://127.0.0.1:8086/media")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func doJSON(t *testing.T, client *http.Client, method, url, access string, body any, dest any) int {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}

		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if dest != nil && res.StatusCode < 300 && len(payload) > 0 {
		if err = json.Unmarshal(payload, dest); err != nil {
			t.Fatalf("decode %s: %v body %s", url, err, payload)
		}
	}

	if res.StatusCode >= 400 {
		t.Logf("%s %s -> %d %s", method, url, res.StatusCode, payload)
	}

	return res.StatusCode
}
