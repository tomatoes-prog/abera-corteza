# Plantillas de solución

Copyright 2026 Abera/Corteza contributors

Licensed under the Apache License, Version 2.0. See [`LICENSE`](LICENSE).

## Selección

La imagen contiene plantillas declarativas en `templates/<identificador>`.
Una instalación nueva puede seleccionar una plantilla mediante
`ABERA_MODE=template` y `ABERA_TEMPLATE`:

```bash
docker run --name corteza-inmobiliaria \
  -p 8080:80 \
  -e ABERA_MODE=template \
  -e ABERA_TEMPLATE=inmobiliaria-co \
  -v corteza-inmobiliaria-data:/data \
  abera-corteza:es
```

El punto de entrada añade el directorio `provision` de la plantilla a
`PROVISION_PATH`. Corteza importa el espacio de trabajo, módulos, páginas,
gráficos, roles, permisos, traducciones y workflows durante el primer
aprovisionamiento de una base vacía.

La plantilla seleccionada se guarda en `/data/.abera-template`. Los reinicios
pueden omitir la variable porque el valor persistido se reutiliza. Para evitar
mezclar modelos de datos, el contenedor rechaza una plantilla diferente sobre
el mismo volumen.

La versión inicial de este mecanismo está diseñada para instalaciones nuevas.
No se debe usar `--replace-existing` automáticamente sobre instancias con datos.
Las actualizaciones futuras se publicarán como migraciones versionadas.

## Demo autoinicializada

El modo demo es un producto separado de las instalaciones de cliente:

```bash
docker run --name abera-corteza \
  -p 8080:80 \
  -e ABERA_MODE=demo \
  -e ABERA_DEMO_BUNDLE=showroom-co \
  -v abera-corteza-demo-data:/data \
  abera-corteza:demo
```

Sobre un volumen nuevo, `showroom-co` instala las seis plantillas del catálogo,
usuarios funcionales y registros sintéticos relacionados para todos los
módulos. Los datos se cargan mediante el aprovisionamiento nativo de Envoy antes
de habilitar la aplicación; no requieren OAuth, scripts posteriores ni un
contenedor sembrador.

Los reinicios no duplican datos. El marcador `/data/.abera-deployment` impide
convertir un volumen demo en una instalación de cliente o cambiar la plantilla
de un volumen existente. Si `ABERA_DEMO_BUNDLE` se omite en un reinicio, el
bundle se recupera desde ese marcador persistido.

## Catálogo incluido

La imagen contiene seis soluciones seleccionables:

| Identificador | Solución | Objetivo comercial |
| --- | --- | --- |
| `inmobiliaria-co` | CRM inmobiliario | Convertir leads interesados en inmuebles en citas y negociaciones. |
| `automotriz-co` | Concesionario y taller | Vender vehículos y conservar clientes mediante posventa. |
| `admisiones-educativas-co` | Admisiones educativas | Llevar prospectos desde la consulta inicial hasta la matrícula. |
| `servicios-tecnicos-co` | Servicios de campo | Cotizar, programar y ejecutar instalaciones, mantenimientos y reparaciones. |
| `centro-contacto-co` | Televentas | Mejorar contacto efectivo, callbacks, calificación y conversión por campaña. |
| `soporte-renovaciones-co` | Soporte y retención | Cumplir SLA y convertir la atención en renovación o venta adicional. |

Cada solución tiene cinco módulos de negocio, páginas de inicio y operación,
gráficos alimentados por registros, roles funcionales, permisos y workflows.
Los handles públicos se mantienen estables para permitir integraciones y
migraciones posteriores.

Cada plantilla aprovisiona además un usuario técnico de tipo `sys` y un rol
interno de mínimo privilegio. Los workflows de asignación y las tareas
programadas usan esa identidad mediante `runAs`, por lo que no dependen de los
permisos del usuario que crea el registro. Estas identidades no son cuentas de
acceso humano.

En modo demo, cada uno de los 30 módulos contiene entre 1 y 100 registros. Los
catálogos pequeños, como campañas o programas, usan cantidades plausibles; los
módulos operativos usan volúmenes mayores para que calendarios, funnels y
gráficos sean representativos.

## Estructura

Cada plantilla es autónoma:

```text
templates/
  identificador/
    manifest.yaml
    README.md
    CHANGELOG.md
    NOTICE
    provision/
      roles.yaml
      namespace.yaml
      modules.yaml
      pages.yaml
      charts.yaml
      workflows.yaml
      permissions.yaml
      locale-es.yaml
    scripts/
      seed-demo.ps1
    demo/
      provision/
        records.yaml
        <datos-sinteticos>.csv
```

Los identificadores deben contener únicamente letras minúsculas, números y
guiones. Los handles internos deben ser estables porque forman parte de las
referencias de Envoy, los filtros, los workflows y la API.

`templates/_shared/seed-api.ps1` se conserva para pruebas de API y cargas
manuales explícitas. El primer arranque de la demo utiliza los datasets
declarativos de `demo/provision`.

## Datos de demostración

Los sembradores usan exclusivamente OAuth y las APIs estándar de System y
Compose. Son idempotentes: identifican sus registros sintéticos mediante
códigos, correos o identificadores externos estables, por lo que una segunda
ejecución no duplica datos.

Estos sembradores no forman parte del arranque normal de la demo; sirven para
pruebas CRUD, integraciones y regeneración controlada de instancias existentes.

Ejemplo:

```powershell
$env:ABERA_CLIENT_SECRET = '<secreto-oauth>'
$env:ABERA_ADMIN_EMAIL = 'admin@ejemplo.local'
$env:ABERA_ADMIN_PASSWORD = '<contraseña>'

.\templates\automotriz-co\scripts\seed-demo.ps1 `
  -BaseUrl http://localhost:8080 `
  -ClientId '<id-cliente-oauth>'
```

Los scripts no escriben directamente en SQLite ni llaman servicios de
telefonía, WhatsApp, correo, mapas o facturación. Las credenciales se reciben
por parámetros o variables de entorno y no se guardan en los archivos.

## Manifiesto

`manifest.yaml` declara como mínimo:

- `id`, `name` y `version`;
- `minimumCortezaVersion`;
- `defaultLocale`;
- `license`;
- recursos incluidos.

La licencia Apache 2.0 de la plantilla no cubre automáticamente conjuntos de
datos externos. Cada fuente adicional debe documentar separadamente su origen
y licencia.

Todos los datos de demostración incluidos son sintéticos. Cada plantilla
incluye `NOTICE`, `README.md` y `CHANGELOG.md`, y referencia la licencia
Apache 2.0 del repositorio.
