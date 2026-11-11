# Plan de implementación

## Estado y objetivo

**Estado:** especificación lista para implementar; no equivale a integración verificada.

Migrar newsletters configurados a una aplicación self-hosted. El sistema investiga fuentes, produce contenido editorial, persiste JSON y publica feeds RSS estáticos.

### Alcance

- Go para toda la lógica propia.
- Codex CLI y Claude Code con distribuciones oficiales, probadas dentro de la imagen final.
- Supercronic como scheduler.
- JSON como persistencia; RSS 2.0 estático.
- Caddy existente como servidor de archivos.
- Usuario no root y credenciales persistentes fuera de la imagen.

### Fuera de alcance

API HTTP propia, frontend, panel, base de datos, Redis, correo, AgentMail, n8n y agentes instalados en el host.

## Tareas editoriales

Horarios y timezone pertenecen a la configuración local, no a este documento público. No incluir nombres de newsletters, prompts originales ni identificadores de automatizaciones en este repositorio.

## Configuración declarativa

La configuración se edita sin cambiar código:

```text
config/defaults.json
config/newsletters/<slug>.json
prompts/<slug>.md
```

Un JSON local por newsletter. Contiene `slug`, `enabled`, ruta `prompt`, `agent` y `schedule`; puede incluir overrides operativos como timeout. Mantener configuraciones reales y prompts fuera del repositorio público.

La aplicación descubre los JSON del directorio, valida que slugs sean únicos y que cada configuración tenga exactamente un prompt correspondiente. Agregar newsletter requiere agregar esos dos archivos, no modificar Go. No duplicar horarios en YAML o crontab mantenido a mano.

Al iniciar, validar configuración y generar el crontab usado por Supercronic. Una configuración inválida impide iniciar el scheduler. Cambiar frecuencia o prompt no requiere modificar Go; reiniciar el contenedor aplica los cambios. No hace falta recarga dinámica en primera versión.

Montar `config/` y `prompts/` desde el host y read-only dentro del contenedor, según runtime. Rechazar rutas de prompt fuera de `prompts/`.

## Requisitos funcionales

### CLI

```text
newsletter run <slug>
newsletter generate-feeds
newsletter status
newsletter validate-config
newsletter check-agents
newsletter generate-crontab
```

`run` valida configuración y slug, obtiene lock, calcula fecha editorial, construye contexto, ejecuta agente, valida y persiste edición, regenera RSS y registra resultado. `generate-feeds` no llama agentes ni modifica el historial. `status` no revela secretos. `check-agents` separa chequeos locales de operaciones que consumen uso. `generate-crontab` valida configuración y emite el schedule que consume Supercronic; el entrypoint lo ejecuta en cada arranque.

### Agentes

Interfaz mínima `Agent.Run(context.Context, Request) (Result, error)`. Ejecutar con `exec.CommandContext`; no interpolar prompts en comandos shell. Capturar stdout, stderr, código de salida y duración. Usar salida no interactiva estructurada documentada por la versión instalada.

No desactivar globalmente sandbox/aprobaciones. Limitar herramientas y workspace a lo necesario para investigación. El agente no escribe los archivos finales.

### Investigación

Verificar búsqueda reciente, lectura de páginas, fechas, enlaces y contraste con fuentes. Preservar fuentes especializadas definidas en prompts locales. No inventar enlaces ni fechas. No agregar servicios pagos sin aprobación.

Cada historia lleva título, fuente, URL verificable, fecha disponible y resumen; incluir justificación de relevancia cuando el prompt lo requiera.

### Datos y publicación

- Una edición JSON por slug y fecha editorial.
- Clave lógica slug + fecha; no sobrescribir silenciosamente.
- Escrituras temporales, validación y rename atómico.
- Contexto de deduplicación configurable; URLs normalizadas y duplicados semánticos tratados por el agente.
- Tracking editorial conserva cambios relevantes específicos de cada configuración.
- Generar un RSS por configuración y `all.xml`; un ítem por edición; GUID estable.
- RSS conserva inicialmente 30 ítems configurables; JSON histórico no se elimina.
- Sanitizar HTML; permitir enlaces e imágenes válidas, excluir scripts e iframes.

