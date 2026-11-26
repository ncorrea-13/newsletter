# Arquitectura

[English](ARCHITECTURE.md)

## Objetivo

Ejecutar newsletters con agentes de IA, guardar las ediciones en JSON y publicar RSS 2.0 estático. El servicio HTTP de feeds no ofrece API de escritura.

## Componentes

```text
config/newsletters/*.json + prompts/<slug>.md
  └── validación + generación de crontab
      └── Supercronic
          └── newsletter run <slug>
              ├── config + prompt editorial
              ├── historial reciente y lock por newsletter
              ├── adaptador Codex o Claude
              ├── validación y sanitización del contrato
              ├── historial por fecha editorial
              └── reemplazo atómico de cada feed RSS
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

Go genera el crontab desde los JSON al iniciar el contenedor. El proyecto no usa base de datos, servicio de correo, frontend ni API de escritura.

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
  "reasoning_effort": "high",
  "schedule": {
    "days": ["monday", "wednesday", "friday"],
    "time": "06:00"
  },
  "timeout_minutes": 45
}
```

`config/defaults.json` define la zona horaria y los valores comunes: agente, timeout, contexto histórico, retención de feeds y `feed_base_url`.

Definí `reasoning_effort` por newsletter para pasarlo al CLI elegido. Si lo omitís, el proveedor usa su valor predeterminado.

- `history_context_editions`: ediciones recientes que recibe el agente. Default 5, máximo 30.
- `feed_retention_editions`: ítems por feed. Default 30, máximo 200.
- `feed_base_url`: base pública de los feeds. Si un newsletter no define `site_url`, su canal apunta a `<base>/<slug>.xml`.

Cada slug tiene un JSON y un prompt con el mismo nombre. Go valida los nombres, la ubicación del prompt y la configuración antes de arrancar Supercronic. Reiniciá el contenedor para aplicar los cambios; no hay recarga en caliente.

## Ejecución de una edición

1. Validar slug y configuración.
2. Tomar el lock del newsletter. Una segunda ejecución del mismo slug falla.
3. Calcular la fecha editorial en la zona horaria configurada.
4. Leer el prompt y el contexto compacto de ediciones recientes.
5. Ejecutar el proveedor como proceso separado, con timeout.
6. Validar el contrato de salida (ver abajo).
7. Guardar la edición con archivo temporal y enlace atómico, sin sobrescribir una fecha existente.
8. Regenerar los feeds configurados y `all.xml`, y reemplazar cada archivo por separado.
9. Registrar el resultado en logs y liberar el lock.

Para corregir una edición existente, hacé un respaldo y seguí [Operación](OPERATIONS.es.md#corrección-de-una-edición).

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
- `stories` alimenta el contexto editorial reciente. El agente evita repetir noticias según el prompt; una URL puede respaldar varias noticias o reutilizarse entre ediciones.
- `content_html` pasa por una lista blanca de etiquetas (`p`, `h1` a `h3`, `ul`, `li`, `a`, `strong`, `em`, `br`, `img`, `table`, `thead`, `tbody`, `tr`, `th`, `td`) y atributos (`href`, `src`, `alt`, `width`). Cualquier otra etiqueta, un esquema distinto de `http(s)`, un `<` suelto o un comentario rechaza la edición.
- Las ediciones guardadas antes de `content_html` conservan su resumen plano en el feed.

## Persistencia

```text
data/history/<slug>/YYYY-MM-DD.json   ediciones completas (fuente de verdad)
data/state/locks/                     locks de flock
data/public/<slug>.xml                feed por newsletter
data/public/all.xml                   feed combinado
```

La aplicación conserva el historial. Ejecutá `newsletter generate-feeds` para reconstruir los feeds sin llamar a un agente.

## Feeds RSS

- RSS 2.0. Cada edición es un ítem en orden descendente. El GUID usa `<slug>:<fecha>` en el feed individual y `all:<fecha>` en `all.xml`. En el feed combinado, dos ediciones de la misma fecha comparten GUID.
- `<description>` contiene el HTML sanitizado de la edición, escapado como texto XML. El lector lo renderiza.
- El `<link>` del canal sale de `site_url`. Si falta, sale de `feed_base_url` más `/<slug>.xml`. Si tampoco hay base, sale del enlace del ítem más reciente.
- Go reemplaza cada XML de forma atómica, no el conjunto completo. Si falla la regeneración, algunos feeds pueden haberse actualizado y otros no. La edición guardada permanece en el historial; ejecutá `newsletter generate-feeds` para reconstruirlos.
- Al regenerar, se borran los XML de slugs que ya no están configurados.

## Publicación HTTP

`newsletter serve` acepta solo `GET` y `HEAD` sobre archivos `.xml` del directorio público. No lista directorios ni sirve otros archivos. `newsletter-feeds` monta `data/public/` en solo lectura. Configurá la interfaz y el puerto según [Despliegue](DEPLOYMENT.es.md).

## Importación de historial

`newsletter import-history <directorio>` convierte un export de correos de ediciones anteriores en ediciones del historial. Aplica el mismo contrato: el HTML pasa por la lista blanca, cada edición necesita al menos una historia con enlace y nunca sobrescribe una fecha existente. `--dry-run` muestra el plan sin escribir nada.

## Concurrencia y fallos

- Los newsletters distintos pueden correr en paralelo. Cada slug tiene su propio lock.
- La regeneración de feeds usa un lock global, así que dos escrituras de RSS no se pisan.
- Los locks usan `flock` del kernel. Se liberan al terminar el proceso, incluso tras una caída.
- La aplicación no reintenta la ejecución del agente.
- Al reiniciar, el scheduler no ejecuta en lote las tareas perdidas.

## Confianza

El resultado del agente y las páginas que consulta son datos no confiables. Go valida el contrato antes de guardar nada.

Codex corre con `--sandbox read-only` y `--disable shell_tool`. Claude Code recibe `--safe-mode`, `--restricted`, modo `plan` y solo las herramientas `WebSearch` y `WebFetch`. Los procesos de los CLI comparten el contenedor y los montajes del bot, incluido `auth/`; estas opciones no reemplazan la verificación de los límites de cada proveedor. Go valida y publica la salida.
