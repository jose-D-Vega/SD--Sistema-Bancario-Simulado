package config

import "os"

type Config struct {
	BancoID       string
	Port          string
	DatabaseURL   string
	PrivateKeyB64 string

	// banco_id -> URL base. Se completa cuando el BCP simulado reparta
	// el registro de los otros 3 bancos.
	BancoURLs map[string]string
}

func Load() Config {
	return Config{
		BancoID:       getEnv("BANCO_ID", "SUDAMERIS"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		PrivateKeyB64: getEnv("BANCO_PRIVATE_KEY", ""),
		BancoURLs:     map[string]string{},
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
