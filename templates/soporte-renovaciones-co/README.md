# CRM Soporte y Renovaciones Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Plantilla para atención al cliente y gestión de ingresos recurrentes:

```env
ABERA_TEMPLATE=soporte-renovaciones-co
```

El SLA inicial utiliza horas calendario. Un workflow ejecutado cada 15 minutos
marca los casos como `En riesgo` al consumir el 75 % del plazo y como
`Incumplido` al vencer la primera respuesta o la resolución.

Una tarea diaria crea de forma idempotente las renovaciones de contratos
activos que entran en la ventana de 60 días antes del vencimiento. Casos
críticos, incumplimientos y CSAT bajo elevan automáticamente el riesgo.

Las tareas programadas y la asignación por carga se ejecutan con el usuario
interno `automatizacion-soporte`, que solo tiene permisos de lectura de
usuarios y roles y CRUD sin eliminación dentro de esta plantilla.

Los campos de integración aceptan identificadores de correo, chat, voz o
plataformas externas, pero ningún workflow contacta proveedores por defecto.

El sembrador API usa los prefijos `SUP-DEMO-` y `sup.demo.`.
