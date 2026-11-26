<div align="center">

# Newsletter Bot

**Publicación editorial automatizada a través de feeds RSS estáticos**

[![CI](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml/badge.svg)](https://github.com/ncorrea-13/newsletter/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![RSS](https://img.shields.io/badge/RSS-2.0-FFA500?logo=rss&logoColor=white)](https://www.rssboard.org/rss-specification)
[![Docker](https://img.shields.io/badge/Docker-GHCR-2496ED?logo=docker&logoColor=white)](https://github.com/ncorrea-13/newsletter/pkgs/container/newsletter-bot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md)

</div>

---

Cada newsletter tiene un JSON de configuración y un prompt Markdown. Supercronic ejecuta los horarios definidos en esos archivos. Codex CLI o Claude Code investigan y redactan cada edición; Go la valida y regenera los feeds RSS.

El proyecto no tiene API de escritura, frontend, base de datos ni envío de correo. Un servicio HTTP de solo lectura sirve los XML.

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

Cada newsletter tiene un feed RSS; `all.xml` combina sus ediciones. Si falla la regeneración después de guardar una edición, reconstruí los XML desde el historial.

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

Si todavía no tenés configuración local, copiá los ejemplos:

```sh
cp -r examples/config config
cp -r examples/prompts prompts
```

No ejecutes esos comandos sobre directorios existentes: pueden sobrescribir archivos. El newsletter de ejemplo está desactivado.

Descargá las imágenes publicadas:

```sh
export NEWSLETTER_DIR="$PWD"
export FEED_PORT=8080
docker compose -f compose.prod.yaml pull
```

Para configurar permisos, autenticar agentes y arrancar los servicios, seguí [Despliegue](docs/DEPLOYMENT.es.md). La guía incluye instrucciones para Podman.

## Configuración

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Cada slug tiene un JSON y un prompt con el mismo nombre. Reiniciá el contenedor para aplicar los cambios.

## Documentación

- [Arquitectura](docs/ARCHITECTURE.es.md): componentes, configuración, contrato de salida, feeds y límites de confianza.
- [Despliegue](docs/DEPLOYMENT.es.md): imagen, compose, autenticación de agentes y exposición de feeds.
- [Operación](docs/OPERATIONS.es.md): cambios de configuración, ejecuciones manuales, importación, diagnóstico y respaldos.
- [Modelos](docs/MODELS.es.md): catálogo de modelos de Codex y Claude Code.
- [Instrucciones para agentes](AGENTS.md): reglas de trabajo sobre este repo.

## Licencia

Distribuido bajo la licencia MIT. Ver [LICENSE](LICENSE).
