package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib" // registra el driver "pgx"
)

func Connect(databaseURL string) (*sqlx.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL no está configurada")
	}

	dbConn, err := sqlx.Connect("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error conectando a la base: %w", err)
	}

	if err := dbConn.Ping(); err != nil {
		return nil, fmt.Errorf("error haciendo ping a la base: %w", err)
	}

	return dbConn, nil
}
