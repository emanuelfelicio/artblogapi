## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## up: start local dependencies, apply migrations and run the API
.PHONY: up
up: check-env
	docker compose up -d --wait db minio
	docker compose run --rm minio-init
	$(MAKE) migrate-up
	docker compose up --build api

## clean: stop local dependencies and remove their volumes
.PHONY: clean
clean:
	docker compose down -v

## logs: follow local dependency logs
.PHONY: logs
logs:
	docker compose logs -f

## check-env: verify that the local environment file exists
.PHONY: check-env
check-env:
	@test -f .env || (echo "Erro: arquivo .env não encontrado. Execute: cp .env.example .env" >&2; exit 1)

## build: build go application
.PHONY: build
build:
	go build -o bin/api cmd/main.go

## migrate-up: apply pending database migrations
.PHONY: migrate-up
migrate-up: check-env
	@set -a; . ./.env; set +a; go tool goose up "$$GOOSE_DRIVER" "$$GOOSE_DBSTRING" -dir "$$GOOSE_MIGRATION_DIR" up

## migrate-create: Creates a new migration (e.g., make migrate-create NAME=add_users)
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Erro: use 'make migrate-create NAME=nome_da_migracao'"; exit 1; fi
	go tool goose -s create $(NAME) sql

## sqlc-diff: Compare the schema with the queries (SQLC)
.PHONY: sqlc-diff
sqlc-diff:
	go tool sqlc diff -f db/sqlc.yaml

## sqlc-gen: generate sqlc code
.PHONY: sqlc-gen
sqlc-gen:
	go tool sqlc generate -f db/sqlc.yaml

## run: run the application
.PHONY: run
run: build
	./bin/api

## tidy: tidy modfiles and modernize and format .go files
.PHONY: tidy
tidy:
	go mod tidy -v
	go fix ./...
	go fmt ./...

.PHONY: test
test:
	go test -v -race -buildvcs ./...

## test/debug: run all tests with testcontainers and goose debug logs enabled
.PHONY: test/debug
test/debug:
	TEST_DEBUG=true go test -v -race -buildvcs ./...

## audit: run quality control checks
.PHONY: audit
audit: test sqlc-diff
	go tool sqlc vet -f db/sqlc.yaml
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)"
	go vet ./...
	go tool staticcheck -checks=all,-ST1000,-U1000 ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

.PHONY: swag/init
swag/init:
	go tool swag init -g ./cmd/main.go -o ./docs/swagger --pd --st -q

.PHONY: swag/fmt
swag/fmt:
	go tool swag fmt