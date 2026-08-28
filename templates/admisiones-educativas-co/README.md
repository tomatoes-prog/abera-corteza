# CRM de Admisiones Educativas Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial para acompañar a una persona desde su primera consulta hasta
la matrícula, con visibilidad del programa y del avance documental.

```env
ABERA_MODE=template
ABERA_TEMPLATE=admisiones-educativas-co
```

## Modelo de trabajo

Los módulos principales son Prospectos, Programas, Solicitudes, Citas de
admisión y Matrículas. `requisitos-solicitud` modela el checklist documental y
`actividades-admision` conserva el seguimiento; ambos se presentan dentro de
las fichas y no como procesos aislados.

El prospecto registra programa principal y alternativas, condición de menor,
acudiente y autorizaciones. Programa, solicitud y matrícula comparten periodo y
permiten medir cupos, completitud y conversión. La experiencia combina agenda,
funnel, checklist y vista 360° por persona o programa.

## Automatización

Los workflows asignan prospectos, crean seguimiento, calculan completitud,
preparan la matrícula al admitir y recuperan solicitudes estancadas. Los
ejemplos externos de correo y Slack están desactivados y no contienen secretos.

## Demo y producción

El showroom aporta 100 registros sintéticos por módulo principal y 100 por cada
auxiliar. El modo plantilla instala la solución vacía. Los CSV/YAML se publican
ya materializados; ningún generador corre al iniciar la instancia.

Los datos de menores incluidos son ficticios y solo ilustran controles de
autorización; la plantilla no constituye asesoría ni certificación jurídica.
La versión 2.0.0 requiere una base nueva.
