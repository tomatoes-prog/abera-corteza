# CRM Centro de Contacto Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Plantilla para campañas de televentas:

```env
ABERA_TEMPLATE=centro-contacto-co
```

Cargado → Intentado → Contactado → Calificado → Oportunidad → Venta o No
interesado.

El módulo `registros-campana` conserva la participación de un contacto en una
campaña, incluyendo prioridad, intentos, agente y callback. Los campos
`externalID`, `grabacionURL` y `transcripcionURL` están preparados para una
integración posterior con telefonía. No se realizan llamadas externas.

El sembrador API usa los prefijos `CALL-DEMO-` y `call.demo.`.
