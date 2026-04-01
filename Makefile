

.PHONY: help docker-up migrate-up migrate-create sqlc-gen run fmt

help: ## Exibe este help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt:
	@echo "Formantando arquivos Go..."
	go fmt ./...

docker-up: ## Inicia o banco de dados (Postgres)
	docker-compose up -d

migrate-up: ## Aplica as migrações no banco de dados
	goose up

migrate-create: ## Cria uma nova migração (ex: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then echo "Erro: use 'make migrate-create NAME=nome_da_migracao'"; exit 1; fi
	goose -s create $(NAME) sql

gen: ## Gera o código Go a partir do SQL (SQLC)
	cd db && sqlc generate

run: ## Executa a aplicação Go
	go run main.go
