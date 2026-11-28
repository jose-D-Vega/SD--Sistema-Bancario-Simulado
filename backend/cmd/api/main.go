package main

import (
	"crypto/ed25519"
	"log"

	"github.com/gin-contrib/cors"
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

	// Tiene que ir antes de registrar cualquier ruta: si no, las respuestas
	// que salen antes de este middleware (incluido el preflight OPTIONS)
	// no llevarían los headers de CORS. Hace falta porque el frontend se
	// sirve en un puerto distinto al del backend (p. ej. :5500 contra
	// :8080): sin esto el navegador bloquea las respuestas y, sobre todo,
	// nunca llega a mandar los POST/PATCH reales (Content-Type:
	// application/json dispara un preflight OPTIONS que Gin sin este
	// middleware no responde con los headers que el navegador exige).
	//
	// AllowCredentials queda en false porque RequireSesion no usa cookies:
	// el token de sesión va en el header Authorization (Bearer <token>),
	// que el frontend agrega a mano en cada request. Por eso alcanza con
	// validar el origen contra una lista explícita, sin el modo
	// "credentials" de CORS (más estricto y pensado para cuando el
	// navegador manda cookies solo).
	if len(cfg.CORSAllowedOrigins) == 0 {
		log.Printf("aviso: CORS_ALLOWED_ORIGINS no está configurada; ningún navegador podrá llamar a esta API")
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Content-Type", "Authorization",
			"X-Bank-Id", "X-Timestamp", "X-Idempotency-Key", "X-Signature",
		},
		AllowCredentials: false,
	}))

	router.GET("/health", handlers.Health(dbConn, cfg.BancoID))

	// Back office del propio banco: ni forman parte del contrato entre
	// bancos (sin firma) ni de banca personal (sin sesión de usuario), así
	// que van protegidas con un secreto de admin en vez de quedar abiertas
	// en un backend que de por sí tiene que estar expuesto en red para el
	// contrato interbancario. Ver handlers.RequireAdminKey.
	router.POST("/cuentas", handlers.RequireAdminKey(cfg.AdminAPIKey), handlers.CrearCuenta(dbConn, cfg.BancoID))
	router.PATCH("/cuentas/:numero_cuenta/estado", handlers.RequireAdminKey(cfg.AdminAPIKey), handlers.CambiarEstadoCuenta(dbConn, cfg.BancoID))

	// Banca personal (usuarios/login/cuentas propias): extensión propia de
	// este banco para el frontend, fuera del contrato de interoperabilidad.
	router.POST("/usuarios/registro", handlers.RegistrarUsuario(dbConn, cfg.BancoID, cfg.SesionDuracion))
	router.POST("/usuarios/login", handlers.LoginUsuario(dbConn, cfg.SesionDuracion))
	router.POST("/usuarios/logout", handlers.RequireSesion(dbConn), handlers.LogoutUsuario(dbConn))
	router.GET("/usuarios/me", handlers.RequireSesion(dbConn), handlers.ObtenerMiPerfil(dbConn))
	router.PATCH("/usuarios/me", handlers.RequireSesion(dbConn), handlers.ActualizarMiPerfil(dbConn))
	router.PATCH("/usuarios/me/password", handlers.RequireSesion(dbConn), handlers.CambiarMiPassword(dbConn))
	router.GET("/usuarios/me/cuentas", handlers.RequireSesion(dbConn), handlers.ListarMisCuentas(dbConn, cfg.BancoID))
	router.PATCH("/cuentas/:numero_cuenta/alias", handlers.RequireSesion(dbConn), handlers.CambiarAliasCuenta(dbConn, cfg.BancoID))
	router.GET("/cuentas/:numero_cuenta/movimientos", handlers.RequireSesion(dbConn), handlers.ListarMovimientos(dbConn, cfg.BancoID))

	// GET /bancos y /transferencias/resolver-destino: extensión de banca
	// personal para el wizard de envío del frontend. Van sin middleware de
	// firma (no son parte del contrato entre bancos) y se registran
	// siempre, incluso sin BANCO_PRIVATE_KEY configurada: resolver un
	// destino DENTRO de este banco no necesita hablar con nadie más. El
	// caso "otro banco sin cliente configurado" lo maneja el propio
	// handler (ver ResolverDestino).
	router.GET("/bancos", handlers.ListarBancos(cfg.BancoID, registro))
	router.GET("/transferencias/resolver-destino", handlers.ResolverDestino(dbConn, cfg.BancoID, cliente, registro))

	router.POST("/transferencias/internas", handlers.RequireSesion(dbConn), handlers.TransferenciaInterna(dbConn, cfg.BancoID))
	if cliente != nil {
		router.POST("/transferencias/interbancarias", handlers.RequireSesion(dbConn), handlers.TransferenciaInterbancaria(dbConn, cfg.BancoID, cliente, registro))
		router.POST("/transferencias/interbancarias/:idempotency_key/reconciliar",
			handlers.RequireSesion(dbConn), handlers.ForzarReconciliacion(dbConn, cfg.BancoID, cliente, registro))
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
