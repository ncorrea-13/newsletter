# Operación y recuperación

Este documento describe operación prevista. Destino de referencia: Podman rootless, con Docker Compose compatible. Caddy permanece externo y hace proxy al servicio de feeds. Iniciarlo con perfil `feeds` cuando se requiera publicar XML.

## Configuración de newsletters

Cada newsletter se configura en `config/newsletters/<slug>.json` y tiene un prompt en `prompts/<slug>.md`; ambos comparten slug. `config/defaults.json` contiene zona horaria y defaults compartidos. Los directorios se montan editables en el host y read-only dentro del contenedor.

Para cambiar una frecuencia, editar `schedule.days` o `schedule.time` en el JSON del newsletter. Para cambiar comportamiento editorial, editar el Markdown del mismo slug. Para agregar un newsletter, crear ambos archivos con el mismo slug. Reiniciar el contenedor para validar cambios y regenerar el crontab. No editar un crontab paralelo ni incluir horarios en prompts.

La validación debe comprobar JSON válido, slug único, agente permitido, hora/días válidos y correspondencia con un prompt existente. Si falla, corregir configuración antes de arrancar scheduler.

## Scheduler

La aplicación convierte configuraciones locales en expresiones cron al iniciar. Configurar y verificar zona horaria local antes de habilitar tareas. Registrar timestamp UTC y fecha editorial local.

Al reiniciar, no ejecutar automáticamente todos los trabajos perdidos. Recuperación manual mediante comando `newsletter run <slug>` tras revisar fecha, estado y edición existente.

## Estado y logs

El proceso principal previsto es Supercronic; logs estructurados van a stdout/stderr. Docker o el runtime elegido administra retención de logs.

Registrar inicio/fin, slug, agente, fecha editorial, duración, resultado, número de historias/duplicados y errores resumidos. No guardar secretos ni razonamiento interno.

Cada ejecución puede persistir JSON resumido en `/data/state/runs/`: ID, newsletter, programación, inicio/fin, estado, proveedor, código de error y mensaje resumido.

## Diagnóstico

| Síntoma | Revisar |
| --- | --- |
| No corre tarea | Estado del scheduler, crontab, timezone y logs del contenedor. |
| Agente ausente | Binario, arquitectura y PATH de imagen final. |
| Error de autenticación | Estado local de CLI, mount persistente, HOME y permisos; reautenticar interactivamente. |
| Cuota o límite | Error del proveedor y plan asociado; no reintentar indefinidamente. |
| Timeout | Duración, disponibilidad de fuentes y timeout configurado; no aumentar sin revisar evidencia. |
| JSON inválido | Guardar error resumido, conservar historial y RSS previos; revisar salida estructurada del adaptador. |
| URL o noticia inválida | Revisar evidencia y prompt; no degradar validación para forzar publicación. |
| Feed viejo | Comprobar generación desde historial y permisos de `/data/public`; reconstruir con `newsletter generate-feeds`. |
| Caddy no sirve XML | Confirmar servicio de feeds, puerto privado, hostname y configuración de proxy. |
| Lock retenido | Verificar proceso propietario y mecanismo de lock antes de limpiar; no borrar lock activo. |

## Corrección y reconstrucción

`newsletter generate-feeds` reconstruye los siete feeds desde el historial, sin invocar agentes ni alterar JSON.

Una edición con la misma fecha no se reemplaza en una ejecución normal. Para corregir: conservar copia, ejecutar operación explícita de reemplazo prevista por la CLI, validar JSON, regenerar feeds y comprobar GUID estable. La interfaz exacta de reemplazo queda por definir durante implementación.

Una ejecución fallida no debe retirar ni sobrescribir la edición o RSS válidos anteriores.

## Backups y restauración

Respaldar diariamente `history/` y `state/`. Respaldar `/srv/newsletter-bot/auth/` solo con almacenamiento cifrado y acceso restringido; alternativa: documentar reautenticación al restaurar. `public/` es regenerable desde historial.

Mantener retención configurable y copia externa opcional. Probar restauración: recuperar datos, revisar permisos del usuario de contenedor, ejecutar reconstrucción de feeds, validar XML y comprobar publicación desde Caddy.

No declarar backup válido sin prueba de restauración. Las credenciales no van a repositorios ni backups públicos.

## Seguridad operativa

- Servir solamente XML de `public/`.
- Mantener JSON, estado y credenciales privados.
- Scheduler sin puerto propio; servicio HTTP solo lectura para XML, habilitado con perfil `feeds`.
- Sin modo privilegiado o socket Docker.
- Restringir permisos de binds y ejecución de agente.
- Mantener timeout, validación de URLs y sanitización HTML.
- Las páginas externas son datos no confiables.
- Caddy y DNS requieren aprobación para cualquier cambio de exposición.