## Requisitos operativos

- Supercronic: schedule generado desde JSON, zona horaria verificada en runtime.
- Configuración: montar `config/` y `prompts/` editables en host y read-only en contenedor; reiniciar contenedor tras cambios.
- No ejecutar en lote trabajos perdidos tras apagado.
- Lock global para una ejecución de agente a la vez.
- Timeouts y reintentos limitados; no reintentar indefinidamente autenticación o cuota.
- Logs estructurados a stdout/stderr, sin razonamientos internos ni secretos.
- Estado de ejecución JSON resumido bajo `/data/state/runs/`.
- Bind mounts para historial, estado, feeds y autenticación.
- Caddy sirve solo `public/`, sin listar directorios ni exponer datos privados.
- Sin puertos publicados por el servicio bot; sin Docker socket ni modo privilegiado.

## Imagen y distribución

Propuesta inicial: multistage `golang:1-trixie` a `debian:trixie-slim`; binario con `CGO_ENABLED=0` cuando aplique, `-trimpath` y símbolos reducidos. Fijar versiones/digests reales, no inventados.

Instalar paquetes mínimos justificados, Codex CLI oficial, Claude Code oficial y Supercronic oficial; verificar arquitectura, checksums disponibles, comandos de versión y ayuda. Crear usuario no root (propuesta UID/GID `10001`) y comprobar permisos reales de volúmenes.

Podman rootless es el destino de referencia; mantener compatibilidad con Docker Compose. Caddy queda externo. Ver [Instalación y despliegue](DEPLOYMENT.md).

## Estructura prevista

```text
cmd/newsletter/
internal/config/ internal/agent/ internal/runner/
internal/history/ internal/editorial/ internal/rss/ internal/status/
prompts/ config/defaults.json config/newsletters/ schemas/ testdata/ scripts/
Dockerfile compose.yaml entrypoint.sh .dockerignore .gitignore .env.example
go.mod go.sum README.md
```

Evitar paquetes adicionales sin necesidad concreta.

## Pruebas

### Unitarias

Configuración, fechas locales, normalización de URL, deduplicación, validación JSON, serialización, RSS, GUID, orden, sanitización, escritura atómica y locks.

### Integración

Agente falso para éxito, JSON inválido, timeout, fallo de proceso, cancelación, edición existente, reintentos, reconstrucción RSS y fallos de escritura. No consumir suscripciones en CI.

### Imagen y smoke test real

Comprobar Debian Trixie, usuario no root, binarios y versiones, permisos, timezone, Supercronic, ausencia de puertos. Con autenticación real, probar ambos agentes no interactivos, una edición completa, JSON, RSS, Caddy, recreación del contenedor y persistencia de sesiones.

No afirmar que una prueba pasó si no se ejecutó. Los agentes y la investigación real solo se consideran integrados tras pruebas dentro de la imagen final.

## Fases

0. Investigar instalación, auth, salida estructurada, búsqueda, arquitectura host y Supercronic; entregar compatibilidad verificada.
1. Bootstrap de módulo, imagen, configuración declarativa, generación de crontab y README.
2. CLI, logging, locks y agente falso.
3. Adaptadores Codex/Claude y pruebas reales.
4. Esquema, historial, validación y feeds.
5. Migrar una configuración editorial de extremo a extremo.
6. Migrar las configuraciones restantes y sus requisitos específicos.
7. Activar scheduler y verificar zona horaria, concurrencia y reintentos.
8. Configurar Caddy, DNS/exposición según decisión, HTTPS y lector RSS.
9. Hardening, backups, restauración y prueba de reinicio.

## Criterio de finalización

Imagen reproducible; ambas CLI instaladas, autenticadas y no interactivas; sesiones persistentes; prompts originales adaptados; investigación real; JSON histórico y deduplicación; validación editorial específica; feeds por configuración y feed combinado válidos; publicación atómica servida por Caddy; tests y recuperación documentados. Resolver antes el destino y alcance de exposición de los feeds.
