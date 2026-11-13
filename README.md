# Newsletter Bot

Publicación editorial automatizada a través de feeds RSS estáticos.

Cada newsletter tiene un JSON de configuración y un prompt Markdown separado. Supercronic ejecuta los horarios generados desde los JSON. Codex CLI o Claude Code investigan y redactan. Go valida cada edición y regenera los feeds RSS.

No hay API de escritura, frontend, base de datos ni envío de correo. Un servicio HTTP de solo lectura sirve los XML. Cómo se expone ese servicio se decide en el despliegue, no en este repo.

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

Cada edición es un ítem RSS. Hay un feed por newsletter y un `all.xml` combinado. Un fallo nunca reemplaza una edición o un feed válido. El agente trabaja sin herramientas de escritura: solo Go publica archivos validados.

## Configuración

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

`config/` y `prompts/` contienen configuraciones y prompts reales, así que no están en Git. El repo incluye ejemplos genéricos en `examples/`:

```sh
cp -r examples/config config
cp -r examples/prompts prompts
```

## Inicio rápido

Pruebas:

```sh
go test -race ./cmd/... ./internal/...
```

Construir la imagen localmente:

```sh
podman compose -f compose.yaml -f compose.podman.yaml build newsletter-bot
```

Para desplegar la imagen publicada y habilitar el servicio de feeds, ver [DEPLOYMENT.md](docs/DEPLOYMENT.md).

## Stack

| Capa | Tecnología |
| --- | --- |
| Aplicación | Go |
| Agentes | Codex CLI y Claude Code, con versiones fijadas en la imagen |
| Scheduler | Supercronic, con crontab generado desde los JSON |
| Imagen | Debian Trixie slim, compilación multietapa; publicada en GHCR |
| Persistencia | Archivos JSON y XML en bind mounts |
| Publicación | HTTP de solo lectura |

## Documentación

- [Arquitectura](docs/ARCHITECTURE.md): componentes, configuración, contrato de salida, feeds y límites de confianza.
- [Despliegue](docs/DEPLOYMENT.md): imagen, compose, autenticación de agentes y exposición de feeds.
- [Operación](docs/OPERATIONS.md): cambios de configuración, ejecuciones manuales, importación, diagnóstico y respaldos.
- [Modelos](docs/MODELS.md): catálogo de modelos de Codex y Claude Code.
- [Instrucciones para agentes](AGENTS.md): reglas de trabajo sobre este repo.
