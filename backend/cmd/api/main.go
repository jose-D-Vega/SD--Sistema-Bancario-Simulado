package main

import (
	"crypto/ed25519"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/sudameris/backend/internal/config"
	"github.com/sudameris/backend/internal/crypto"
	"github.com/sudameris/backend/internal/db"
	"github.com/sudameris/backend/internal/handlers"
	"github.com/sudameris/backend/internal/interbanco"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	dbConn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("no se pudo conectar a la base: %v", err)
	}
	defer dbConn.Close()

	registro, err := config.CargarRegistroBancos(cfg.RegistroBancosPath)
	if err != nil {
		log.Fatalf("no se pudo cargar el registro de bancos: %v", err)
	}
	publicKeys := map[string]ed25519.PublicKey{}
	for id, b := range registro {
		publicKeys[id] = b.PublicKey
	}

	var cliente *interbanco.Cliente
	if cfg.PrivateKeyB64 != "" {
		priv, err := crypto.LoadPrivateKey(cfg.PrivateKeyB64)
		if err != nil {
			log.Fatalf("clave privada inválida: %v", err)
		}
		cliente = interbanco.NuevoCliente(cfg.BancoID, priv)
	}

	router := gin.Default()

	router.GET("/health", handlers.Health(dbConn))

	router.POST("/cuentas", handlers.CrearCuenta(dbConn, cfg.BancoID))
	router.PATCH("/cuentas/:numero_cuenta/estado", handlers.CambiarEstadoCuenta(dbConn, cfg.BancoID))
	router.POST("/transferencias/internas", handlers.TransferenciaInterna(dbConn, cfg.BancoID))
	if cliente != nil {
		router.POST("/transferencias/interbancarias", handlers.TransferenciaInterbancaria(dbConn, cfg.BancoID, cliente, registro))
		router.POST("/transferencias/interbancarias/:idempotency_key/reconciliar",
			handlers.ForzarReconciliacion(dbConn, cfg.BancoID, cliente, registro))
		handlers.IniciarJobReconciliacion(dbConn, cliente, registro, cfg.BancoID, cfg.ReconciliacionIntervalo)
		log.Printf("job de reconciliación corriendo cada %s", cfg.ReconciliacionIntervalo)
	}

	interbancario := router.Group("/interbancario")
	interbancario.Use(handlers.VerifyBankSignature(publicKeys))
	{
		interbancario.POST("/creditos", handlers.RecibirCredito(dbConn, cfg.BancoID))
		interbancario.GET("/creditos/:transfer_id", handlers.ConsultarCredito(dbConn, cfg.BancoID))
	}

	router.GET("/cuentas/:numero_cuenta", handlers.VerifyBankSignature(publicKeys), handlers.ObtenerCuenta(dbConn, cfg.BancoID))

	cajero := router.Group("/cajero")
	cajero.Use(handlers.VerifyBankSignature(publicKeys))
	{
		cajero.POST("/autorizaciones", handlers.AutorizarRetiroCajero(dbConn, cfg.BancoID))
		cajero.GET("/autorizaciones/:operacion_id", handlers.ConsultarAutorizacionCajero(dbConn, cfg.BancoID))
	}

	log.Printf("Sudameris backend escuchando en :%s (banco_id=%s)", cfg.Port, cfg.BancoID)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("error arrancando el servidor: %v", err)
	}
}
