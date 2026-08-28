# CRM Centro de Contacto Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial de televentas para priorizar contactos autorizados, gestionar
intentos y callbacks, y medir conversión por campaña.

```env
ABERA_MODE=template
ABERA_TEMPLATE=centro-contacto-co
```

## Modelo de trabajo

Contactos y leads se relacionan con Campañas mediante Registros de campaña.
Cada registro conserva agente, prioridad, intentos y próximo contacto; cada
Llamada añade disposición e historial; una calificación puede originar una
Oportunidad. No se necesita un módulo intermedio adicional.

La ficha documenta autorización, canales permitidos, consulta RNE y bloqueo de
contacto. La experiencia se distingue por Mi cola, callbacks vencidos, panel de
supervisión y funnel por campaña. Los campos de grabación y transcripción son
referencias opcionales; la plantilla no llama servicios externos.

## Automatización

Los workflows validan autorización/RNE, distribuyen registros, procesan
disposiciones, respetan el máximo de intentos, programan callbacks y crean
oportunidades. Los ejemplos de correo y Slack están desactivados y sin secretos.

## Demo y producción

El showroom aporta 100 registros relacionados por cada uno de los cinco
módulos. El modo plantilla instala la solución vacía. Los CSV/YAML son estáticos
y versionados; el materializador de desarrollo no forma parte del runtime.

La gestión RNE y de autorización apoya el control operativo, pero no sustituye
la revisión jurídica de cada organización. La versión 2.0.0 requiere una base
nueva.
