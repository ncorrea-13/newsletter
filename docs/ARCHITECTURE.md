# Arquitectura

## Objetivo

Ejecutar newsletters con agentes de IA, guardar las ediciones en JSON y publicar RSS 2.0 estático. El proceso no escucha conexiones ni ofrece API de escritura.

## Componentes

```text
config/newsletters/*.json + prompts/<slug>.md
  └── validación + generación de crontab
      └── Supercronic
          └── newsletter run <slug>
              ├── config + prompt editorial
              ├── historial reciente y lock por newsletter
              ├── adaptador Codex o Claude (sin escritura)
              ├── validación y sanitización del contrato
              ├── historial por fecha editorial
              └── regeneración atómica de feeds RSS
```

| Componente | Responsabilidad |
| --- | --- |
| Configuración JSON | Newsletter, prompt, agente, modelo, horario y opciones operativas. |
| Generador de schedule | Valida las configuraciones y produce el crontab para Supercronic. |
| Supercronic | Ejecuta las tareas del crontab generado. |
| CLI Go | Configuración, locks, validación, persistencia, logs y feeds. |
| Codex / Claude | Investigan fuentes y redactan una edición según el prompt. |
| Historial JSON | Fuente de verdad de las ediciones. Los feeds salen de aquí. |
| Servidor de feeds | Sirve XML estáticos desde `data/public/`, en solo lectura. |

El proyecto no usa base de datos, servicio de correo, frontend ni API de escritura. El crontab no es fuente de verdad: Go lo genera desde los JSON al iniciar el contenedor.

## Configuración declarativa

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Cada JSON describe un newsletter y referencia su prompt:

```json
{
  "slug": "ejemplo",
  "enabled": true,
  "prompt": "prompts/ejemplo.md",
  "agent": "codex",
  "model": "gpt-6-luna",
  "schedule": {
    "days": ["monday", "wednesday", "friday"],
    "time": "06:00"
  },
  "timeout_minutes": 45
}
```

`config/defaults.json` define la zona horaria y los valores comunes: agente, timeout, contexto histórico, retención de feeds y `feed_base_url`.

- `history_context_editions`: ediciones recientes que recibe el agente. Default 5, máximo 30.
- `feed_retention_editions`: ítems por feed. Default 30, máximo 200.
- `feed_base_url`: base pública de los feeds. Si un newsletter no define `site_url`, su canal apunta a `<base>/<slug>.xml`.

Cada slug tiene un JSON y un prompt con el mismo nombre. El validador comprueba que el prompt exista, que esté dentro de `prompts/` y que el slug sea único. Si una configuración es inválida, el arranque se detiene antes de Supercronic.

Cambiar un JSON o un prompt no requiere tocar Go. Reiniciar el contenedor aplica los cambios. No hay recarga en caliente.

## Ejecución de una edición

1. Validar slug y configuración.
2. Tomar el lock del newsletter. Una segunda ejecución del mismo slug falla.
3. Calcular la fecha editorial en la zona horaria configurada.
4. Leer el prompt y el contexto compacto de ediciones recientes.
5. Ejecutar el proveedor como proceso separado, con timeout.
6. Validar el contrato de salida (ver abajo).
7. Guardar la edición con archivo temporal y rename atómico.
8. Regenerar el feed del newsletter y `all.xml`, y validarlos antes de reemplazarlos.
9. Registrar el resultado resumido y liberar el lock.

Una fecha editorial existente no se reemplaza de forma automática. Para corregir una edición, hace falta una operación explícita y un respaldo.

## Contrato de salida

El agente devuelve un único objeto JSON, sin texto alrededor:

```json
{
  "title": "Título corto de la edición",
  "content_html": "<h1>Título</h1><p>Cuerpo de la edición.</p>",
  "stories": [
    {
      "title": "Título de la noticia",
      "source": "Nombre del medio",
      "url": "https://example.com/article",
      "published_at": "2026-11-11",
      "summary": "Resumen breve con el dato principal."
    }
  ]
}
```

- `title`, `content_html` y `stories` son obligatorios. El esquema está en `internal/agent/agent.go`.
- `stories` alimenta la deduplicación: ninguna URL puede repetirse dentro de la edición ni respecto del contexto reciente.
- `content_html` pasa por una lista blanca de etiquetas (`p`, `h1` a `h3`, `ul`, `li`, `a`, `strong`, `em`, `br`, `img`) y atributos (`href`, `src`, `alt`, `width`). Cualquier otra etiqueta, un esquema distinto de `http(s)`, un `<` suelto o un comentario rechaza la edición.
- Las ediciones guardadas antes de `content_html` conservan su resumen plano en el feed.

## Persistencia

```text
data/history/<slug>/YYYY-MM-DD.json   ediciones completas (fuente de verdad)
data/state/runs/*.json                resumen de cada ejecución
data/state/locks/                     locks de flock
data/public/<slug>.xml                feed por newsletter
data/public/all.xml                   feed combinado
```

El historial no se purga. Los feeds se derivan del historial: `newsletter generate-feeds` los reconstruye sin llamar a ningún agente.

## Feeds RSS

- RSS 2.0. Cada edición es un ítem con GUID estable `<slug>:<fecha>`, en orden descendente.
- `<description>` contiene el HTML sanitizado de la edición, escapado como texto XML. El lector lo renderiza.
- El `<link>` del canal sale de `site_url`. Si falta, sale de `feed_base_url` más `/<slug>.xml`. Si tampoco hay base, sale del enlace del ítem más reciente.
- El reemplazo es atómico: si falla la generación, queda el XML anterior publicado.
- Al regenerar, se borran los XML de slugs que ya no están configurados.

## Publicación HTTP

`newsletter serve` acepta solo `GET` y `HEAD` sobre archivos `.xml` del directorio público. No lista directorios ni sirve otros archivos. El servicio `newsletter-feeds` usa el mismo binario y monta `data/public` en solo lectura. El despliegue decide qué interfaz y qué puerto se publican (ver [DEPLOYMENT.md](DEPLOYMENT.md)).

## Importación de historial

`newsletter import-history <directorio>` convierte un export de correos de ediciones anteriores en ediciones del historial. Aplica el mismo contrato: el HTML pasa por la lista blanca, cada edición necesita al menos una historia con enlace y nunca sobrescribe una fecha existente. `--dry-run` muestra el plan sin escribir nada.

## Concurrencia y fallos

- Los newsletters distintos pueden correr en paralelo. Cada slug tiene su propio lock.
- La regeneración de feeds usa un lock global, así que dos escrituras de RSS no se pisan.
- Los locks usan `flock` del kernel. Se liberan al terminar el proceso, incluso tras una caída.
- Los reintentos son limitados y solo aplican a errores transitorios. Autenticación inválida, cuota y errores editoriales no se reintentan.
- Al reiniciar, el scheduler no ejecuta en lote las tareas perdidas.

## Confianza

El resultado del agente y las páginas que consulta son datos no confiables. Go valida el contrato antes de guardar nada.

Codex corre sin herramientas de shell (`features.shell_tool=false`) y con sandbox de solo lectura. Claude Code solo recibe `WebSearch` y `WebFetch`. Así el agente no tiene ruta para leer archivos locales, incluido el directorio de credenciales, y el sandbox le impide escribir. El agente no modifica configuración, prompts, historial ni directorio público.
