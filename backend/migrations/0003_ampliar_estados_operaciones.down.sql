ALTER TABLE operaciones DROP CONSTRAINT operaciones_estado_check;

ALTER TABLE operaciones ADD CONSTRAINT operaciones_estado_check
    CHECK (estado IN ('PENDIENTE', 'COMPLETADA', 'RECHAZADA', 'REVERTIDA'));
