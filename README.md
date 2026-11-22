# SD--Sistema-Bancario-Simulado
Proyecto final de sistemas distribuidos - 2026 - Grupo Sudameris

Simulación de un banco (Sudameris) para la cátedra: 4 grupos, cada uno con
su propio banco (backend + frontend + base propia), interoperando entre sí
para transferencias y una red de cajeros compartida según el contrato que
define el BCP simulado (el grupo líder del proyecto). Este repo es
**solo nuestro banco** — el contrato en sí (reglamento, especificación de
interoperabilidad, OpenAPI, plan de pruebas) vive en un repo aparte,
provisto por el BCP.

## Estructura

```
.
├── backend/   # API del banco: todo el contrato del BCP + banca personal
│              # (usuarios, login, movimientos, alias). Ver backend/README.md.
├── frontend/  # Prototipo visual (Vue), con un modelo bancario simulado
│              # en el propio navegador (localStorage), sin conectarse
│              # todavía al backend real. Ver frontend/README.md.
├── cajero/    # Interfaz del cajero automático -- vacía por ahora.
└── docs/      # Notas técnicas internas del equipo (no el contrato del
               # BCP en sí, que es un repo aparte).
```

## Estado del proyecto

- **`backend/`** implementa el contrato obligatorio del BCP (cuentas,
  transferencias internas e interbancarias con reconciliación automática,
  recepción de créditos, cajero, firma Ed25519) y, además, una capa de
  banca personal propia (usuarios con login, perfil, cuentas propias con
  alias, historial de movimientos) para que el frontend tenga con qué
  trabajar. Tiene tests automatizados (`go test ./...`) cubriendo ambas
  partes. El detalle completo, cómo levantarlo y qué falta están en
  [`backend/README.md`](backend/README.md).
- **`frontend/`** es un prototipo de interfaz hecho por separado, con un
  modelo bancario enteramente local (no llama al backend todavía). Sirve
  para validar la experiencia de usuario; conectarlo al backend real es
  el trabajo que sigue. Detalle en [`frontend/README.md`](frontend/README.md).
- **`cajero/`** (la interfaz de cajero automático) todavía no se empezó.
- Los 20 casos del plan de pruebas del BCP (documento 04) corrieron contra
  un segundo backend de prueba local, pero no se ejecutaron todavía de
  punta a punta contra los bancos de los otros 3 grupos.
