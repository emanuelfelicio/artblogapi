# Artblog API

API REST em Go para um blog comunitário de artes.

## Funcionalidades

- Registro, login, renovação de token e logout
- Perfis públicos e gerenciamento do próprio perfil
- Criação, consulta, edição e remoção de posts
- Imagens ordenadas associadas aos posts
- Upload direto usando URLs pré-assinadas do S3
- Processamento assíncrono de imagens 

Ainda não estão implementados: comentários, curtidas, tags, seguidores e
limpeza de uploads órfãos.

## Stack

- Go 1.26
- Gin
- PostgreSQL
- SQLC
- Goose

## Estrutura de diretórios

```text
cmd/                 Entrada da aplicação e documentação da API
config/              Configuração, logs, validação e respostas da API
db/migrations/       Migrações do PostgreSQL
db/queries/          Consultas usadas pelo SQLC
db/dbgen/            Código gerado pelo sqlc
internal/auth/       Autenticação e sessões
internal/user/       Perfis e usuários
internal/post/       Posts e imagens associadas
internal/storage/    Uploads, armazenamento e processamento de imagens
internal/middleware/ Middlewares HTTP
internal/testutil/   Utilitários dos testes
docs/                Documentação da arquitetura
```

## Pré-requisitos

- Go 1.26 ou superior
- Docker e Docker Compose


## Execução local

Na raiz deste projeto:

```bash
cp .env.example .env
go mod download
docker compose up -d
```

Exporte as variáveis, aplique as migrações e inicie a API:

```bash
set -a
. ./.env
set +a
go tool goose -dir db/migrations postgres "$DATABASE_URL" up
go run ./cmd/main.go
```

A API escuta em `http://localhost:8080`. A interface da documentação da API
(Swagger UI) está disponível em `http://localhost:8080/swagger/index.html`.

## Validação

Verificação local:

```bash
go fmt ./...
go vet ./...
go mod tidy
go test -race ./...
go tool staticcheck -checks=all,-ST1000,-U1000 ./...
```

Os testes de integração exigem Docker.

## Documentação

- [Arquitetura](docs/architecture.md)
- [Especificação Swagger](docs/swagger/swagger.yaml)
