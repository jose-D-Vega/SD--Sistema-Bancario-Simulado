# SUDAMERIS · esta vez, funcional.

Interfaz inspirada en el PDF del equipo, con acceso local validado y cuentas corrientes y cajas de ahorro. Servir `dist` con cualquier servidor HTTP estático. Vue y el generador QR están incluidos localmente.

Los usuarios iniciales `maria`, `lucia`, `martin` y `ana` usan la contraseña `Esmeralda123!`. El archivo adjunto de datos de prueba detalla números, alias, saldos y escenarios. Abrir `?restablecer=1` restaura los datos iniciales de este navegador.

## Límites de implementación

Esta es una implementación de frontend con modelo bancario local para pruebas. Los datos persisten en localStorage por navegador, no en MariaDB. La validación de credenciales del cliente no es autenticación segura de producción. Las operaciones con otros bancos usan registros locales. La firma Ed25519, autorización real, sesiones de servidor, correo y sincronización multiusuario requieren backend. Evitar cambios concurrentes entre pestañas.

La cuenta corriente y la caja de ahorro pueden operar cuando están activas y tienen saldo suficiente. Las cuentas bloqueadas o cerradas rechazan débitos y créditos. Hay validación de importes enteros, saldo, límite diario, destinatarios registrados e idempotencia en el modelo local. Las transferencias pendientes reservan el importe y se mantienen en ese estado. El cajero ATM y la conciliación no forman parte de este cliente móvil.

El QR de recepción es un código legible con los datos de la cuenta; no pertenece a una red de pagos real. El pago usa el código de comercio `ESM-CANTINA-5500`. La consulta de ayuda se guarda localmente y no se envía. El sitio funciona en navegadores móviles; falta empaquetar y probar las aplicaciones nativas de Capacitor en Android/iOS.
