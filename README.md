# quizelive-backend

Go-модуль QuizeLive. Сейчас в нём фундамент и `admin-api`. `game-server` пока только точка входа: игровой хаб будет на следующем этапе, контракт кадров лежит в `proto/game/v1/game.proto`.

Слои, как в promosite-service: `cmd` → `transport` → `app` → `domain`, реализации в `infrastructure`. Зависимости направлены внутрь, границы проверяет `depguard`.

## Локальный запуск

```bash
cp .env.example .env
make docker-up
make tools
go run ./cmd/admin-api migrate
go run ./cmd/admin-api create-admin --email admin@quizelive.local --password 'change-me-now'
go run ./cmd/admin-api start
```

Postgres поднимается из `docker-compose-local.yml` на порту 5432. HTTP слушает `:8080`. Проверка: `GET /healthz`.

Access JWT живёт 15 минут и возвращается в теле. Refresh кладётся в cookie `quizelive_refresh` (`HttpOnly`, `SameSite=Strict`) и ротируется. Повтор уже отозванного refresh отзывает всю цепочку токенов пользователя.

Роли:

- `admin` управляет пользователями и всеми квизами
- `editor` управляет своими квизами
- `host` видит опубликованные квизы и получает host-токен

Вопрос — агрегат: варианты и допустимые текстовые ответы сохраняются вместе с ним. Host-токен выдаётся только для `published` квиза.

`JWT_ED25519_SEED` в `.env.example` годится только для локальной разработки.
