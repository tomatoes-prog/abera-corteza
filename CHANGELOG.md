# Changelog

Los cambios propios de esta distribución se documentan en este archivo. El
historial general de Corteza se mantiene en la documentación oficial del
proyecto de origen.

## Unreleased

### Added

- Versiones 2.0.0 productivas de las seis plantillas comerciales, con fichas
  360°, relaciones, reportes, workflows y experiencias sectoriales distintas.
- Siete módulos auxiliares para historiales, checklists, evidencias y problemas
  conocidos, integrados en las fichas sin sobrecargar la navegación.
- Descripciones empresariales y de edición para todos los módulos y campos.
- Validadores de cuadrícula, relaciones, datos sintéticos, workflows y
  neutralidad del materializador de desarrollo.
- Widget de asistente de IA compartido por todas las aplicaciones web, con
  conversaciones, streaming SSE, archivos y aprobaciones explícitas.
- Puente Go autenticado con lista cerrada de rutas, aislamiento por tenant,
  validación en vivo del rol `abera-ai-user` y soporte IAM SigV4/API Gateway.
- Endpoint neutral de contexto para validar tokens Corteza sin compartir claves
  internas de firma.
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

- El showroom usa archivos CSV/YAML estáticos con 100 registros relacionados
  por cada módulo principal; no ejecuta generadores durante el arranque.
- Una instancia sin `ABERA_MODE` ni `ABERA_TEMPLATE` queda como Corteza limpio
  en español y no aprovisiona espacios de negocio.
- Los workflows de asignación usan identidades técnicas internas con permisos
  mínimos y ya no dependen de los permisos del usuario que crea el registro.
- Soporte actualiza el estado de SLA cada 15 minutos y crea renovaciones de
  forma idempotente al entrar en la ventana de 60 días.
- Los gráficos de barras optimizan etiquetas categóricas tanto en el eje
  horizontal como en el vertical.
- El sembrador inmobiliario usa `http://localhost:8080` de forma
  predeterminada.

### Security

- El navegador no recibe secretos del agente, MCP, API Gateway ni AWS; el
  puente elimina el Bearer de usuario antes de construir la solicitud interna.
- Las rutas de secretos rechazan valores relativos y enlaces simbólicos; las
  cargas y cuerpos tienen límites independientes.
- SigV4 vincula criptográficamente el contexto del actor y las aprobaciones
  pendientes reaparecen al recargar una conversación.
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
