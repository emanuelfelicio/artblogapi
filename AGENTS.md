# Project instructions

## Scope and documentation

- The implemented architecture is documented in
  [`docs/architecture.md`](docs/architecture.md).
- The code, tests, migrations, queries, configuration, and Compose setup are
  the source of truth for current behavior.
- Keep documentation aligned with the implementation. Do not describe plans or
  future features as existing components.
- Prefer small, focused changes that follow the existing code patterns.
- Do not add routes, queries, services, domain states, or infrastructure
  without a real consumer or an explicit requirement.
- Update directly related documentation when behavior or developer workflows
  change.

## Architecture and dependencies

- Preserve the `Handler -> Service -> Repository` direction.
- Handlers own HTTP concerns, repositories own persistence, and services own
  use cases and authorization rules.
- Define interfaces at the consumer side.
- Keep limits and business rules in the domain/service layer.
- Preserve transaction propagation through `context.Context`; do not start
  independent transactions inside an already transactional use case.
- Keep authentication and ownership checks in services, not only in handlers.

## Database and generated code

- Edit migrations under `db/migrations/` and queries under `db/queries/`; do
  not edit `db/dbgen/` manually.
- After changing migrations or queries, run `make sqlc-gen` and include the
  generated changes when they are part of the requested change.
- Validate SQL, constraints, locks, and transaction changes with integration
  tests against real PostgreSQL.
- Treat PostgreSQL as the durable source of truth for asynchronous worker jobs;
  in-memory notifications must not be required for correctness.
- Migrations are not applied by the application at startup. Keep local or
  deployment migration steps explicit.

## Tests and validation

- Use the commands below as the canonical local validation flow. The
  corresponding `Makefile` targets are only shortcuts for these commands.
- Go 1.26 is required by `go.mod`.
- Docker must be available for integration tests because they use
  Testcontainers PostgreSQL.
- Start local dependencies with `docker compose up -d` before running the API
  or integration tests that need PostgreSQL/MinIO.
- For local verification, run:
  `go fmt ./...`, `go vet ./...`, `go mod tidy`, `go test -race ./...`, and
  `go tool staticcheck -checks=all,-ST1000,-U1000 ./...`.
- For a focused code change, run the smallest relevant test or package first,
  then run the complete local verification flow when practical.
- After schema or query changes, run:
  `go tool sqlc generate -f db/sqlc.yaml` and
  `go tool sqlc vet -f db/sqlc.yaml`.
- After changing Swagger annotations in Go code, run:
  `go tool swag init -g ./cmd/main.go -o ./docs/swagger --pd --st -q`. Generated
  files under `docs/swagger/` are derived artifacts and should not be edited
  manually.
- If Docker or another required dependency is unavailable, report that
  explicitly instead of silently skipping the affected validation.

Do not hide errors or introduce success-shaped fallbacks. For handler tests,
use typed structs or maps with `testhttp.DoRequest`; do not pass an
`io.Reader` or pre-serialized JSON when the helper can encode the request.

## Git

- Use Conventional Commits.
- Keep unrelated changes separate.
- Do not revert or overwrite other contributors' changes.
- Do not commit secrets, local `.env` files, generated temporary artifacts, or
  credentials.
