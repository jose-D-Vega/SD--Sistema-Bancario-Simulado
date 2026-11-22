-- Usuarios de banca personal (login de clientes). Esto NO forma parte del
-- contrato de interoperabilidad (docs 01/02/03): es una extensión propia de
-- este banco para que el frontend tenga con quién autenticar a una persona,
-- distinta del banco_id+numero_cuenta que sí viaja entre bancos.
CREATE TABLE IF NOT EXISTS usuarios (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT NOT NULL,
    documento       TEXT NOT NULL,
    nombre          TEXT NOT NULL,
    email           TEXT,
    telefono        TEXT,
    password_hash   TEXT NOT NULL,
    creado_en       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (username),
    UNIQUE (documento)
);
