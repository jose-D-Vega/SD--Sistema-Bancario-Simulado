package config

import (
	"os"
	"strconv"
	"time"
)

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

	// Cada cuánto el job de reconciliación revisa las operaciones
	// PENDIENTE. 30s por defecto para que sea fácil de probar; en un
	// entorno real conviene algo más espaciado (1-5 min).
	ReconciliacionIntervalo time.Duration

	// Cuánto dura una sesión de usuario (login del frontend) antes de
	// expirar. Esto es parte de la capa de banca personal, separada del
	// contrato de interoperabilidad entre bancos.
	SesionDuracion time.Duration

	// Secreto compartido que protege las operaciones de back office del
	// propio banco (alta y bloqueo/cierre de cuentas, ver
	// handlers.RequireAdminKey): no son parte del contrato entre bancos
	// (no llevan firma Ed25519) ni de banca personal (no hay un usuario
	// dueño todavía), así que necesitan su propio mecanismo. Vacío por
	// defecto a propósito: RequireAdminKey bloquea todo si esto no está
	// configurado, en vez de dejar pasar.
	AdminAPIKey string
}

func Load() Config {
	return Config{
		BancoID:       getEnv("BANCO_ID", "SUDAMERIS"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		PrivateKeyB64:       getEnv("BANCO_PRIVATE_KEY", ""),
		RegistroBancosPath:      getEnv("REGISTRO_BANCOS_PATH", ""),
		BancoURLs:               map[string]string{},
		ReconciliacionIntervalo: time.Duration(getEnvInt("RECONCILIACION_INTERVALO_SEGUNDOS", 30)) * time.Second,
		SesionDuracion:          time.Duration(getEnvInt("SESION_DURACION_HORAS", 24)) * time.Hour,
		AdminAPIKey:             getEnv("ADMIN_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
