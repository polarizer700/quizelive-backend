package api_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	pgdb "github.com/polarizer700/quizelive-backend/internal/infrastructure/postgres"
)

// Мульти-фильтры списка игр: game_type/status/category принимают
// несколько значений через запятую и комбинируются между собой.
func TestListQuizzesMultiFilters(t *testing.T) {
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

	httpServer := httptest.NewServer(newAPI(t, pool))
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

	createQuiz := func(title, gameType, category string) string {
		t.Helper()

		var quiz struct {
			ID string `json:"id"`
		}
		status := doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes", login.AccessToken, map[string]string{
			"title":     title,
			"game_type": gameType,
		}, &quiz)
		if status != http.StatusCreated || quiz.ID == "" {
			t.Fatalf("create quiz %q status %d id %q", title, status, quiz.ID)
		}

		status = doJSON(t, client, http.MethodPost, httpServer.URL+"/api/v1/quizzes/"+quiz.ID+"/questions", login.AccessToken, map[string]any{
			"type":           "single",
			"text":           "Вопрос",
			"time_limit_sec": 20,
			"points":         1000,
			"category":       category,
			"options": []map[string]any{
				{"text": "Да", "is_correct": true},
				{"text": "Нет", "is_correct": false},
			},
		}, nil)
		if status != http.StatusCreated {
			t.Fatalf("create question status %d", status)
		}

		return quiz.ID
	}

	createQuiz("Водка квиз", "quiz", "Водка")
	jeopardyID := createQuiz("Своя игра про науку", "jeopardy", "Наука")
	createQuiz("Паб квиз", "pub_quiz", "")

	status = doJSON(t, client, http.MethodPatch, httpServer.URL+"/api/v1/quizzes/"+jeopardyID, login.AccessToken, map[string]string{
		"status": "published",
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("publish status %d", status)
	}

	type listResult struct {
		Total int64 `json:"total"`
	}

	assertTotal := func(path string, query url.Values, want int64) {
		t.Helper()

		var list listResult
		status := doJSON(t, client, http.MethodGet, httpServer.URL+path+"?"+query.Encode(), login.AccessToken, nil, &list)
		if status != http.StatusOK {
			t.Fatalf("GET %s?%s status %d", path, query.Encode(), status)
		}
		if list.Total != want {
			t.Fatalf("GET %s?%s total %d, want %d", path, query.Encode(), list.Total, want)
		}
	}

	assertTotal("/api/v1/quizzes", url.Values{}, 3)
	assertTotal("/api/v1/quizzes", url.Values{"game_type": {"quiz,jeopardy"}}, 2)
	assertTotal("/api/v1/quizzes", url.Values{"game_type": {"pub_quiz"}}, 1)
	assertTotal("/api/v1/quizzes", url.Values{"status": {"draft,published"}}, 3)
	assertTotal("/api/v1/quizzes", url.Values{"status": {"draft"}}, 2)
	assertTotal("/api/v1/quizzes", url.Values{"category": {"Водка"}}, 1)
	assertTotal("/api/v1/quizzes", url.Values{"category": {"Водка,Наука"}}, 2)
	assertTotal("/api/v1/quizzes", url.Values{"game_type": {"quiz,jeopardy"}, "category": {"Наука"}}, 1)
	assertTotal("/api/v1/games/public", url.Values{"game_type": {"quiz,jeopardy"}, "category": {"Наука"}}, 1)
	assertTotal("/api/v1/games/public", url.Values{"game_type": {"quiz"}}, 0)
}
