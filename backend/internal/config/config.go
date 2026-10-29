package config

import "os"

type Config struct {
	BancoID       string
	Port          string
	DatabaseURL   string
	PrivateKeyB64 string

	// Ruta a un JSON {"BCO1": "clave_publica_base64", ...} que distribuye
	// el BCP simulado. Vacío hasta que ese registro exista.
	RegistroBancosPath string

	// banco_id -> URL base. Se completa cuando el BCP simulado reparta
	// el registro de los otros 3 bancos.
	BancoURLs map[string]string
}

func Load() Config {
	return Config{
		BancoID:       getEnv("BANCO_ID", "SUDAMERIS"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		PrivateKeyB64:       getEnv("BANCO_PRIVATE_KEY", ""),
		RegistroBancosPath:  getEnv("REGISTRO_BANCOS_PATH", ""),
		BancoURLs:           map[string]string{},
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
