package config

import (
	"os"
	"strconv"
	"strings"
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

	// Orígenes permitidos para CORS (handlers.CORS), p. ej.
	// "http://localhost:5500" o una lista separada por comas:
	// "http://localhost:5500,http://localhost:3000". Vacío por defecto:
	// sin esta variable configurada, ningún origen de navegador puede
	// llamar a la API (fail-closed, igual que AdminAPIKey).
	CORSAllowedOrigins []string
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
		CORSAllowedOrigins:      getEnvList("CORS_ALLOWED_ORIGINS", nil),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getEnvList lee una variable separada por comas (p. ej.
// "http://a,http://b") y devuelve los valores ya sin espacios. Entradas
// vacías entre comas se descartan (",," o una variable no configurada no
// generan un origen vacío "" que terminaría matcheando por accidente).
func getEnvList(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	var out []string
	for _, parte := range strings.Split(v, ",") {
		parte = strings.TrimSpace(parte)
		if parte != "" {
			out = append(out, parte)
		}
	}
	return out
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
