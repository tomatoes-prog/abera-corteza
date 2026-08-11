# CRM de Admisiones Educativas Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Gestiona el recorrido desde la primera solicitud de información hasta la
matrícula:

```env
ABERA_TEMPLATE=admisiones-educativas-co
```

Interesado → Contactado → Calificado → Solicitud iniciada → Solicitud completa
→ Admitido → Matriculado o No matriculado.

Los campos de integración permiten conectar formularios, portales o un LMS por
las APIs estándar de Compose. No se envían datos a servicios externos.

`scripts/seed-demo.ps1` crea datos sintéticos identificados con `EDU-DEMO-` y
`edu.demo.` para poder ejecutarse nuevamente sin duplicados.
