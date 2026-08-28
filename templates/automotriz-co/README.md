# CRM Automotriz Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial para unir ventas de vehículos y posventa en un historial
continuo por prospecto y por unidad de inventario.

```env
ABERA_MODE=template
ABERA_TEMPLATE=automotriz-co
```

## Modelo de trabajo

Los cinco módulos principales son Prospectos, Vehículos, Pruebas de manejo,
Oportunidades y Órdenes de servicio. `actividades-comerciales` y
`tareas-servicio` son auxiliares: aparecen en las fichas 360° como historial y
checklist, pero no sobrecargan la navegación.

Un prospecto puede interesarse en varios vehículos. Cada vehículo conserva sus
pruebas, oportunidad, venta y posventa; una orden documenta kilometraje,
diagnóstico, evidencias, aceptación y próxima intervención. La experiencia usa
agenda de pruebas, pipeline comercial, inventario y puesto de trabajo de taller.

## Automatización

Los workflows asignan prospectos, crean la actividad inicial, previenen cruces
de pruebas, crean oportunidades tras una prueba positiva y actualizan venta y
mantenimiento. Los ejemplos de correo y Slack permanecen desactivados y sin
credenciales.

## Demo y producción

El showroom aporta 100 registros sintéticos por módulo principal y 100 por cada
auxiliar, con relaciones completas. El modo plantilla instala la misma solución
vacía. Los datos son CSV/YAML versionados y no se generan durante el arranque;
el sembrador API se reserva para pruebas manuales idempotentes.

La versión 2.0.0 requiere una base nueva.
