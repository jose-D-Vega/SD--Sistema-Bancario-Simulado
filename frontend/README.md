# SUDAMERIS · esta vez, funcional.

Interfaz inspirada en el PDF del equipo, con acceso local validado, cuentas
corrientes y cajas de ahorro. Servir `dist` con cualquier servidor HTTP
estático. Vue y el generador QR están incluidos localmente.

## Cómo entrar

Contraseña para todos: `Esmeralda123!`

| Usuario (@) | Cédula | Alias cargado |
|---|---|---|
| `@maria` | 4567890 | CI `4567890` (cuenta corriente) y celular (caja de ahorro) |
| `@lucia` | 4567891 | celular `595981222222` (su CI la usa el Banco 2) |
| `@martin` | 4567892 | ninguno (sin celular cargado) |
| `@ana` | 4567893 | CI `4567893` (cuenta bloqueada y otra cerrada) |

Se puede entrar con el **@usuario** o con la **cédula**. Abrir
`?restablecer=1` restaura los datos iniciales de este navegador.

## Usuario (@) y alias son cosas distintas

- **Usuario (@)**: identidad **dentro de la app**. Sirve para entrar, para el
  saludo y para que te encuentren los contactos. **No sirve para transferir.**
- **Alias**: la **cédula o el celular** del titular. Es **uno por cuenta** y
  **único en todo el sistema**: si ese dato ya se usa en otro banco, no se
  puede elegir. Sirve para recibir transferencias desde cualquier banco sin
  pasar el número de cuenta.

Por eso una persona puede tener cuentas en los cinco bancos, pero no alias en
todos: sus datos (cédula y celular) alcanzan para dos alias como máximo.

## Cuentas

- Se puede tener **cuenta corriente y caja de ahorro a la vez** sin volver a
  registrarse: "Solicitar nuevo producto" en Inicio o en "Mis alias" (máximo 5
  por persona).
- El tipo se guarda como `CUENTA_CORRIENTE` / `CAJA_AHORRO` y el estado como
  `ACTIVA` / `BLOQUEADA` / `CERRADA`, igual que en la base del backend.
- Al abrir una cuenta nueva la app pide el alias enseguida; si ya no quedan
  datos disponibles, avisa y deja la cuenta sin alias.

## Límites de implementación

Esta es una implementación de frontend con modelo bancario local para pruebas.
Los datos persisten en `localStorage` por navegador, no en CockroachDB. La
validación de credenciales del cliente no es autenticación segura de
producción. Las operaciones con otros bancos usan registros locales. La firma
Ed25519, autorización real, sesiones de servidor, correo y sincronización
multiusuario requieren backend. Evitar cambios concurrentes entre pestañas.

Los cambios que necesita el backend para conectar esto son: abrir cuentas
adicionales, alias único entre bancos, formato del alias y resolución de
alias remotos.

La cuenta corriente y la caja de ahorro pueden operar cuando están activas y
tienen saldo suficiente. Las cuentas bloqueadas o cerradas rechazan débitos y
créditos. Hay validación de importes enteros, saldo, límite diario,
destinatarios registrados e idempotencia en el modelo local. Las transferencias
pendientes reservan el importe y se mantienen en ese estado. El cajero ATM y la
conciliación no forman parte de este cliente móvil.

El QR de recepción es un código legible con los datos de la cuenta; no
pertenece a una red de pagos real. El pago usa el código de comercio
`ESM-CANTINA-5500` y el alias `80012345`. La consulta de ayuda se guarda
localmente y no se envía. El sitio funciona en navegadores móviles; falta
empaquetar y probar las aplicaciones nativas de Capacitor en Android/iOS.
