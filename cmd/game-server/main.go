package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"github.com/polarizer700/quizelive-backend/internal/infrastructure/di"
)

func main() {
	_ = godotenv.Load()

	app := fx.New(di.GameModule())
	app.Run()

	if err := app.Err(); err != nil {
		slog.Error("game-server", "category", "lifecycle", "error", err)
		os.Exit(1)
	}
}
