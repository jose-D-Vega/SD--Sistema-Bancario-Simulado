package main

import (
	"crypto/ed25519"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/sudameris/backend/internal/config"
	"github.com/sudameris/backend/internal/db"
	"github.com/sudameris/backend/internal/handlers"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	dbConn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("no se pudo conectar a la base: %v", err)
	}
	defer dbConn.Close()

	// TODO: cargar claves públicas de los otros bancos cuando el BCP
	// simulado las distribuya.
	publicKeys := map[string]ed25519.PublicKey{}

	router := gin.Default()

	router.GET("/health", handlers.Health(dbConn))

	interbancario := router.Group("/")
	interbancario.Use(handlers.VerifyBankSignature(publicKeys))
	{
		// interbancario.POST("/transferencias/interbancarias", handlers.RecibirTransferencia(dbConn))
		// interbancario.POST("/cajero/autorizaciones", handlers.AutorizarRetiro(dbConn))
	}

	log.Printf("Sudameris backend escuchando en :%s (banco_id=%s)", cfg.Port, cfg.BancoID)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("error arrancando el servidor: %v", err)
	}
}
