<div align="center">

# Newsletter Bot

**Automated editorial publishing through static RSS feeds**

[![Go](https://img.shields.io/badge/Go-aplicación-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![RSS](https://img.shields.io/badge/RSS-2.0-FFA500?logo=rss&logoColor=white)](https://www.rssboard.org/rss-specification)

</div>

---

Newsletter Bot is a self-hosted application for scheduled editorial publishing. Each newsletter uses a JSON configuration and a separate Markdown prompt. Supercronic runs schedules generated from JSON; Codex CLI or Claude Code research and draft; Go validates editions and regenerates static RSS feeds.

No hay API HTTP, frontend, base de datos ni envío de correo. Caddy sirve los XML estáticos desde un directorio compartido.

> **Estado:** configuration validation, cron generation, and container bootstrap implemented. Provider adapters and editorial prompts remain local and pending.

## Flujo

```text
Supercronic
  → newsletter run <slug>
  → prompt + historial reciente
  → Codex CLI o Claude Code
  → validación estricta en Go
  → edición JSON persistente
  → feeds RSS 2.0 estáticos
  → Caddy sirve los XML
```

Una edición completa equivale a un ítem RSS. Se genera un feed por configuración y `all.xml`. Los fallos no reemplazan publicaciones válidas. El agente trabaja en un directorio temporal; solo Go publica archivos validados.

## Configuración por archivos

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Each local JSON configures one newsletter, schedule, and prompt path. The application validates configs and generates Supercronic's crontab at startup. See [architecture](docs/ARCHITECTURE.md).

Copy the generic files from `examples/` into local `config/` and `prompts/` directories. Keep real editorial configs and prompts out of public version control.

```sh
cp -r examples/config config
cp -r examples/prompts prompts
```

## Stack previsto

| Capa | Tecnología |
| --- | --- |
| Aplicación | Go |
| Agentes | Codex CLI y Claude Code, distribuciones oficiales |
| Scheduler | Supercronic con crontab generado desde JSON |
| Imagen | Debian Trixie slim, compilación multistage |
| Persistencia | Archivos JSON y XML en bind mounts |
| Publicación | Caddy existente, sin servidor HTTP propio |

## Inicio rápido

Run `go test ./...`. Build with `docker compose build` or `podman compose build`. Caddy remains external; see [deployment](docs/DEPLOYMENT.md).

## Documentación

- [Arquitectura](docs/ARCHITECTURE.md): componentes, configuración por archivos y límites de confianza.
- [Plan de implementación](docs/PLAN.md): alcance, requisitos y fases.
- [Instalación y despliegue](docs/DEPLOYMENT.md): contenedor, Caddy externo y autenticación pendiente.
- [Operación y recuperación](docs/OPERATIONS.md): scheduler, estado, backups y troubleshooting previstos.
- [Editorial migration](docs/EDITORIAL-MIGRATION.md): prompt migration and research validation.

## Infraestructura

Reference runtime: rootless Podman, with Docker Compose compatibility. Caddy remains independent and consumes the published directory through a shared mount or explicit synchronization. No ports or public exposure are configured here.

---
