# Deployment

[Español](DEPLOYMENT.es.md)

Use Docker Compose with `compose.prod.yaml`. For Podman, follow [Podman rootless](#podman-rootless). Configure DNS, TLS, and inbound proxies outside this project.

## Images and services

`compose.prod.yaml` uses `ghcr.io/ncorrea-13/newsletter-bot:latest` and `ghcr.io/ncorrea-13/newsletter-egress-proxy:latest`. The `image` workflow publishes both to GHCR for `linux/amd64` and `linux/arm64`, tagged with `latest` and the commit SHA. For a reproducible deployment, replace `latest` with the same SHA in all three image references.

The bot image includes `newsletter`, Codex CLI, Claude Code, and Supercronic. The container runs as UID/GID `10001`. `egress-proxy` connects the bot's internal network to the internet. `newsletter-feeds` serves XML and uses the `feeds` profile.

```sh
export NEWSLETTER_DIR="$PWD"
export FEED_PORT=8080
docker compose -f compose.prod.yaml pull
```

`NEWSLETTER_DIR` points to the deployment directory. `FEED_PORT` sets the host port. Export both variables for each shell session or set them in `.env`.

## Directories and permissions

```text
config/          defaults.json and newsletters/*.json, read-only
prompts/         one <slug>.md per newsletter, read-only
data/            history, state, and feeds, read-write
auth/            agent sessions, read-write
```

From `NEWSLETTER_DIR`, copy the examples if `config/` and `prompts/` do not exist. Do not copy over existing directories: the commands can overwrite files. The sample is disabled and uses a test prompt.

```sh
cp -r examples/config config
cp -r examples/prompts prompts
mkdir -p data/public auth/codex auth/claude
chmod 0700 auth auth/codex auth/claude
```

With Docker and no user remapping, UID/GID `10001` needs write access to `data/` and `auth/`, and read access to `config/` and `prompts/`. Adjust ownership or ACLs for the host; changing ownership can affect your user's access. With rootless Docker or user remapping, check the UID/GID mapping first.

Keep these four directories under `NEWSLETTER_DIR`. Protect credentials and back up `data/` and `auth/` separately.

## Authentication

Authenticate each provider you intend to use from an interactive terminal:

```sh
docker run --rm -it -v "$NEWSLETTER_DIR/auth:/auth" --entrypoint codex ghcr.io/ncorrea-13/newsletter-bot:latest login --device-auth
docker run --rm -it --network host -v "$NEWSLETTER_DIR/auth:/auth" --entrypoint claude ghcr.io/ncorrea-13/newsletter-bot:latest auth login
```

Claude uses host networking for its authentication callback. On Docker Desktop, check host-network compatibility. A session may stop working if you copy it to another host. If you pinned a different image tag in Compose, use the same tag to authenticate.

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot check-agents
```

`check-agents` checks both providers and fails if either is unavailable or unauthenticated. Run an edition to test generation and web search.

## Validation and startup

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot validate-config
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-crontab
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-feeds
docker compose -f compose.prod.yaml run --rm newsletter-bot run <slug>
```

Review the configuration, schedule, edition, and XML before starting the scheduler. A real run consumes provider usage and does not replace an edition for the same date. Test research and sources with the chosen provider inside the final image.

```sh
docker compose -f compose.prod.yaml up -d newsletter-bot
docker compose -f compose.prod.yaml --profile feeds up -d newsletter-feeds
```

The first command also starts `egress-proxy`. The scheduler validates configuration and generates its crontab at startup. Restart the container after changing JSON or prompts.

## Network and feeds

The bot uses an internal network and configures Squid as its HTTP and HTTPS proxy. Squid accepts requests from the `agent` network, restricts ports to 80 and 443, and denies private destinations listed in `docker/squid/squid.conf`. Verify network behavior and each provider in the target environment.

`newsletter-feeds` serves only `GET` and `HEAD` requests for XML files in `data/public/`. `FEED_PORT` sets the host port; `FEED_BIND_ADDRESS` sets the interface and defaults to `127.0.0.1`. For access from another host, choose a private IP and restrict access with a firewall. Public exposure, DNS, tunnels, or port changes require explicit approval.

## Podman rootless

Use the same production file with the Podman override:

```sh
podman compose -f compose.prod.yaml -f compose.podman.yaml pull
podman compose -f compose.prod.yaml -f compose.podman.yaml run --rm newsletter-bot validate-config
```

The override sets `slirp4netns` for the feed service. Create the directories as described above and adjust their owners inside the namespace after reviewing the effect on existing files:

```sh
podman unshare chown -R 10001:10001 data auth
```

This ownership change can prevent your host user from writing to those directories; use `podman unshare` when needed. Use `podman run` for authentication; add `--network slirp4netns` for Codex and keep `--network host` for Claude. The container UID needs write access to the mounted directories.

```sh
podman compose -f compose.prod.yaml -f compose.podman.yaml up -d newsletter-bot
podman compose -f compose.prod.yaml -f compose.podman.yaml --profile feeds up -d newsletter-feeds
```

Before startup, validate configuration, permissions, authentication, and a real edition using the same Compose files. Check that history and sessions persist after recreating the container.
