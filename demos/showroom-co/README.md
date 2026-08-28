# Showroom comercial Abera

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See the repository `LICENSE`.

Este bundle se selecciona con `ABERA_MODE=demo` y
`ABERA_DEMO_BUNDLE=showroom-co`. Sobre un volumen nuevo aprovisiona las seis
plantillas v2 de `templates.list`, sus usuarios demo y 100 registros
sintéticos relacionados por cada módulo principal.

Cada solución conserva navegación y herramientas propias; el showroom no
duplica un tablero genérico seis veces. Los datos provienen de CSV/YAML
versionados y no requieren un generador ni un contenedor auxiliar al arrancar.

Este bundle se usa exclusivamente para demostración y pruebas. Una instalación
de cliente puede seleccionar una sola plantilla con `ABERA_MODE=template` o
arrancar Corteza limpio omitiendo tanto el modo como la plantilla. Ninguna de
esas dos opciones recibe datos sintéticos.
