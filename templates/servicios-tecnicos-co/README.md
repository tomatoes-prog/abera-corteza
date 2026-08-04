# CRM de Servicios Técnicos Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Plantilla para empresas de instalación, mantenimiento, inspección y reparación:

```env
ABERA_TEMPLATE=servicios-tecnicos-co
```

Solicitud → Diagnóstico → Cotización → Aceptada → Programada → Ejecutada →
Cerrada o Perdida.

La orden de trabajo contiene la cita y alimenta el calendario. Los técnicos son
usuarios con el rol `tecnico-servicio`. La plantilla no usa mapas, facturación
ni mensajería externa por defecto.

El sembrador API usa los prefijos `SERV-DEMO-` y `serv.demo.`.
