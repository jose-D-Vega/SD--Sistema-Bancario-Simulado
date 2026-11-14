# Nota técnica: nombres de constraints sin nombre en CockroachDB (migración 0003)

**Fecha:** 2026-11-08
**Alcance:** `backend/migrations/0003_ampliar_estados_operaciones.{up,down}.sql`

## Resumen

Durante una revisión del código se marcó como *posible bug* la migración
`0003_ampliar_estados_operaciones.up.sql`, porque hace:

```sql
ALTER TABLE operaciones DROP CONSTRAINT check_estado;
```

sobre una constraint que en `0002_create_operaciones.up.sql` se definió
**sin nombre**, inline en la columna:

```sql
estado TEXT NOT NULL DEFAULT 'PENDIENTE'
    CHECK (estado IN ('PENDIENTE', 'COMPLETADA', 'RECHAZADA', 'REVERTIDA')),
```

La sospecha era que el nombre autogenerado no iba a ser `check_estado`,
siguiendo la convención de PostgreSQL (`<tabla>_<columna>_check`, p. ej.
`operaciones_estado_check`), lo que haría fallar el `DROP CONSTRAINT` al
correr la migración contra una base limpia.

**Conclusión: no es un bug.** La migración corre bien tal cual está.

## Por qué la sospecha era razonable pero incorrecta

CockroachDB **no sigue la misma convención que PostgreSQL** para nombrar
constraints `CHECK` sin nombre explícito. Mientras que PostgreSQL usa el
patrón `<tabla>_<columna>_check`, CockroachDB usa `check_<columna>` para
un `CHECK` de una sola columna sin nombre — tanto si se escribe inline en
la definición de la columna como si se escribe como una línea aparte en
el `CREATE TABLE`.

Esto está documentado oficialmente por Cockroach Labs y es un
comportamiento estable (se verifica igual desde la documentación de la
v19.2 hasta la documentación "stable" actual): el ejemplo oficial de
`SHOW CONSTRAINTS` muestra una tabla `orders` con columnas `priority` y
`status`, cada una con un `CHECK` sin nombre, y los nombres generados son
exactamente `check_priority` y `check_status`.

Fuente: [CockroachDB Docs — SHOW CONSTRAINTS](https://www.cockroachlabs.com/docs/stable/show-constraints)

## Verificación empírica en este proyecto

No fue posible levantar una instancia real de CockroachDB desde el
entorno donde se hizo la revisión inicial (sin acceso de red a
`binaries.cockroachdb.com` ni a Docker Hub), así que la verificación
final se hizo contra la base real del proyecto, ya con las migraciones
0001 a 0004 aplicadas:

```bash
docker exec -it sudameris-cockroachdb ./cockroach sql --insecure --database=sudameris \
  -e "SHOW CONSTRAINTS FROM operaciones;"
```

Resultado (resumido):

```
 table_name  | constraint_name  | constraint_type |                      details
--------------+------------------+------------------+----------------------------------------------------
 operaciones | check_estado_v2 | CHECK            | CHECK (estado IN ('PENDIENTE', 'RECIBIDA', ...))
 operaciones | check_importe    | CHECK            | CHECK (importe > 0)
 operaciones | check_tipo       | CHECK            | CHECK (tipo IN (...))
```

- `check_estado_v2` existe → la migración 0003 corrió con éxito (si el
  `DROP CONSTRAINT check_estado` hubiera fallado, esta constraint no
  existiría y la migración habría quedado "dirty").
- `check_importe` y `check_tipo`, ambos CHECK sin nombre creados en la
  migración 0002 y nunca tocados después, confirman el patrón
  `check_<columna>` de forma independiente.

Con esto, el caso queda cerrado: **las migraciones 0001 a 0004 son
correctas tal cual están** y no requieren cambios.

## Qué hacer si esto se repite con otro motor de base de datos

Si en algún momento el proyecto migra de CockroachDB a PostgreSQL "real"
(u otro motor), este mismo patrón **sí podría romperse**, porque la
convención de nombres por defecto no es la misma entre motores. La forma
de evitar este tipo de duda a futuro — sin que haga falta memorizar la
convención de cada motor — es nombrar explícitamente cualquier
constraint que después se vaya a referenciar en una migración posterior:

```sql
estado TEXT NOT NULL DEFAULT 'PENDIENTE'
    CONSTRAINT operaciones_estado_check
    CHECK (estado IN ('PENDIENTE', 'COMPLETADA', 'RECHAZADA', 'REVERTIDA')),
```

No se aplicó este cambio en 0002 porque ya está aplicada en la base de
todo el equipo y tocarla retroactivamente no trae ningún beneficio real
(el proyecto no va a cambiar de motor). Queda solo como recomendación
para constraints *nuevas* que se agreguen de ahora en adelante y que se
sepa de antemano que van a necesitar un `DROP CONSTRAINT` más adelante.
