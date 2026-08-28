# CRM Inmobiliario Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Solución inicial para conectar captación, inventario, visitas, seguimiento y
cierre comercial sin crear módulos administrativos innecesarios.

```env
ABERA_MODE=template
ABERA_TEMPLATE=inmobiliaria-co
```

## Modelo de trabajo

- **Leads** conserva contacto, autorización, búsqueda, presupuesto, prioridad,
  asesor y varios inmuebles de interés.
- **Inmuebles** reúne ubicación nativa, disponibilidad, características,
  fotografías y datos básicos del propietario.
- **Citas** vincula un lead, su asesor y uno o varios inmuebles en una visita.
- **Actividades** funciona como agenda de llamadas, mensajes y seguimientos.
- **Negociaciones** representa el pipeline, valores, probabilidad, próximo paso,
  cierre real y comisión estimada.

Las fichas 360° muestran las relaciones en ambos sentidos. El inventario usa
mapa y listado; la agenda usa calendario; el cierre usa organizador por etapa.

## Automatización

Los workflows activos asignan leads por menor carga, crean el primer
seguimiento, notifican al asesor, convierten una visita positiva en negociación
y sincronizan el cierre. Los ejemplos de correo y Slack están desactivados y
no contienen credenciales ni realizan llamadas externas.

## Demo y producción

El showroom contiene 100 registros por módulo principal, relacionados y
completamente sintéticos. Una instalación con `ABERA_MODE=template` recibe la
estructura vacía. Los CSV/YAML demo están versionados; no se generan durante el
arranque. `scripts/seed-demo.ps1` queda disponible solo para pruebas API
manuales e idempotentes.

La versión 2.0.0 se instala únicamente en una base nueva. No cambie de plantilla
ni use reemplazo automático sobre una instancia con datos.
