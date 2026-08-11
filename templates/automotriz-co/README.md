# CRM Automotriz Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Plantilla comercial para concesionarios y talleres que conecta la captación
de prospectos con el inventario, las pruebas de manejo, la negociación y la
posventa.

```env
ABERA_TEMPLATE=automotriz-co
```

## Funnel

Nuevo → Contactado → Calificado → Prueba de manejo → Cotización →
Negociación → Vendido o Perdido.

Los asesores son usuarios con el rol `asesor-automotriz`. El rol
`jefe-comercial-automotriz` supervisa ventas y `coordinador-taller` administra
las órdenes de servicio.

## Integraciones

La plantilla funciona sin proveedores externos. Los campos `externalID`,
`proveedorExterno`, `estadoIntegracion` y `ultimoErrorIntegracion` permiten
conectar portales, telefonía o sistemas DMS mediante las APIs estándar de
Compose. Ningún workflow realiza solicitudes externas por defecto.

## Datos de demostración

`scripts/seed-demo.ps1` crea por API 40 vehículos, 100 prospectos, 60 pruebas
de manejo, 35 oportunidades y 50 órdenes de servicio. Usa los prefijos
`AUTO-DEMO-` y `auto.demo.` para poder ejecutarse nuevamente sin duplicar.
