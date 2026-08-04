# Changelog

Los cambios propios de esta distribución se documentan en este archivo. El
historial general de Corteza se mantiene en la documentación oficial del
proyecto de origen.

## Unreleased

### Added

- Bootstrap OAuth agnóstico para integraciones externas durante
  `corteza provision`.
- Generación criptográficamente segura de la contraseña administrativa y del
  secreto OAuth.
- Entrega cifrada mediante RSA-OAEP-SHA256 y AES-256-GCM, con publicación
  atómica y permisos `0600`.
- Roles estables `abera-mcp-basic`, `abera-mcp-pro`, `abera-mcp-admin` y
  `abera-consulta`, con matrices RBAC de privilegio mínimo.
- Recuperación idempotente de entregas parciales y cambios explícitos de modo.
- Registro en Action Log de ascensos del cliente OAuth al modo `admin`.
- Documentación del contrato neutral en `BOOTSTRAP_OAUTH.md`.

### Changed

- Los workflows de asignación usan identidades técnicas internas con permisos
  mínimos y ya no dependen de los permisos del usuario que crea el registro.
- Soporte actualiza el estado de SLA cada 15 minutos y crea renovaciones de
  forma idempotente al entrar en la ventana de 60 días.
- Los gráficos de barras optimizan etiquetas categóricas tanto en el eje
  horizontal como en el vertical.
- El sembrador inmobiliario usa `http://localhost:8080` de forma
  predeterminada.

### Security

- Los secretos OAuth creados por el bootstrap se almacenan en la base como hash
  SHA-256 y se verifican en tiempo constante.
- El payload de credenciales no se escribe en texto plano ni se incluye en logs.
- Se rechazan claves RSA menores de 3072 bits, rutas relativas, enlaces
  simbólicos, archivos manipulados y colisiones con recursos sin marcador.
- Los tokens fuerzan exclusivamente el rol MCP seleccionado y sus reglas
  deniegan explícitamente los permisos sensibles heredados del rol global
  `authenticated`.

### License

- Se añadieron `NOTICE` y documentación explícita de las modificaciones
  distribuidas bajo Apache License 2.0.
