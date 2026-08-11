# CRM Inmobiliario Colombia

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See
[`../../LICENSE`](../../LICENSE).

Plantilla declarativa y autocontenida para administrar leads, inmuebles,
citas, actividades y negociaciones. Está diseñada para instalarse en una base
de datos nueva con:

```env
ABERA_TEMPLATE=inmobiliaria-co
```

## Asesores

Los asesores son usuarios de Corteza asignados al rol `asesor-comercial`. El
workflow `asignar-lead-menor-carga` asigna cada lead sin responsable al usuario
activo con menos leads en estados no terminales. Los empates se resuelven por
ID de usuario ascendente.

Los usuarios con el rol `coordinador-comercial` pueden consultar, crear,
actualizar y eliminar registros de los cinco módulos. Si no existen asesores,
el lead queda sin responsable y aparece en el bloque correspondiente de la
página de inicio.

## Decisiones de alcance

- Los criterios de búsqueda forman parte del lead.
- El propietario se guarda como nombre y teléfono dentro del inmueble.
- Un lead y una cita pueden referenciar varios inmuebles.
- La cita conserva un único resultado general.
- Departamento usa una lista colombiana fija y ciudad es texto libre.
- No se incluyen propietarios, perfiles de búsqueda, campañas, fuentes,
  barrios, zonas ni tablas intermedias.

## Actualizaciones

La versión 1.0.0 se aprovisiona únicamente sobre bases nuevas. No cambie
`ABERA_TEMPLATE` sobre un volumen existente ni importe la plantilla con
`--replace-existing`.

## Datos de demostración mediante API

El script `scripts/seed-demo.ps1` utiliza OAuth y las APIs públicas de Corteza.
No escribe directamente en la base de datos y es idempotente para los registros
que identifica con los prefijos `DEMO-` y `demo.crm.`.

De forma predeterminada crea:

- 36 inmuebles distribuidos en doce ciudades colombianas;
- 100 leads con presupuesto, criterios de búsqueda y entre uno y tres inmuebles
  de interés;
- 60 citas, 100 actividades y 35 negociaciones relacionadas.

El cliente OAuth debe permitir el flujo `authorization_code`, incluir el scope
`api` y ser de confianza para poder ejecutar el flujo sin interacción manual.
Las credenciales y el secreto se reciben en memoria mediante parámetros o
variables de entorno y nunca deben guardarse en el repositorio:

```powershell
$env:ABERA_CLIENT_SECRET = Read-Host 'Secreto OAuth'
$env:ABERA_ADMIN_PASSWORD = Read-Host 'Contraseña del administrador'

.\scripts\seed-demo.ps1 `
  -BaseUrl 'http://localhost:8080' `
  -ClientId '<ID-del-cliente-OAuth>'

Remove-Item Env:ABERA_CLIENT_SECRET
Remove-Item Env:ABERA_ADMIN_PASSWORD
```
