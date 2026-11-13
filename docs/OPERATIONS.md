# Operación y recuperación

Este documento es el runbook del despliegue. Para la arquitectura, ver [ARCHITECTURE.md](ARCHITECTURE.md). Para instalar, ver [DEPLOYMENT.md](DEPLOYMENT.md).

## Cambios de configuración

Cada newsletter tiene `config/newsletters/<slug>.json` y `prompts/<slug>.md`, con el mismo slug.

- **Frecuencia:** editar `schedule.days` o `schedule.time` en el JSON.
- **Comportamiento editorial:** editar el Markdown del mismo slug.
- **Nuevo newsletter:** crear ambos archivos con el mismo slug.
- **Desactivar:** `"enabled": false`.

Después de cualquier cambio, reiniciar el contenedor. No editar un crontab paralelo ni poner horarios dentro de los prompts.

Antes de reiniciar, validá la configuración:

```sh
podman-compose -f compose.prod.yaml run --rm newsletter-bot validate-config
podman-compose -f compose.prod.yaml run --rm newsletter-bot generate-crontab
```

Una configuración inválida impide arrancar el scheduler.

## Estado

```sh
podman-compose -f compose.prod.yaml run --rm newsletter-bot status
```

Muestra por newsletter si está habilitado, el agente, el horario y la fecha de la última edición. No imprime títulos, resúmenes, prompts ni credenciales.

Para ver el scheduler y sus logs:

```sh
podman ps --format '{{.Names}} {{.Status}}' | grep newsletter
podman logs -f <contenedor-del-scheduler>
```

Cada `run` emite eventos JSON de inicio y fin con newsletter, agente, modelo solicitado, resultado y duración. Si `model` no está configurado, el log registra `provider-default`, y eso no confirma qué versión resuelve el proveedor detrás de un alias.

## Ejecución manual

```sh
podman-compose -f compose.prod.yaml run --rm newsletter-bot run <slug>
```

Usá `--rm` para que el contenedor no quede reiniciándose. Consume uso del proveedor. Si ya existe la edición de la fecha, el comando la rechaza antes de llamar al agente.

Los newsletters distintos pueden correr en paralelo, pero no dos veces el mismo slug. Si una corrida programada del mismo slug sigue activa, esperá a que termine.

## Importación de historial

Para importar un export de correos de ediciones anteriores:

```sh
podman-compose -f compose.prod.yaml run --rm -T \
  -v "$PWD/import:/import:ro" newsletter-bot \
  import-history /import/<directorio> --dry-run
```

Revisá el plan. Después, sin `--dry-run`. Una fecha que ya existe no se sobrescribe. Los feeds se regeneran al final de la importación.

## Feeds

`generate-feeds` reconstruye todos los XML desde el historial, sin llamar a agentes ni modificar JSON:

```sh
podman-compose -f compose.prod.yaml run --rm newsletter-bot generate-feeds
```

El servicio de feeds lee `data/public/` en solo lectura. Si cambió `feed_base_url` o `site_url`, reconstruí los feeds para que el `<link>` del canal se actualice.

## Diagnóstico

| Síntoma | Revisar |
| --- | --- |
| No corre tarea | Estado del scheduler, crontab, zona horaria y logs del contenedor. |
| Agente ausente | Binario, arquitectura y PATH de la imagen. |
| Error de autenticación | Sesión en `auth/`, HOME y permisos. Reautenticar con los comandos de [DEPLOYMENT.md](DEPLOYMENT.md). |
| Cuota o límite | Error del proveedor y plan asociado. No reintentar en bucle. |
| Timeout | Duración, disponibilidad de fuentes y timeout configurado. No subir el timeout sin evidencia. |
| `invalid content_html` | El agente devolvió etiquetas o atributos fuera de la lista blanca, o un `<` suelto. La edición no se guarda. Revisar el prompt y el resultado. |
| `story N has invalid required fields or URL` | Una historia sin título, fuente, URL absoluta o resumen. Revisar el prompt; no relajar la validación. |
| `edition already exists` | La fecha ya tiene edición. Es el comportamiento esperado. |
| Feed sin ítems | Confirmar que `data/public/` tiene XML y que el volumen del servicio de feeds apunta a `data/public`. |
| Feed vacío o 404 | Reconstruir con `generate-feeds` y revisar el mount del servicio. |
| Lock retenido | El lock se libera cuando termina el proceso. Verificar que no haya otra ejecución del mismo slug antes de intervenir. No borrar archivos de lock en uso. |

## Corrección de una edición

Una edición con la misma fecha no se reemplaza en una ejecución normal. Para corregirla:

1. Copiar `data/history/<slug>/<fecha>.json` a un lugar fuera de `data/`.
2. Editar el JSON o borrarlo, según el caso, con `podman unshare` si el ownership lo requiere.
3. Ejecutar `generate-feeds`.
4. Validar el XML resultante.

Una ejecución fallida no borra ni sobrescribe la edición ni el XML válidos anteriores.

## Respaldos

- **Diario:** `data/history/` y `data/state/`.
- **Credenciales:** `auth/`, solo en almacenamiento cifrado y con acceso restringido. Alternativa: reautenticar al restaurar.
- **Regenerable:** `data/public/`, que sale del historial con `generate-feeds`.

Probá la restauración antes de considerar válido un respaldo: recuperá los datos, verificá permisos del usuario del contenedor, ejecutá `generate-feeds`, validá los XML y comprobá la respuesta del servicio de feeds.

## Seguridad operativa

- El servicio de feeds sirve solo XML de `data/public/`.
- JSON, estado y credenciales son privados.
- El scheduler no publica puertos. El servicio de feeds solo se levanta con el perfil `feeds`.
- Sin modo privilegiado ni socket de Docker.
- Las páginas externas son datos no confiables.
- Cualquier cambio de exposición pública (dominio, DNS, túnel o puertos) requiere aprobación explícita.
