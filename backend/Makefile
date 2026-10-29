.PHONY: run build tidy db-up db-down migrate-up migrate-down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

tidy:
	go mod tidy

db-up:
	docker compose up -d

db-down:
	docker compose down

# Requiere tener instalado golang-migrate con el driver de cockroachdb:
#   go install -tags 'cockroachdb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
# Usa MIGRATE_DATABASE_URL (esquema cockroachdb://), no DATABASE_URL
# (esquema postgresql://, esa es la que usa el backend vía pgx).
migrate-up:
	migrate -path migrations -database "$$MIGRATE_DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$MIGRATE_DATABASE_URL" down 1
