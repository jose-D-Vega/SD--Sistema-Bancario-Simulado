ALTER TABLE operaciones DROP CONSTRAINT check_estado_v2;

ALTER TABLE operaciones ADD CONSTRAINT check_estado
    CHECK (estado IN ('PENDIENTE', 'COMPLETADA', 'RECHAZADA', 'REVERTIDA'));
