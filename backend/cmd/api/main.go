package main

import (
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

	publicKeys, err := config.CargarRegistroBancos(cfg.RegistroBancosPath)
	if err != nil {
		log.Fatalf("no se pudo cargar el registro de bancos: %v", err)
	}

	router := gin.Default()

	router.GET("/health", handlers.Health(dbConn))

	router.POST("/cuentas", handlers.CrearCuenta(dbConn, cfg.BancoID))
	router.GET("/cuentas/:numero_cuenta", handlers.ObtenerCuenta(dbConn, cfg.BancoID))
	router.POST("/transferencias/internas", handlers.TransferenciaInterna(dbConn, cfg.BancoID))

	interbancario := router.Group("/interbancario")
	interbancario.Use(handlers.VerifyBankSignature(publicKeys))
	{
		interbancario.POST("/creditos", handlers.RecibirCredito(dbConn, cfg.BancoID))
		interbancario.GET("/creditos/:transfer_id", handlers.ConsultarCredito(dbConn, cfg.BancoID))
		// interbancario.POST("/cajero/autorizaciones", handlers.AutorizarRetiro(dbConn))
	}

	log.Printf("Sudameris backend escuchando en :%s (banco_id=%s)", cfg.Port, cfg.BancoID)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("error arrancando el servidor: %v", err)
	}
}
