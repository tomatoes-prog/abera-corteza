# Traducciones y atribución

## Español

El contenido de `locale/es` se incorporó desde el repositorio oficial de
traducciones de Corteza:

- Repositorio: <https://github.com/cortezaproject/corteza-locale>
- Rama: `2024.9.x`
- Commit de origen: `57b1f2403207c44055ebce19d95cedd5573f39df`
- Licencia de origen: Apache License 2.0

La traducción oficial disponible para esta versión era parcialmente traducida.
Este repositorio completa las claves ausentes y mantiene inglés como idioma de
fallback defensivo. Las traducciones conservan la misma estructura YAML que
espera Corteza; no se renombran aplicaciones, claves, handles, APIs ni demás
identificadores técnicos.

La fuente oficial contiene algunos archivos YAML vacíos. Se normalizaron como
mapas YAML vacíos (`{}`) para que el parser de locales del servidor no falle
durante el arranque; sus claves continúan resolviéndose mediante la herencia
del inglés.

## Traducciones complementarias de este repositorio

Las traducciones adicionales de `locale/es` fueron redactadas para este
repositorio y se mantienen bajo Apache License 2.0, igual que el proyecto
original. No sustituyen la atribución de Corteza ni la licencia de origen.

Esta ampliación cubre menús, acciones, búsqueda, notificaciones, mensajes
generales y pasos de workflow de las aplicaciones One, Admin, Compose y
Workflow. Se conservaron las claves, la sintaxis YAML y todos los
placeholders (`{{...}}`, `{...}` y variables técnicas).

La estructura española contiene todas las claves presentes en los locales
ingleses incluidos en esta versión. Los nombres propios y términos técnicos
que perderían precisión al traducirse se conservan sin cambios. En la interfaz
de Compose, el concepto técnico `namespace` se presenta al usuario como
«espacio de trabajo», sin modificar la clave ni los identificadores internos.

## Cambios de este repositorio

Este repositorio configura `es` como idioma predeterminado del contenedor y
mantiene `en` como fallback. La variable `CORTEZA_DEFAULT_LOCALE` permite
modificar el idioma inicial en tiempo de ejecución sin recompilar la imagen.
También se aplica a las páginas anónimas de autenticación, que se renderizan
en el servidor antes de que la configuración del cliente web esté disponible.
Una vez autenticado, el idioma preferido del perfil del usuario continúa
teniendo prioridad.

El código original de Corteza y las traducciones incorporadas conservan sus
avisos de atribución y se distribuyen bajo Apache License 2.0. La licencia
completa se encuentra en [`LICENSE`](LICENSE). Cualquier redistribución de la
imagen o de las traducciones debe conservar dicha licencia y este documento.

## Construcción

```bash
docker build -t abera-corteza:es .
docker run --name abera-corteza -p 8080:80 -v corteza-data:/data abera-corteza:es
```

Con esta configuración, la aplicación queda disponible en `http://localhost:8080`
y los enlaces OAuth conservan el mismo puerto. Si se publica con otro dominio o
puerto, deben sobrescribirse `DOMAIN` y `DOMAIN_WEBAPP` al ejecutar el contenedor.

La imagen configura explícitamente la base de datos SQLite en
`/data/corteza.db`, dentro del volumen declarado, y ejecuta las actualizaciones
del esquema en cada arranque (`UPGRADE_ALWAYS=true`). Esto evita el valor
predeterminado en memoria de Corteza, que no es persistente y puede provocar
errores OAuth `500` cuando desaparecen tablas como `auth_sessions`,
`auth_oa2tokens` o `reminders`.

En despliegues locales publicados por HTTP, la protección CSRF permanece
activa y valida el token del formulario. El servidor marca estas solicitudes
como HTTP plano cuando la cookie de sesión no usa el atributo `Secure`, para
que el origen local (`http://localhost`) no sea tratado erróneamente como un
origen distinto de HTTPS. En despliegues TLS se conserva la validación estricta
de origen.

SQLite es apropiado para esta ejecución local y de evaluación. Para producción
se recomienda sobrescribir `DB_DSN` con una base de datos persistente soportada
por Corteza y mantener una estrategia externa de copias de seguridad.

Para cambiar el idioma predeterminado del contenedor:

```bash
docker run -e CORTEZA_DEFAULT_LOCALE=en abera-corteza:es
```
