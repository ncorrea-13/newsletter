# Despliegue

[English](DEPLOYMENT.md)

Usá Docker Compose con `compose.prod.yaml`. Para Podman, seguí [Podman rootless](#podman-rootless). Configurá DNS, TLS y proxies de entrada fuera de este proyecto.

## Imagen y servicios

`compose.prod.yaml` usa `ghcr.io/ncorrea-13/newsletter-bot:latest` y `ghcr.io/ncorrea-13/newsletter-egress-proxy:latest`. El workflow `image` publica ambas en GHCR para `linux/amd64` y `linux/arm64`, con tags `latest` y el SHA del commit. Para fijar una versión reproducible, reemplazá `latest` por el mismo SHA en las tres referencias del archivo.

La imagen del bot incluye `newsletter`, Codex CLI, Claude Code y Supercronic. El contenedor usa UID/GID `10001`. `egress-proxy` conecta la red interna del bot con internet; `newsletter-feeds` sirve los XML y se activa con el perfil `feeds`.

```sh
export NEWSLETTER_DIR="$PWD"
export FEED_PORT=8080
docker compose -f compose.prod.yaml pull
```

`NEWSLETTER_DIR` apunta al directorio del despliegue. `FEED_PORT` determina el puerto del host. Exportá ambas variables antes de cada sesión de comandos o configuralas en `.env`.

## Directorios y permisos

```text
config/          defaults.json y newsletters/*.json, solo lectura
prompts/         un <slug>.md por newsletter, solo lectura
data/            historial, estado y feeds, lectura y escritura
auth/            sesiones de los agentes, lectura y escritura
```

Desde `NEWSLETTER_DIR`, copiá los ejemplos si `config/` y `prompts/` no existen. No copies sobre directorios existentes: podés sobrescribir archivos. El newsletter de ejemplo está desactivado y usa un prompt de prueba.

```sh
cp -r examples/config config
cp -r examples/prompts prompts
mkdir -p data/public auth/codex auth/claude
chmod 0700 auth auth/codex auth/claude
```

En Docker sin remapeo de usuarios, el UID/GID `10001` necesita permiso de escritura en `data/` y `auth/`, y acceso de lectura a `config/` y `prompts/`. Ajustá ownership o ACL según el host; cambiar ownership puede afectar el acceso de tu usuario. Con Docker rootless o remapeo de usuarios, comprobá primero el mapeo de UID/GID.

Mantené estos cuatro directorios bajo `NEWSLETTER_DIR`. Protegé las credenciales y respaldá `data/` y `auth/` por separado.

## Autenticación

Autenticá cada proveedor que vayas a usar desde una terminal interactiva.

```sh
docker run --rm -it -v "$NEWSLETTER_DIR/auth:/auth" --entrypoint codex ghcr.io/ncorrea-13/newsletter-bot:latest login --device-auth
docker run --rm -it --network host -v "$NEWSLETTER_DIR/auth:/auth" --entrypoint claude ghcr.io/ncorrea-13/newsletter-bot:latest auth login
```

Claude usa host networking para el callback de autenticación. En Docker Desktop, comprobá la compatibilidad de esa red con el host. Una sesión puede dejar de funcionar si la copiás a otro host. Si fijaste otro tag en Compose, usá ese mismo tag para autenticar.

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot check-agents
```

`check-agents` consulta ambos proveedores y falla si alguno no está autenticado o disponible. Ejecutá una edición para probar generación y búsqueda web.

## Validación y arranque

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot validate-config
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-crontab
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-feeds
docker compose -f compose.prod.yaml run --rm newsletter-bot run <slug>
```

Revisá la configuración, el horario, la edición y los XML antes de activar el scheduler. Una ejecución real consume uso del proveedor y no reemplaza una edición existente de la misma fecha. Probá la investigación y las fuentes con el proveedor elegido dentro de la imagen final.

```sh
docker compose -f compose.prod.yaml up -d newsletter-bot
docker compose -f compose.prod.yaml --profile feeds up -d newsletter-feeds
```

El primer comando arranca también `egress-proxy`. El scheduler valida la configuración y genera el crontab al iniciar. Reiniciá el contenedor después de cambiar JSON o prompts.

## Red y feeds

El bot usa una red interna y configura Squid como proxy HTTP y HTTPS. Squid acepta solicitudes desde la red `agent`, restringe los puertos a 80 y 443 y deniega destinos privados definidos en `docker/squid/squid.conf`. Verificá el comportamiento de red y de cada proveedor en el entorno de destino.

`newsletter-feeds` sirve solo `GET` y `HEAD` sobre XML de `data/public/`. `FEED_PORT` determina el puerto del host y `FEED_BIND_ADDRESS` la interfaz; esta última es `127.0.0.1` por defecto. Para acceso desde otro host, elegí una IP privada y restringí el acceso con un firewall. Cualquier exposición pública, DNS, túnel o cambio de puertos requiere aprobación explícita.

## Podman rootless

Usá el mismo archivo de producción con el override de Podman:

```sh
podman compose -f compose.prod.yaml -f compose.podman.yaml pull
podman compose -f compose.prod.yaml -f compose.podman.yaml run --rm newsletter-bot validate-config
```

El override configura `slirp4netns` para el servicio de feeds. Creá los directorios como se indicó arriba y ajustá sus propietarios dentro del namespace, tras revisar el impacto en los archivos existentes:

```sh
podman unshare chown -R 10001:10001 data auth
```

Ese cambio de ownership puede impedir que tu usuario escriba en esos directorios; usá `podman unshare` cuando lo necesites. Para autenticar, usá `podman run`; agregá `--network slirp4netns` para Codex y conservá `--network host` para Claude. El UID del contenedor necesita permiso de escritura en los directorios montados.

```sh
podman compose -f compose.prod.yaml -f compose.podman.yaml up -d newsletter-bot
podman compose -f compose.prod.yaml -f compose.podman.yaml --profile feeds up -d newsletter-feeds
```

Antes del arranque, validá configuración, permisos, autenticación y una edición real con el mismo archivo Compose. Comprobá que historial y sesiones persistan al recrear el contenedor.
