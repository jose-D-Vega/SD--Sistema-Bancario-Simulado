CREATE TABLE IF NOT EXISTS cuentas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banco_id        TEXT NOT NULL,
    numero_cuenta   TEXT NOT NULL,
    titular         TEXT NOT NULL,
    saldo           BIGINT NOT NULL DEFAULT 0 CHECK (saldo >= 0),
    estado          TEXT NOT NULL DEFAULT 'ACTIVA' CHECK (estado IN ('ACTIVA', 'BLOQUEADA', 'CERRADA')),
    creado_en       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (banco_id, numero_cuenta)
);
