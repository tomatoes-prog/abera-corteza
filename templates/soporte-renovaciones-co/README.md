# CRM Soporte y Renovaciones Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial para resolver casos dentro del SLA y convertir el conocimiento
de soporte en retención, renovación y expansión de ingresos.

```env
ABERA_MODE=template
ABERA_TEMPLATE=soporte-renovaciones-co
```

## Modelo de trabajo

Clientes se relacionan con Contratos, Casos, Interacciones y Renovaciones.
`problemas-conocidos` agrupa incidentes con causa y solución compartida; se ve
desde el caso sin añadir complejidad a la operación diaria.

Un caso puede depender de un caso padre y registra impacto, urgencia, nivel de
escalamiento, SLA, causa raíz, evidencias y satisfacción. Las interacciones
pueden aportar primera respuesta o contexto de renovación. La experiencia se
distingue por Mi cola, panel de SLA, ficha 360° del cliente, problemas conocidos
y pipeline de valor recurrente en riesgo.

## Automatización

Los workflows enrutan casos, calculan y escalan SLA, registran primera respuesta,
crean renovaciones 60 días antes, calculan riesgo y actualizan el contrato al
renovar. Las horas son calendario. Los ejemplos de correo y Slack están
desactivados, sin credenciales ni tráfico externo.

## Demo y producción

El showroom aporta 100 registros sintéticos por módulo principal y 20 problemas
conocidos relacionados. El modo plantilla instala la solución vacía. Los datos
son CSV/YAML versionados y no se materializan durante el arranque.

La versión 2.0.0 requiere una base nueva.
