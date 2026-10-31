.PHONY: run build tidy db-up db-down migrate-up migrate-down docker-up docker-down docker-logs docker-rebuild

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

# Levanta TODO (base + migraciones + backend) con un solo comando. Pensado
# para quien no quiere instalar Go: solo necesita Docker.
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f backend

docker-rebuild:
	docker compose up -d --build backend
