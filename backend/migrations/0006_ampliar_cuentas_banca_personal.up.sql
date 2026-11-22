-- Extiende "cuentas" para la capa de banca personal (frontend), sin tocar
-- lo que exige el reglamento (doc 01 solo pide banco_id + numero_cuenta
-- para identificar una cuenta; esto es todo agregado propio del banco):
--
--   usuario_id: dueño de la cuenta. Nullable a propósito: una cuenta creada
--     directo por POST /cuentas (ej. la cuenta de un comercio para cobros
--     QR, o cuentas de prueba/seed) puede no tener un usuario asociado.
--   alias: identificador alternativo, local a este banco. El contrato de
--     interoperabilidad (doc 02) nunca comparte alias entre bancos, solo
--     banco_id + numero_cuenta — así que el alias solo sirve para resolver
--     destinos DENTRO de SUDAMERIS.
--   tipo: cuenta corriente / caja de ahorro. El reglamento lo deja
--     explícitamente fuera del mínimo obligatorio (doc 01, sección 8), lo
--     sumamos porque el frontend lo necesita para mostrarlo.
--
-- Los índices sobre estas columnas van en la migración 0007, aparte: en
-- CockroachDB no se puede agregar una columna y crear un índice (sobre
-- todo uno parcial) sobre ella en la misma transacción — el cambio de
-- esquema de la columna todavía no está "public" cuando el CREATE INDEX
-- intenta leerla, y falla con "column ... which is not public".
ALTER TABLE cuentas ADD COLUMN IF NOT EXISTS usuario_id UUID REFERENCES usuarios(id);

ALTER TABLE cuentas ADD COLUMN IF NOT EXISTS alias TEXT;

ALTER TABLE cuentas ADD COLUMN IF NOT EXISTS tipo TEXT NOT NULL
    DEFAULT 'CUENTA_CORRIENTE'
    CONSTRAINT cuentas_tipo_check CHECK (tipo IN ('CUENTA_CORRIENTE', 'CAJA_AHORRO'));
