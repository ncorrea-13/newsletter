# Instalación y despliegue

## Destino elegido

Destino de referencia: Podman rootless. La imagen debe seguir siendo compatible con Docker/Compose. Esta decisión no autoriza cambios de red, DNS ni Caddy.

Caddy queda fuera de la imagen y del Compose del bot. Puede servir los XML desde un montaje compartido o desde una copia sincronizada de `/data/public`; el segundo caso requiere sincronización operada fuera del bot. No implementar sincronización implícita.

Feeds pueden quedar internos o hacerse públicos mediante configuración externa de Caddy. La exposición pública requiere aprobación explícita. No abrir puertos, activar Funnel, ni modificar Caddy o DNS.

## Requisitos del despliegue previsto

- Imagen multistage: builder `golang:1-trixie`, runtime `debian:trixie-slim`.
- Supercronic instalado desde release oficial fijada y verificada.
- Codex CLI y Claude Code aún no se incluyen: instalación, credenciales y generación no interactiva requieren validación dentro de la imagen final.
- Usuario de ejecución no root; no instalar Go ni agentes en host.
- Persistencia fuera de capa efímera: historia, estado, XML y sesiones de agente.
- Sin puertos, privilegios, socket Docker ni proceso HTTP de Go.
- Versión/digest y checksums fijados tras validar releases reales.

El Compose del repo solo ejecuta el bot. No publica puertos ni inicia Caddy. Podman Compose usa un proveedor externo; Docker Compose funciona con el mismo archivo.

## Directorios persistentes propuestos

```text
/srv/newsletter-bot/
├── history/<slug>/
├── state/{runs,locks,events}/
├── public/
└── auth/{codex,claude}/

# Configuración editable, montada read-only en el contenedor
config/defaults.json
config/newsletters/<slug>.json
prompts/<slug>.md
```

Solo `public/` se sirve con Caddy. Entre hosts, acordar montaje compartido o sincronización sin hacer accesible historia, estado o autenticación.

El usuario de contenedor debe poder escribir en mounts privados y públicos del bot. Verificar UID/GID y mapeo rootless con el runtime elegido; no asumir que `chown` del host coincide con UID del contenedor.

Montar `config/` y `prompts/` desde una ruta editable del host, read-only en el contenedor. El entrypoint valida los JSON, genera el crontab, comprueba sintaxis y arranca Supercronic. Tras editar horarios o prompts, reiniciar el contenedor; no se necesita rebuild de imagen ni cambio de código.

Arranque local: `podman compose up --build -d` o `docker compose up --build -d`. Preparar `data/` con permisos para UID 10001 antes de iniciar. Compose no publica puertos; `data/public/` queda disponible para un Caddy configurado aparte.

## Caddy externo

El servicio Caddy existente debe montar únicamente el directorio de XML, en solo lectura. Si Caddy está en otro host, usar almacenamiento compartido o sincronización explícita. No exponer el padre, habilitar listado, ni servir `history/`, `state/` o credenciales. Definir hostname y exposición antes de editar Caddy.

## Autenticación de agentes (pendiente)

No autenticar todavía: imagen no incluye agentes. Tras validar su integración, autenticar dentro del contenedor con HOME y mounts definitivos:

```sh
docker compose exec newsletter-bot codex login --device-auth
docker compose exec newsletter-bot codex login status
docker compose exec newsletter-bot bash
```

Dentro de shell, iniciar `claude` y completar el flujo oficial soportado. Reemplazar `docker compose` por `podman compose` cuando corresponda.

Antes de automatizar, verificar rutas reales de credenciales, persistencia al recrear contenedor, permisos del usuario, expiración y reautenticación. No copiar sesiones al build ni guardar tokens en `.env` o Git. Suscripciones pueden tener límites de cuota/uso.

## Caddy, DNS y TLS

No configurar DNS ni TLS hasta decidir si el feed es interno o público. No asumir que otras aplicaciones o túneles habilitan exposición pública automáticamente.

## Verificación antes de scheduler

1. Construir imagen para arquitectura real del host.
2. Confirmar usuario no root y permisos de mounts.
3. Verificar versiones y ayuda de ambos agentes y Supercronic.
4. Autenticar y consultar estado local.
5. Ejecutar prueba no interactiva mínima con cada agente.
6. Confirmar búsqueda y lectura web con fuentes verificables.
7. Probar generación con agente falso y validar XML.
8. Recrear contenedor y comprobar JSON, feeds y sesiones.
9. Verificar horario local con Supercronic sin ejecutar en lote jobs perdidos.
10. Habilitar publicación y scheduler tras decisión de exposición.
