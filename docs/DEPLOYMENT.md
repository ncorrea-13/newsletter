# Despliegue

## Destino

Podman rootless es el destino de referencia. La imagen también funciona con Docker Compose. El proyecto no configura DNS, TLS ni proxies: la exposición de los feeds se decide fuera del repo.

## Imagen

El workflow `image` publica en GHCR la imagen de cada push a `main`, para `linux/amd64` y `linux/arm64`, con los tags `latest` y `<sha>`. En despliegues conviene fijar el `<sha>` y no `latest`. Así el rollback es un cambio de tag.

La imagen trae el binario `newsletter`, Codex CLI, Claude Code y Supercronic. Las versiones están fijadas en el `Dockerfile`. El contenedor corre como usuario no root (UID/GID `10001`).

Para construir localmente:

```sh
podman compose -f compose.yaml -f compose.podman.yaml build newsletter-bot
```

Docker Compose usa solo `compose.yaml`.

## Directorios del host

```text
config/          defaults.json y newsletters/*.json, en solo lectura
prompts/         un <slug>.md por newsletter, en solo lectura
data/            historial, estado y feeds (lectura y escritura)
auth/            sesiones de Codex y Claude (lectura y escritura, privado)
```

`config/`, `prompts/` y `data/` no están en Git: son locales al despliegue. Respaldá `data/` y `auth/` por separado.

El usuario del contenedor (UID 10001) tiene que poder escribir en `data/` y `auth/`. Creá los directorios y asigná el ownership dentro del namespace de Podman:

```sh
mkdir -p data auth/codex auth/claude
chmod 0700 auth auth/codex auth/claude
podman unshare chown -R 10001:10001 data auth
```

Ese ownership impide que tu usuario del host escriba en `data/`. Para copiar archivos ahí, usá `podman unshare`.

## Compose de producción

En producción no se compila. El servicio usa `image:` con la imagen publicada.

```yaml
services:
  egress-proxy:
    build: docker/squid          # no hay imagen publicada; se construye desde el checkout
    image: newsletter-egress-proxy:local
    restart: unless-stopped
    networks:
      agent:
        ipv4_address: 172.28.10.10
      egress: {}

  newsletter-bot:
    image: ghcr.io/<owner>/newsletter-bot:<sha>
    init: true
    pids_limit: 128
    restart: unless-stopped
    depends_on:
      - egress-proxy
    environment:
      HTTP_PROXY: http://172.28.10.10:3128
      HTTPS_PROXY: http://172.28.10.10:3128
      http_proxy: http://172.28.10.10:3128
      https_proxy: http://172.28.10.10:3128
      NO_PROXY: localhost,127.0.0.1
    networks:
      - agent
    volumes:
      - ./config:/config:ro
      - ./prompts:/prompts:ro
      - ./data:/data
      - ${NEWSLETTER_AUTH_DIR:-./auth}:/auth

  newsletter-feeds:
    image: ghcr.io/<owner>/newsletter-bot:<sha>
    command: ["serve"]
    profiles: ["feeds"]
    init: true
    pids_limit: 128
    restart: unless-stopped
    network_mode: slirp4netns    # Podman rootless; en Docker, quitar esta línea
    environment:
      FEED_DIR: /data/public
      FEED_LISTEN_ADDR: :8080
    ports:
      - "${FEED_BIND_ADDRESS:-127.0.0.1}:${FEED_PORT:-8080}:8080"
    volumes:
      - ./data/public:/data/public:ro

networks:
  agent:
    internal: true               # sin ruta a internet ni a la LAN: el bot solo llega al proxy
    ipam:
      config:
        - subnet: 172.28.10.0/24
  egress: {}
```

El bot sale a internet solo por `egress-proxy`. Squid acepta la red `agent`, solo 80 y 443, y deniega destinos privados (`10/8`, `172.16/12`, `192.168/16`, `100.64/10`, loopback, link-local y similares). Los agentes usan `HTTPS_PROXY`, y `WebFetch` pasa por el mismo proxy.

Variables de `.env`:

```sh
NEWSLETTER_AUTH_DIR=./auth
FEED_BIND_ADDRESS=127.0.0.1   # IP privada del equipo, alcanzable desde el proxy
FEED_PORT=8080
```

Si el paquete de GHCR es privado, iniciá sesión antes con `podman login ghcr.io`.

Comandos habituales:

```sh
podman-compose -f compose.prod.yaml pull
podman-compose -f compose.prod.yaml up -d newsletter-bot
podman-compose -f compose.prod.yaml --profile feeds up -d newsletter-feeds
```

El servicio `newsletter-feeds` tiene `profiles: ["feeds"]`. Un `up` sin ese perfil no lo levanta.

## Autenticación de los agentes

Las sesiones se guardan en `auth/`. Cada host las crea una vez, desde una terminal interactiva.

Codex usa device auth. Su callback escucha en loopback dentro del contenedor, así que no publica ningún puerto:

```sh
podman run --rm -it --network slirp4netns -v "$PWD/auth:/auth" --entrypoint codex <imagen> login --device-auth
```

Claude usa un puerto de loopback aleatorio en cada ejecución. Por eso necesita host networking, solo para este paso:

```sh
podman run --rm -it --network host -v "$PWD/auth:/auth" --entrypoint claude <imagen> auth login
```

Para verificar sin generar contenido:

```sh
podman-compose -f compose.prod.yaml run --rm newsletter-bot check-agents
```

No copies `auth/` entre hosts. Las sesiones pueden dejar de funcionar fuera del equipo donde se crearon, y los tokens son credenciales.

## Exposición de los feeds

El servicio de feeds publica el puerto solo en la interfaz configurada. Por defecto es `127.0.0.1`.

Para que un proxy de otro host llegue al feed, `FEED_BIND_ADDRESS` debe ser una IP privada de este equipo. Nunca `0.0.0.0`. Además, un firewall debe permitir solo al proxy llegar al puerto.

Cualquier cambio de exposición pública (dominio, DNS, túnel o puertos abiertos) requiere aprobación explícita del responsable del despliegue. Una regla de proxy que apunte a un feed no reemplaza esa aprobación.

Ejemplo de proxy, con valores de ejemplo:

```caddyfile
feeds.example.com {
	reverse_proxy <IP_PRIVADA>:8080
}
```

El feed sirve solo `GET` y `HEAD` sobre `/<slug>.xml`. Cualquier otra ruta responde 404.

## Verificación antes de activar el scheduler

1. Confirmar que la imagen corre como usuario no root y que los binarios responden (`check-agents`).
2. Confirmar permisos de `data/` y `auth/`.
3. Ejecutar `validate-config` y revisar el crontab con `generate-crontab`.
4. Ejecutar una edición manual por newsletter con `run <slug>` y revisar el JSON y el XML generado.
5. Recrear el contenedor y comprobar que el historial y las sesiones persisten.
6. Activar el scheduler solo después de los pasos anteriores.
