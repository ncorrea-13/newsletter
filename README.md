<div align="center">

# Newsletter Bot

**Publicación editorial automatizada a través de feeds RSS estáticos**

[![CI](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml/badge.svg)](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![RSS](https://img.shields.io/badge/RSS-2.0-FFA500?logo=rss&logoColor=white)](https://www.rssboard.org/rss-specification)
[![Docker](https://img.shields.io/badge/Docker-GHCR-2496ED?logo=docker&logoColor=white)](https://github.com/ncorrea-13/newsletter/pkgs/container/newsletter-bot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

Cada newsletter tiene un JSON de configuración y un prompt Markdown aparte. Supercronic ejecuta los horarios generados desde los JSON. Codex CLI o Claude Code investigan y redactan. Go valida cada edición y regenera los feeds RSS.

El proyecto no tiene API de escritura, frontend, base de datos ni envío de correo. Un servicio HTTP de solo lectura sirve los XML. El despliegue decide cómo se expone ese servicio.

## Flujo

```text
Supercronic
  → newsletter run <slug>
  → prompt + historial reciente
  → Codex CLI o Claude Code
  → validación estricta en Go
  → edición JSON persistente
  → feeds RSS 2.0 estáticos
  → servicio HTTP de solo lectura
```

Cada edición es un ítem RSS. Hay un feed por newsletter y un `all.xml` combinado. Un fallo no reemplaza una edición ni un feed válidos. El agente no escribe archivos: Go valida y publica.

## Stack

| Capa         | Tecnología                                                     |
| ------------ | -------------------------------------------------------------- |
| Aplicación   | Go 1.27, sin dependencias externas                             |
| Agentes      | Codex CLI y Claude Code, con versiones fijadas en la imagen    |
| Scheduler    | Supercronic, con crontab generado desde los JSON               |
| Imagen       | Debian Trixie slim, compilación multietapa, publicada en GHCR  |
| Persistencia | Archivos JSON y XML en bind mounts                             |
| Publicación  | HTTP de solo lectura                                           |
| CI / CD      | GitHub Actions: tests en cada push, imagen multi-arch en `main` |

## Quick Start

Pruebas:

```sh
go test -race ./cmd/... ./internal/...
```

Configuración local. `config/` y `prompts/` no están en Git, así que partís de los ejemplos:

```sh
cp -r examples/config config
cp -r examples/prompts prompts
```

Construir la imagen:

```sh
podman compose -f compose.yaml -f compose.podman.yaml build newsletter-bot
```

Para desplegar la imagen publicada y habilitar el servicio de feeds, seguí [Despliegue](docs/DEPLOYMENT.md).

## Configuración

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Cada slug tiene un JSON y un prompt con el mismo nombre. Cambiar horarios o prompts no requiere tocar Go. Reiniciar el contenedor aplica los cambios.

## Documentación

- [Arquitectura](docs/ARCHITECTURE.md): componentes, configuración, contrato de salida, feeds y límites de confianza.
- [Despliegue](docs/DEPLOYMENT.md): imagen, compose, autenticación de agentes y exposición de feeds.
- [Operación](docs/OPERATIONS.md): cambios de configuración, ejecuciones manuales, importación, diagnóstico y respaldos.
- [Modelos](docs/MODELS.md): catálogo de modelos de Codex y Claude Code.
- [Instrucciones para agentes](AGENTS.md): reglas de trabajo sobre este repo.

## Licencia

Distribuido bajo la licencia MIT. Ver [LICENSE](LICENSE).
