package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
	"go.uber.org/fx"

	"github.com/polarizer700/quizelive-backend/internal/app/usecase"
	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/di"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/postgres"
)

func main() {
	_ = godotenv.Load()

	cmd := &cli.Command{
		Name:  "admin-api",
		Usage: "QuizeLive admin API",
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Запускает HTTP-сервер",
				Action: func(ctx context.Context, _ *cli.Command) error {
					fx.New(di.Module(), fx.Invoke(di.StartHTTP)).Run()
					return nil
				},
			},
			{
				Name:  "migrate",
				Usage: "Накатывает миграции goose",
				Action: func(ctx context.Context, _ *cli.Command) error {
					cfg, err := config.Load()
					if err != nil {
						return err
					}

					return postgres.Up(cfg.DatabaseURL)
				},
			},
			{
				Name:  "create-admin",
				Usage: "Создаёт пользователя с ролью admin",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "email", Required: true},
					&cli.StringFlag{Name: "password", Required: true},
				},
				Action: createAdmin,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("admin-api", "error", err)
		os.Exit(1)
	}
}

func createAdmin(ctx context.Context, cmd *cli.Command) error {
	var users *usecase.Users

	app := fx.New(di.Module(), fx.Populate(&users))
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		return err
	}

	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = app.Stop(stopCtx)
	}()

	user, err := users.CreateAdmin(ctx, cmd.String("email"), cmd.String("password"))
	if err != nil {
		return err
	}

	fmt.Printf("admin created: %s %s\n", user.ID, user.Email)

	return nil
}
