

.PHONY: help docker-up migrate-up migrate-create generate run run-dev run-prod build fmt sqlc-diff

help: ## Exibe este help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt: ## Formata o código Go
	@echo "Formatando arquivos Go..."
	go fmt ./...

build: fmt ## Compila a aplicação Go
	@echo "Compilando binário..."
	go build -o bin/api cmd/main.go

docker-up: ## Inicia o banco de dados (Postgres)
	docker-compose up -d

migrate-up: ## Aplica as migrações no banco de dados
	goose up

migrate-create: ## Cria uma nova migração (ex: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then echo "Erro: use 'make migrate-create NAME=nome_da_migracao'"; exit 1; fi
	goose -s create $(NAME) sql

generate: ## Gera o código Go a partir do SQL (SQLC)
	sqlc generate -f db/sqlc.yaml

sqlc-diff: ## Compara o schema com as queries (SQLC)
	sqlc diff -f db/sqlc.yaml

run: build ## Compila e executa a aplicação
	./bin/api

run-dev: build ## Executa em desenvolvimento com logs em texto
	APP_ENV=development LOG_LEVEL=INFO ./bin/api

run-prod: build ## Executa em produção com logs em JSON
	APP_ENV=production LOG_LEVEL=WARN ./bin/api
