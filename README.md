<div align="center">

# Newsletter Bot

**Automated editorial publishing through static RSS feeds**

[![CI](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml/badge.svg)](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![RSS](https://img.shields.io/badge/RSS-2.0-FFA500?logo=rss&logoColor=white)](https://www.rssboard.org/rss-specification)
[![Docker](https://img.shields.io/badge/Docker-GHCR-2496ED?logo=docker&logoColor=white)](https://github.com/ncorrea-13/newsletter/pkgs/container/newsletter-bot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Español](README.es.md)

</div>

---

Each newsletter has a JSON configuration and a Markdown prompt. Supercronic runs the schedules from those files. Codex CLI or Claude Code researches and writes each edition; Go validates it and regenerates the RSS feeds.

The project has no write API, frontend, database, or mail delivery. A read-only HTTP service serves the XML files.

## Flow

```text
Supercronic
  → newsletter run <slug>
  → prompt + recent history
  → Codex CLI or Claude Code
  → validation in Go
  → persistent JSON edition
  → static RSS 2.0 feeds
  → read-only HTTP service
```

Each newsletter has an RSS feed; `all.xml` combines their editions. If feed generation fails after saving an edition, regenerate the XML from history.

## Stack

| Layer | Technology |
| --- | --- |
| Application | Go 1.27, no external dependencies |
| Agents | Codex CLI and Claude Code, pinned in the image |
| Scheduler | Supercronic, with crontab generated from JSON |
| Image | Debian Trixie slim, multi-stage build, published to GHCR |
| Storage | JSON and XML files in bind mounts |
| Publishing | Read-only HTTP |
| CI / CD | GitHub Actions: tests on push, multi-arch images on `main` |

## Quick start

Run tests:

```sh
go test -race ./cmd/... ./internal/...
```

If you do not already have local configuration, copy the examples:

```sh
cp -r examples/config config
cp -r examples/prompts prompts
```

Do not run these commands over existing directories: they can overwrite files. The sample newsletter is disabled.

Download the published images:

```sh
export NEWSLETTER_DIR="$PWD"
export FEED_PORT=8080
docker compose -f compose.prod.yaml pull
```

Follow [Deployment](docs/DEPLOYMENT.md) to set permissions, authenticate agents, and start the services. The guide also covers Podman.

## Configuration

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Each slug has a JSON file and a prompt with the same name. Restart the container to apply changes.

## Documentation

- [Architecture](docs/ARCHITECTURE.md): components, configuration, output contract, feeds, and trust boundaries.
- [Deployment](docs/DEPLOYMENT.md): images, Compose, agent authentication, and feed exposure.
- [Operations](docs/OPERATIONS.md): configuration changes, manual runs, imports, troubleshooting, and backups.
- [Models](docs/MODELS.md): Codex and Claude Code model catalog.
- [Agent instructions](AGENTS.md): rules for working on this repository.

## License

Distributed under the MIT license. See [LICENSE](LICENSE).
