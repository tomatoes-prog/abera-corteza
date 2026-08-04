# Bootstrap OAuth cifrado para integraciones externas

Este mecanismo permite que una instalación nueva de Corteza cree un
administrador y un cliente OAuth `client_credentials` sin intervención manual.
El contrato es independiente del sistema que despliegue la imagen: Corteza
recibe una clave pública y publica un único sobre cifrado. El sistema externo
conserva la clave privada, descifra la entrega y decide dónde almacenar las
credenciales.

El código y esta documentación se distribuyen bajo Apache License 2.0. Consulte
[`LICENSE`](LICENSE), [`NOTICE`](NOTICE) y [`CHANGELOG.md`](CHANGELOG.md).

## Configuración

El bootstrap se habilita cuando se define cualquiera de sus variables. Cuando
está habilitado, todas las variables obligatorias deben estar presentes:

```env
ABERA_TEMPLATE=inmobiliaria-co

ABERA_INITIAL_ADMIN_EMAIL=admin@cliente.com
ABERA_INITIAL_ADMIN_NAME=Administrador Cliente
ABERA_INITIAL_ADMIN_HANDLE=admin-cliente

ABERA_MCP_MODE=basic

ABERA_BOOTSTRAP_PUBLIC_KEY_FILE=/run/abera/public-key.pem
ABERA_BOOTSTRAP_OUTPUT_FILE=/run/abera/bootstrap.enc.json
```

`ABERA_INITIAL_ADMIN_HANDLE` es opcional. Si se omite, se deriva de la parte
local del correo. Las demás variables del bloque son obligatorias.

Las rutas deben ser absolutas. La clave debe ser RSA de al menos 3072 bits y
estar codificada como PEM PKIX o PKCS#1. El directorio padre del archivo de
salida debe existir antes de iniciar Corteza.

`AUTH_PROVISION_SUPER_USER` y este bootstrap son mutuamente excluyentes. Una
instalación que configure ambos detendrá el aprovisionamiento con un error.

Ejemplo para crear una clave temporal:

```sh
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 -out private-key.pem
openssl pkey -in private-key.pem -pubout -out public-key.pem
```

La clave privada no se monta en Corteza.

## Entrega

En el primer `corteza provision` exitoso se crean:

- El usuario administrador solicitado, con idioma preferido `es`.
- Una contraseña aleatoria imprimible de 32 caracteres.
- Los roles `abera-mcp-basic`, `abera-mcp-pro`, `abera-mcp-admin` y
  `abera-consulta`.
- El cliente OAuth `abera-mcp`, ligado al administrador mediante
  `impersonateUser`.
- Una entrega cifrada en `ABERA_BOOTSTRAP_OUTPUT_FILE`.

El contenido descifrado tiene este contrato:

```json
{
  "version": 1,
  "admin": {
    "userID": "123",
    "email": "admin@cliente.com",
    "handle": "admin-cliente",
    "password": "generada-por-corteza"
  },
  "oauth": {
    "clientID": "456",
    "clientSecret": "generado-por-corteza",
    "grantType": "client_credentials",
    "scope": "api",
    "tokenURL": "/auth/oauth2/token",
    "mode": "basic",
    "explicitApprovalRequired": true
  }
}
```

El archivo publicado contiene únicamente:

```json
{
  "version": 1,
  "algorithm": "RSA-OAEP-256+A256GCM",
  "encryptedKey": "...",
  "nonce": "...",
  "ciphertext": "..."
}
```

El cifrado usa una clave AES aleatoria de 256 bits en modo GCM. Esa clave se
envuelve con RSA-OAEP y SHA-256. El valor ASCII `abera-bootstrap-v1` se usa como
datos autenticados adicionales de AES-GCM. Todos los campos binarios usan
Base64 URL sin relleno.

El archivo se escribe primero como temporal, se sincroniza y se publica por
renombrado atómico con permisos `0600`. Se rechazan enlaces simbólicos y nunca
se sobrescribe un archivo inesperado.

La contraseña se almacena en la base únicamente mediante el hash de
credenciales normal de Corteza. El secreto OAuth se guarda como un hash SHA-256
de un valor aleatorio de 384 bits; la validación OAuth compara el secreto
recibido con ese hash en tiempo constante. El texto plano solo está en memoria
durante el aprovisionamiento y dentro del payload cifrado.

