-- Sesiones de usuario (login de la app, no confundir con la firma Ed25519
-- entre bancos). El token es un valor aleatorio de alta entropía generado
-- por el servidor (ver internal/handlers/middleware.go), no algo que haga
-- falta hashear como la contraseña o el PIN.
CREATE TABLE IF NOT EXISTS sesiones (
    token       TEXT PRIMARY KEY,
    usuario_id  UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    creado_en   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expira_en   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sesiones_usuario ON sesiones (usuario_id);
