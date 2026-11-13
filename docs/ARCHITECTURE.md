# Arquitectura

## Objetivo

Ejecutar newsletters configurados mediante agentes de IA, conservar ediciones en JSON y publicar RSS 2.0 estático. El proceso no escucha conexiones ni ofrece API.

## Componentes

```text
config/newsletters/*.json + prompts/<slug>.md
  └── validación + generación de crontab
      └── Supercronic
          └── newsletter run <slug>
              ├── config + prompt editorial
              ├── historial reciente y lock por newsletter
              ├── adaptador Codex o Claude
              ├── validación del contrato JSON
              ├── historial por fecha editorial
              └── regeneración atómica de feeds RSS
```

| Componente | Responsabilidad |
| --- | --- |
| Configuración JSON | Declarar newsletter, prompt, agente, frecuencia y opciones operativas. |
| Generador de schedule | Descubrir y validar configuraciones; producir crontab para Supercronic. |
| Supercronic | Ejecutar las tareas declaradas por el crontab generado. |
| CLI Go | Configuración, locks, procesos, validación, persistencia, logs y feeds. |
| Codex / Claude | Investigar fuentes y producir una edición según el prompt del newsletter. |
| Historial JSON | Fuente de verdad para ediciones y reconstrucción de feeds. |
| Caddy | Servir únicamente archivos XML del directorio público. |
| Servidor de feeds | Servir XML estáticos por HTTP desde el directorio público, solo lectura. |

No se agregan base de datos, servicio de correo, frontend ni API de escritura. El servidor HTTP solo sirve archivos XML. El crontab no será fuente de verdad ni se editará por newsletter: se genera desde JSON al iniciar el contenedor.

## Configuración declarativa

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Cada JSON representa un newsletter y referencia su prompt. Ejemplo conceptual:

```json
{
  "slug": "sample",
  "enabled": true,
  "prompt": "prompts/sample.md",
  "agent": "codex",
  "schedule": {
    "days": ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"],
    "time": "06:00"
  },
  "timeout_minutes": 45
}
```

`config/defaults.json` contiene timezone y defaults comunes, por ejemplo agente, timeout, contexto histórico y retención RSS. `history_context_editions` controla cuántas ediciones recientes recibe el agente; su default es 5 y el máximo 30. Cada newsletter declara solo sus diferencias. La frecuencia vive junto a ese newsletter, no dentro del prompt ni en un crontab editado a mano. Cada newsletter elige `agent` (`codex` o `claude`) y, opcionalmente, `model`; si `model` falta, el CLI usa su modelo por defecto.

Correspondencia 1:1 por slug: `config/newsletters/<slug>.json` y `prompts/<slug>.md`. El validador exige que cada prompt exista y corresponda al slug.

Al iniciar, la aplicación descubre `config/newsletters/*.json` y valida slugs únicos, rutas de prompt, días, horarios, agente y valores efectivos. `enabled: false` excluye la tarea. La configuración inválida detiene el arranque antes de Supercronic.

La aplicación genera un crontab temporal y luego ejecuta Supercronic. Cambiar JSON o Markdown no requiere cambiar Go; reiniciar el contenedor carga el prompt y schedule nuevos. No se requiere recarga en caliente.

No hace falta un formateador separado al inicio. Go construye el contexto del agente desde config, prompt e historial; después del agente, Go valida y normaliza el JSON antes de publicar. El generador de crontab solo traduce schedule JSON al formato requerido por Supercronic.

Montar `config/` y `prompts/` desde el host como archivos editables y read-only dentro del contenedor. Resolver rutas de prompts dentro del directorio permitido y rechazar rutas que escapen de él.

## Ejecución de una edición

1. Validar slug y configuración.
2. Adquirir lock por newsletter para impedir duplicados del mismo slug.
3. Calcular timestamp UTC y fecha editorial local.
4. Leer prompt y contexto compacto de ediciones recientes.
5. Ejecutar el proveedor mediante proceso separado y con timeout.
6. Validar estructura, campos, URL, fechas y contenido HTML.
7. Guardar JSON con archivo temporal y rename atómico.
8. Regenerar feed individual y feed combinado; validar XML antes del reemplazo.
9. Registrar resultado resumido y liberar lock.

Una fecha editorial ya existente no se reemplaza de forma implícita. Las correcciones requieren operación explícita y respaldo.

## Persistencia

```text
/data/history/<slug>/YYYY-MM-DD.json
/data/state/runs/*.json
/data/state/locks/
/data/state/events/events.json
/data/public/<slug>.xml
/data/public/all.xml
```

`history/` conserva las ediciones completas. `state/` guarda estado operativo y seguimiento de eventos IT. `public/` contiene solo los feeds servibles. Las credenciales de agente se montan en sus rutas reales verificadas y permanecen privadas.

Los feeds son derivados del historial y pueden reconstruirse con `newsletter generate-feeds`. `feed_retention_editions` limita los ítems por feed; su default es 30 y el máximo 200. El historial no se purga.

## Contrato editorial y confianza

El resultado del agente es entrada no confiable. Go valida JSON estricto, slug, fechas, campos obligatorios y URLs absolutas. El RSS serializa títulos y resúmenes como texto XML escapado; no interpreta HTML.

Las páginas externas y el contenido recuperado son datos, no instrucciones. El agente no modifica configuración, prompts, historial ni directorio público. Su salida pasa por validación antes de persistirse.

Codex se ejecuta sin herramientas de shell (`features.shell_tool=false`) y con sandbox de solo lectura; Claude Code solo recibe `WebSearch` y `WebFetch`. Esa restricción de herramientas elimina la ruta de lectura de archivos locales, incluido el directorio de credenciales, y el sandbox además impide escrituras.

## RSS

- RSS 2.0 con descripciones en texto XML escapado.
- Canal con `title`, `description` y `site_url` del newsletter; si faltan, usa el slug y el enlace del ítem más reciente.
- Un ítem por edición completa.
- GUID estable derivado de newsletter y fecha editorial.
- Orden cronológico descendente.
- `all.xml` combines recent editions from configured categories.
- Reemplazo atómico: un error conserva el XML publicado anteriormente.
- Al regenerar, se eliminan XML de slugs que ya no están configurados.

## Publicación HTTP

`newsletter serve` expone únicamente `GET` y `HEAD` para archivos XML del directorio público. No lista directorios ni sirve otros archivos. El servicio Compose `newsletter-feeds` usa el mismo binario y monta solo `/data/public` en solo lectura. El puerto del host queda en `127.0.0.1:8080` por defecto; Caddy externo hace proxy al puerto configurado.

## Concurrencia y fallos

El scheduler puede ejecutar en paralelo newsletters distintos; cada slug tiene su propio lock y una corrida duplicada del mismo slug falla. La regeneración de feeds usa un lock global bloqueante para serializar escrituras de RSS; `generate-feeds` usa el mismo lock sin espera. Los locks usan `flock` del kernel y se liberan al terminar el proceso, incluso tras una caída.

Los reintentos son limitados y aplican solo a errores transitorios. Autenticación inválida, límites de uso y errores editoriales no se reintentan indefinidamente. Al reiniciar, no se ejecutan en lote tareas perdidas.

## Decisiones operativas pendientes

Podman rootless is the reference runtime; Docker Compose remains supported. Caddy stays outside the bot and proxies to the optional read-only feed service. See [deployment](DEPLOYMENT.md).
