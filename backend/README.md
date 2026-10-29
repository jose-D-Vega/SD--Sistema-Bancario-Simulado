# Backend — Banco Sudameris

Backend del banco Sudameris para el proyecto de Sistemas Distribuidos.

## Stack

- **Go** 1.22
- **Gin** — framework HTTP
- **CockroachDB** — base de datos (protocolo compatible con Postgres)
- **sqlx / pgx** — acceso a datos
- **golang-migrate** — migraciones de esquema
- **crypto/ed25519** (stdlib) — firma y verificación entre bancos

## Requisitos

- Go 1.22+
- Docker (para levantar CockroachDB localmente)
- [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) instalado (para correr migraciones)

## Cómo levantar el entorno de desarrollo

```bash
# 1. Copiar variables de entorno
cp .env.example .env
# completar BANCO_PRIVATE_KEY con tu clave privada en base64

# 2. Instalar golang-migrate con el driver de cockroachdb (una sola vez)
go install -tags 'cockroachdb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 3. Levantar CockroachDB
make db-up

# 4. Crear la base (CockroachDB en modo --insecure no la crea sola)
docker exec -it sudameris-cockroachdb ./cockroach sql --insecure \
  -e "CREATE DATABASE IF NOT EXISTS sudameris;"

# 5. Correr las migraciones (ojo: MIGRATE_DATABASE_URL, no DATABASE_URL)
export MIGRATE_DATABASE_URL="cockroachdb://root@localhost:26257/sudameris?sslmode=disable"
make migrate-up

# 6. Instalar dependencias de Go
make tidy

# 7. Levantar el backend (lee DATABASE_URL desde .env, con esquema postgresql://)
make run
```

El backend queda escuchando en `http://localhost:8080`. Podés probar que
esté vivo con:

```bash
curl http://localhost:8080/health
```

## Estructura

```
backend/
├── cmd/api/            # entrypoint (main.go)
├── internal/
│   ├── config/          # carga de variables de entorno
│   ├── crypto/           # firma/verificación Ed25519 (doc 02)
│   ├── db/                # conexión a CockroachDB
│   ├── handlers/          # endpoints HTTP y middlewares
│   └── models/            # (pendiente) structs de dominio
├── migrations/          # esquema de base, versionado con golang-migrate
├── docker-compose.yml   # CockroachDB para desarrollo local
└── .env.example
```

## Estado actual

Esqueleto inicial: conexión a base, migraciones de `cuentas` y
`operaciones`, middleware de verificación de firma entre bancos, y
endpoint `/health`. Todavía faltan implementar los endpoints de
negocio del documento 02 (transferencias internas e interbancarias) y
03 (autorizaciones de cajero).
