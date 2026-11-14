package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Up(databaseURL string) error {
	dir, err := MigrationsDir()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	defer func() { _ = db.Close() }()

	if err = db.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err = goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

func MigrationsDir() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "migrations")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("migrations directory not found")
	}

	candidate := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", errors.New("migrations directory not found")
	}

	return candidate, nil
}
