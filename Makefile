

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## build: build go aplication
.PHONY: build
build: 
	go build -o bin/api cmd/main.go

## migrate-create: Creates a new migration (e.g., make migrate-create NAME=add_users)
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Erro: use 'make migrate-create NAME=nome_da_migracao'"; exit 1; fi
	goose -s create $(NAME) sql
## sqlc-diff: Compare the schema with the queries (SQLC)
.PHONY: sqlc-diff
sqlc-diff:
	sqlc diff -f db/sqlc.yaml

## sqlc-gen: generete sqlc code
sqlc-gen:
	sqlc generate -f db/sqlc.yaml

# run: run the aplication
.PHONY: build
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

## audit: run quality control checks
.PHONY: audit
audit: test sqlc-diff
	sqlc vet -f db/sqlc.yaml
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)" 
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out

.PHONY: swag/init
swag/init:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g ./cmd/main.go -o ./cmd/docs --pd --st -q

.PHONY: swag/fmt
swag/fmt: 
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 fmt
