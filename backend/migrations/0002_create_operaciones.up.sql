CREATE TABLE IF NOT EXISTS operaciones (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo                TEXT NOT NULL CHECK (tipo IN ('TRANSFERENCIA_INTERNA', 'TRANSFERENCIA_INTERBANCARIA', 'RETIRO_CAJERO')),
    idempotency_key     UUID NOT NULL,

    banco_origen        TEXT NOT NULL,
    numero_cuenta_origen TEXT NOT NULL,

    banco_destino        TEXT,
    numero_cuenta_destino TEXT,

    importe             BIGINT NOT NULL CHECK (importe > 0),
    estado              TEXT NOT NULL DEFAULT 'PENDIENTE'
                            CHECK (estado IN ('PENDIENTE', 'COMPLETADA', 'RECHAZADA', 'REVERTIDA')),
    motivo_rechazo      TEXT,

    creado_en           TIMESTAMPTZ NOT NULL DEFAULT now(),
    actualizado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- La clave de idempotencia es única por banco origen: evita
    -- procesar dos veces la misma operación si el emisor reintenta
    -- una request que no recibió confirmación (ver doc 01/02).
    UNIQUE (banco_origen, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_operaciones_cuenta_origen
    ON operaciones (banco_origen, numero_cuenta_origen);
