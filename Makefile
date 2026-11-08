GOCACHE ?= $(CURDIR)/.gocache
export GOCACHE

TOOLS := $(CURDIR)/bin
export GOBIN := $(TOOLS)
export PATH := $(TOOLS):$(PATH)

DOCKER_COMPOSE_FILE := docker-compose-local.yml

.PHONY: tools generate sqlc openapi docker-up docker-down migrate test test-short lint build env

env:
	@test -f .env || cp .env.example .env

tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0
	go install github.com/pressly/goose/v3/cmd/goose@v3.24.3

generate: sqlc openapi

sqlc:
	mkdir -p .sqlc
	awk 'BEGIN { up = 1 } /^-- \+goose Up/ { up = 1; next } /^-- \+goose Down/ { up = 0; next } up { print }' migrations/*.sql > .sqlc/schema.sql
	$(TOOLS)/sqlc generate

openapi:
	$(TOOLS)/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

docker-up:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d --wait

docker-down:
	docker compose -f $(DOCKER_COMPOSE_FILE) down

migrate: env
	set -a && . ./.env && set +a && \
	GOOSE_DRIVER=postgres GOOSE_MIGRATION_DIR=./migrations GOOSE_DBSTRING="$$DATABASE_URL" $(TOOLS)/goose up

test:
	go test ./...

test-short:
	go test -short ./...

lint:
	golangci-lint run

build:
	go build ./...
