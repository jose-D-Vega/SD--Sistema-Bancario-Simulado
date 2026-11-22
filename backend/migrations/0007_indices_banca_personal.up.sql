-- Separada de 0006 a propósito: CockroachDB no permite agregar una columna
-- y crear un índice sobre ella (sobre todo uno parcial, con WHERE) dentro
-- de la misma transacción. Para cuando esta migración corre, las columnas
-- de 0006 ya quedaron confirmadas y "public", así que acá sí se puede.

-- Alias único por banco cuando está definido (NULL no cuenta como duplicado).
CREATE UNIQUE INDEX IF NOT EXISTS idx_cuentas_alias
    ON cuentas (banco_id, alias) WHERE alias IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cuentas_usuario ON cuentas (usuario_id);
