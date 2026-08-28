# CRM de Servicios Técnicos Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial para instalación, inspección, mantenimiento y reparación en
campo, desde la solicitud hasta la evidencia de cierre.

```env
ABERA_MODE=template
ABERA_TEMPLATE=servicios-tecnicos-co
```

## Modelo de trabajo

Clientes y prospectos se relacionan con Activos, Solicitudes, Cotizaciones y
Órdenes de trabajo. `items-cotizacion` desglosa mano de obra, materiales y
desplazamientos; `tareas-orden` actúa como checklist de ejecución y evidencia.

La cadena cliente → activo → solicitud → cotización → orden permite consultar
historial y rentabilidad sin duplicar información. La ubicación Geometry se
usa en clientes, activos y órdenes para un mapa operativo nativo. Agenda,
ventanas de atención, tiempos reales, firma y mantenimiento siguiente viven en
la misma ficha de trabajo.

## Automatización

Los workflows calculan SLA, asignan responsables, crean la orden desde una
cotización aceptada, previenen cruces de técnico, validan tareas antes del
cierre y actualizan el próximo mantenimiento. Correo y Slack son ejemplos
desactivados, sin credenciales ni tráfico externo.

## Demo y producción

El showroom aporta 100 registros sintéticos por módulo principal y 100 por cada
auxiliar. El modo plantilla instala la estructura vacía. Los CSV/YAML están
versionados y no requieren generación durante el arranque.

La versión 2.0.0 requiere una base nueva.
