# Instalación y despliegue

## Destino elegido

Destino de referencia: Podman rootless. La imagen debe seguir siendo compatible con Docker/Compose. Esta decisión no autoriza cambios de red, DNS ni Caddy.

Caddy queda fuera de la imagen. Compose ofrece `newsletter-feeds`, un segundo servicio con el mismo binario que sirve XML estáticos desde `/data/public`, montado en solo lectura. Caddy externo hace proxy al puerto del host; no requiere compartir archivos entre hosts.

Feeds pueden quedar internos o hacerse públicos mediante configuración externa de Caddy. La exposición pública requiere aprobación explícita. No abrir puertos, activar Funnel, ni modificar Caddy o DNS.

## Requisitos del despliegue previsto

- Imagen multistage: builder `golang:1-trixie`, runtime `debian:trixie-slim`.
- Supercronic instalado desde release oficial fijada y verificada.
- Codex CLI `0.167.0` y Claude Code `2.1.319` se instalan desde fuentes oficiales; la build comprueba sus versiones.
- Usuario de ejecución no root; no instalar Go ni agentes en host.
- Persistencia fuera de capa efímera: historia, estado, XML y sesiones de agente.
- El scheduler no publica puertos. El servicio de feeds ofrece HTTP de solo lectura; sin privilegios ni socket Docker.
- Versión/digest y checksums fijados tras validar releases reales.

El Compose no inicia Caddy. El servicio de feeds es opcional mediante perfil `feeds`. Usa `podman-compose` con el override `compose.podman.yaml` en Podman; Docker Compose usa solo `compose.yaml`.

## Directorios persistentes propuestos

```text
auth/{codex,claude}/          # local: ./auth, production: absolute path outside the repo
data/
├── history/<slug>/
├── state/{runs,locks,events}/
└── public/

# Configuración editable, montada read-only en el contenedor
config/defaults.json
config/newsletters/<slug>.json
prompts/<slug>.md
```

`NEWSLETTER_AUTH_DIR` acepta ruta relativa al compose o absoluta. Para producción, usar una ruta absoluta fuera del checkout, por ejemplo `/srv/newsletter-bot/auth`.

El servicio de feeds monta solo `data/public/`, en solo lectura. No accede a historia, estado ni autenticación.

El usuario del contenedor debe poder escribir en `NEWSLETTER_AUTH_DIR` y los datos. Mantener permisos privados; comprobar UID/GID con el runtime rootless y no asumir que `chown` del host coincide con UID 10001.

Montar `config/` y `prompts/` desde una ruta editable del host, read-only en el contenedor. El entrypoint valida los JSON, genera el crontab, comprueba sintaxis y arranca Supercronic. Tras editar horarios o prompts, reiniciar el contenedor; no se necesita rebuild de imagen ni cambio de código.

Preparar `data/` con permisos de escritura para UID 10001. Arranque del scheduler: `podman-compose -f compose.yaml -f compose.podman.yaml up --build -d` o `docker compose up --build -d`. Para incluir RSS, agregar `--profile feeds` antes de `up`.

El puerto del servicio de feeds publica `8080` solo en `127.0.0.1` por defecto. Para Caddy en otro host, definir `FEED_BIND_ADDRESS` en `.env` con una interfaz privada alcanzable desde Caddy, y restringir acceso en la red. No ligar a `0.0.0.0` salvo decisión explícita.

Rutas aceptadas: `GET /<slug>.xml` y `HEAD /<slug>.xml`. No se listan directorios ni se sirven archivos que no terminen en `.xml`. Ejemplo de sitio Caddy:

```caddyfile
feeds.example.test {
	reverse_proxy <private-host-address>:8080
}
```

## Caddy externo

El Caddy existente debe hacer proxy solo al puerto privado del servicio de feeds. No apuntarlo al contenedor scheduler ni exponer servicios auxiliares. Definir hostname y exposición antes de editar Caddy.

## Autenticación de agentes

Con `NEWSLETTER_AUTH_DIR=./auth`, crear el directorio privado en el checkout y construir la imagen:

```sh
mkdir -p auth/codex auth/claude
chmod 0700 auth auth/codex auth/claude
# Rootless Podman:
podman unshare chown -R 10001:10001 auth
podman-compose -f compose.yaml -f compose.podman.yaml build newsletter-bot
```

`auth/` queda ignorado por Git y fuera del contexto de build. Para Docker rootful, ejecutar `sudo chown -R 10001:10001 auth`. Para producción, apuntar `NEWSLETTER_AUTH_DIR` a una ruta absoluta fuera del repo y aplicar el mismo ownership.

`.env` solo define la ruta; nunca guardes tokens ahí.

El override Podman usa `slirp4netns` para redes rootless y evita depender de `nftables` para la red bridge predeterminada. Aplicarlo también en los comandos siguientes.

Autenticar desde terminal interactiva en el contenedor. Claude abre un callback local en el puerto `1455`; publicarlo en loopback del host o el navegador no podrá conectarse. Codex puede evitar el callback con `--device-auth`:

```sh
podman run --rm -it --network slirp4netns -v "$PWD/auth:/auth" --entrypoint codex localhost/newsletter-bot:local login --device-auth
podman run --rm -it --network slirp4netns -p 127.0.0.1:1455:1455 -v "$PWD/auth:/auth" --entrypoint claude localhost/newsletter-bot:local
```

Si el puerto `1455` sigue ocupado, detener el contenedor de login anterior (`podman ps`) y reintentar. Si el callback no llega, usar `--network host` en lugar de `--network slirp4netns -p ...`. Sin `--device-auth`, Codex también usa el callback `1455` y necesita el mismo publish.

Para Docker: `docker run --rm -it -v "$PWD/auth:/auth" --entrypoint codex newsletter-bot:local login --device-auth` y `docker run --rm -it -p 127.0.0.1:1455:1455 -v "$PWD/auth:/auth" --entrypoint claude newsletter-bot:local`. Si Codex solicita autenticación por dispositivo, abrir la URL en tu equipo e ingresar el código mostrado; el contenedor no tiene navegador.

Comprobar autenticación local sin iniciar generación:

```sh
podman-compose -f compose.yaml -f compose.podman.yaml run --rm newsletter-bot check-agents
```

Para Docker: `docker compose run --rm newsletter-bot check-agents`. `check-agents` ejecuta `codex login status` y `claude auth status`; no genera contenido ni imprime la salida de autenticación del proveedor.

## Ejecutar una edición

Después de autenticar y revisar el prompt local, ejecutar una vez el slug configurado:

```sh
podman-compose -f compose.yaml -f compose.podman.yaml run --rm newsletter-bot run <slug>
```

El proveedor elegido en el JSON investigará la web y consumirá uso. La edición del día no se reemplaza: el comando rechaza duplicados antes de llamar al proveedor.

Para revisar configuración y última edición sin consumir uso:

```sh
podman-compose -f compose.yaml -f compose.podman.yaml run --rm newsletter-bot status
```

Para Docker: `docker compose run --rm newsletter-bot status`. No imprime títulos, resúmenes, prompts ni credenciales.

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