El cliente conserva el rol del modo seleccionado en `permittedRoles` y
`forcedRoles`. Esta combinación elimina del token los roles interactivos del
administrador —incluido `super-admin`— y garantiza que el token incluya
exactamente el rol MCP aun durante el calentamiento inicial de las cachés de
pertenencias.

## Modos y permisos

| Capacidad | `basic` | `pro` | `admin` |
|---|---:|---:|---:|
| Consultar espacios, módulos, campos, páginas, gráficos y registros | Sí | Sí | Sí |
| Crear y modificar registros | Sí | Sí | Sí |
| Eliminar registros | No | Sí | Sí |
| Crear y consultar workflows | Sí | Sí | Sí |
| Actualizar workflows | No | Sí | Sí |
| Eliminar workflows | No | No | Sí |
| Ejecutar workflows | No | No | Sí |
| Crear y modificar módulos | No | No | Sí |
| Crear y consultar usuarios y roles | No | No | Sí |
| Asignar miembros a `abera-consulta` | No | No | Sí |

Ningún modo puede eliminar módulos, usuarios o roles; administrar clientes
OAuth, configuración global o reglas RBAC; ni asignar roles administrativos,
de bypass o roles MCP. El modo `admin` solo recibe `members.manage` sobre el rol
exacto `abera-consulta`.

Las denegaciones sensibles se declaran de forma explícita en los roles MCP.
Esto es necesario porque la instalación base de Corteza concede algunas
operaciones de descubrimiento al rol global `authenticated`; las reglas del rol
MCP tienen precedencia y mantienen el contrato anterior.

El valor `explicitApprovalRequired` documenta que el consumidor debe solicitar
confirmación humana antes de mutar datos. Corteza aplica la matriz RBAC incluso
si otro consumidor llama directamente a la API.

Por compatibilidad histórica, los controladores REST estándar de esta versión
pueden representar una denegación de autorización como HTTP `200` con un objeto
`error` en el cuerpo. Los errores del middleware de autenticación sí usan
códigos HTTP propios. Los consumidores deben comprobar tanto el código HTTP
como `error`; este bootstrap no cambia globalmente el contrato REST de Corteza.

## Idempotencia y recuperación

La base conserva un marcador con versión, IDs, modo, huella de la clave pública,
hash y sobre cifrado. No conserva el payload en texto plano.

- Un reinicio completo no regenera la contraseña ni el secreto.
- Si el proceso se interrumpe después de guardar la base y antes de publicar el
  archivo, el siguiente arranque vuelve a escribir exactamente el mismo sobre.
- Si el archivo fue consumido y eliminado después de completar el bootstrap,
  los reinicios no lo recrean.
- Si existe un usuario, rol o cliente reservado sin marcador, el arranque se
  detiene para evitar enlazar recursos ajenos.
- Una clave pública diferente para el mismo volumen se rechaza.
- Un archivo existente diferente del sobre esperado se considera manipulado y
  no se sobrescribe.

Cambiar `ABERA_MCP_MODE` en un volumen ya inicializado actualiza solamente la
pertenencia del usuario, `permittedRoles` y `forcedRoles` del cliente OAuth. No
rota las credenciales. Los ascensos a `admin` se registran en el Action Log. Los
tokens emitidos previamente conservan sus permisos hasta expirar; configure
`AUTH_OAUTH2_ACCESS_TOKEN_LIFETIME` con una duración corta y reinicie el
consumidor después del cambio.

El sobre representa las credenciales y el modo de su creación inicial. Un
cambio posterior de modo no vuelve a publicar secretos ni modifica ese archivo.

## Responsabilidad del sistema externo

El sistema que despliega Corteza debe:

1. Generar el par de claves temporal.
2. Montar solo la clave pública y un directorio de salida escribible.
3. Esperar a que termine el aprovisionamiento.
4. Leer y descifrar el sobre con la clave privada.
5. Guardar las credenciales en su mecanismo de custodia.
6. Eliminar o archivar de forma segura el archivo de entrega y destruir la clave
   privada temporal cuando su política lo permita.
7. Configurar el consumidor OAuth con `clientID`, `clientSecret`, `scope=api` y
   la URL absoluta formada a partir de `tokenURL`.

La implementación de esos pasos pertenece al orquestador y no forma parte de
este repositorio.
