# Asistente de IA de Abera

Esta distribución añade un punto de integración opcional para un agente de IA
sin incorporar el agente, el MCP ni recursos de nube dentro del repositorio de
Corteza. El código propio está aislado en:

- `server/abera/assistant`: validación de identidad y puente HTTP seguro.
- `client/abera-assistant`: widget Vue compartido por todas las aplicaciones.

Los cambios se distribuyen bajo Apache License 2.0, igual que el resto de este
fork. El repositorio no contiene CloudFormation, SDKs de proveedores ni
credenciales de clientes.

## Flujo de confianza

```text
Navegador autenticado
  -> /api/system/assistant/* (Bearer de Corteza)
  -> puente Go
  -> /v1/internal/* del agente (IAM SigV4 + API key)
  -> MCP Corteza (cuenta técnica limitada + contexto de actor firmado)
```

El navegador nunca recibe la API key, credenciales AWS, el token del agente ni
las credenciales del MCP. La conversación se aísla por `tenantID` y por el ID
estable del usuario de Corteza; el correo solo se usa como dato descriptivo.

El endpoint `GET /api/system/assistant/auth/context` también permite que el
Lambda Authorizer de una API pública valide un Bearer de Corteza sin compartir
`AUTH_JWT_SECRET`. Comprueba que el usuario siga activo y que la membresía
directa al rol `abera-ai-user` continúe vigente. Una asignación o revocación de
ese rol requiere refrescar el token del navegador; la revocación en base de
datos se aplica inmediatamente.

## Variables de Corteza

El asistente está desactivado por defecto.

```env
ABERA_AI_ENABLED=true
ABERA_AI_AGENT_URL=https://api-id.execute-api.us-east-1.amazonaws.com/prod
ABERA_AI_TENANT_ID=cliente-acme
ABERA_AI_REQUIRED_ROLE=abera-ai-user
ABERA_AI_AUTH_MODE=aws_iam
ABERA_AI_AWS_REGION=us-east-1
ABERA_AI_API_KEY_FILE=/run/secrets/abera_agent_api_key
ABERA_AI_TIMEOUT=95s
```

`ABERA_AI_AGENT_URL` es la URL base o URL de etapa; Corteza añade
`/v1/internal`. Las rutas de secretos deben ser absolutas, apuntar a archivos
regulares y no pueden ser enlaces simbólicos. En `aws_iam`, las credenciales se
obtienen de la cadena segura de entorno, rol de tarea ECS o IMDSv2. La política
IAM debe conceder únicamente `execute-api:Invoke` sobre la ruta interna del
cliente.

Para desarrollo local se puede reemplazar la autenticación de salida:

```env
ABERA_AI_AUTH_MODE=local_token
ABERA_AI_LOCAL_TOKEN_FILE=/run/secrets/abera_agent_internal_token
```

El valor debe ser aleatorio y tener al menos 32 caracteres. La API key es
opcional en local; en API Gateway es obligatoria para cuota y medición, pero no
constituye autenticación.

## Acceso de usuarios

El aprovisionamiento base crea el rol `abera-ai-user`, pero no lo asigna a
nadie. Un administrador debe añadir directamente los usuarios autorizados. El
rol solo habilita el widget y el puente; no amplía el RBAC de Compose, System o
Automation.

El widget aparece en Admin, Compose, Discovery, One, Privacy, Reporter y
Workflow. Ofrece:

- conversaciones persistentes por usuario;
- respuestas SSE en tiempo real;
- hasta cinco PDF, TXT, DOCX, CSV, XLSX, PNG o JPG por turno, con 10 MB por archivo;
- historial y eliminación de conversaciones;
- confirmación explícita y de un solo uso antes de herramientas mutables, recuperable después de recargar la página;
- diseño responsive, navegación por teclado y compatibilidad con tema oscuro.

## Rutas del puente

El puente solo expone una lista cerrada; no acepta destinos o métodos
arbitrarios:

- `GET|POST /api/system/assistant/conversations`
- `GET|DELETE /api/system/assistant/conversations/{conversationID}`
- `POST /api/system/assistant/conversations/{conversationID}/messages`
- `POST /api/system/assistant/conversations/{conversationID}/approvals/{approvalID}`
- `POST /api/system/assistant/conversations/{conversationID}/files/prepare`
- `PUT /api/system/assistant/conversations/{conversationID}/files/{fileID}/content`
- `POST /api/system/assistant/conversations/{conversationID}/files/{fileID}/complete`

Los IDs se validan como UUID, los cuerpos JSON se limitan a 1 MB y las cargas
a 10 MB. El puente no sigue redirecciones, no reenvía el Bearer del navegador y
solo copia encabezados de respuesta expresamente permitidos.

En producción, SigV4 firma también las cabeceras de tenant, usuario, roles,
correlación y API key que consume el agente. Por tanto, alterar ese contexto en
tránsito invalida la firma completa.

## Despliegue externo

El repositorio del agente define dos superficies:

- `/v1/internal/*`: IAM SigV4 y API key, para el puente Go.
- `/v1/*`: Bearer de Corteza validado por Lambda Authorizer y API key, para
  consumidores externos futuros.

El repositorio de automatización debe crear API Gateway REST regional, Lambdas,
DynamoDB, S3, usage plan, roles y secretos. Esos recursos no forman parte de
este fork. En producción, use TLS, tokens cortos, concurrencia reservada, logs
sin payloads y análisis antimalware antes de marcar un archivo como disponible.
