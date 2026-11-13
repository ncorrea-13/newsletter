# Modelos de agentes

Referencia revisada el 13 de noviembre de 2026 para Codex CLI 0.167.0 y Claude Code 2.1.319. El acceso a cada modelo depende de la cuenta. Que un modelo aparezca acá no garantiza que la cuenta lo use.

En cada `config/newsletters/<slug>.json`, `agent` elige el CLI y `model` recibe el valor exacto que ese CLI acepta en `--model`. Si omitís `model`, el proveedor usa su default.

```json
{
  "agent": "codex",
  "model": "gpt-6-luna"
}
```

## Codex CLI 0.167.0

Valores de `codex debug models` en la imagen instalada, contrastados con el [catálogo oficial de Codex](https://developers.openai.com/codex/models).

| Nombre | Valor para `model` | Uso orientativo |
| --- | --- | --- |
| GPT-6 Luna | `gpt-6-luna` | Resúmenes, extracción y tareas repetibles. Opción recomendada para newsletters. |
| GPT-5.6 Luna | `gpt-5.6-luna` | Alternativa compacta del catálogo instalado. |
| GPT-6 Sol | `gpt-6-sol` | Uso general y trabajo técnico sostenido. |
| GPT-6.1 Sol | `gpt-6.1-sol` | Trabajo técnico más exigente. |
| GPT-5.6 Sol | `gpt-5.6-sol` | Alternativa anterior de propósito general. |
| GPT-6.1 Terra | `gpt-6.1-terra` | Redacción y tareas cotidianas. |
| GPT-5.6 Terra | `gpt-5.6-terra` | Alternativa anterior para redacción. |
| GPT-6 Astra | `gpt-6-astra` | Casos complejos. Para resúmenes probablemente sobra. |

El catálogo local también muestra `gpt-5.5`. La [documentación oficial](https://developers.openai.com/codex/models#gpt-55-retirement) indica que Codex lo retiró con inicio de sesión de ChatGPT el 14 de octubre de 2026. No lo uses con esa autenticación. Los modelos ocultos del catálogo no son opciones normales.

## Claude Code 2.1.319

El CLI acepta alias y nombres completos. Un alias apunta al modelo recomendado que el proveedor tenga disponible. Para fijar una versión, usá el ID completo.

| Nombre / familia | Alias para `model` | ID completo documentado | Disponibilidad y uso |
| --- | --- | --- | --- |
| Haiku 5.5 | `haiku` | `claude-haiku-5-5` | Rápido y eficiente. Recomendado para resúmenes. |
| Sonnet 5.5 | `sonnet` | `claude-sonnet-5-5` | Buen balance general. |
| Opus 5.5 | `opus` | `claude-opus-5-5` | Tareas complejas. |
| Fable 5.2 | `fable` | `claude-fable-5-2` | Razonamiento exigente y tareas largas. Costo mayor. |
| Mythos 5.2 | ID completo | `claude-mythos-5-2` | Acceso limitado, sujeto a aprobación de Anthropic. |
| Haiku 4.5 | ID completo | `claude-haiku-4-5` | Versión anterior, aún listada. |
| Sonnet 5 | ID completo | `claude-sonnet-5` | Versión anterior, aún listada. |
| Sonnet 4.6 | ID completo | `claude-sonnet-4-6` | Versión anterior, aún listada. |
| Sonnet 4.5 | ID completo | `claude-sonnet-4-5` | Versión anterior, aún listada. |
| Opus 5 | ID completo | `claude-opus-5` | Versión anterior, aún listada. |
| Opus 4.8 | ID completo | `claude-opus-4-8` | Versión anterior, aún listada. |
| Opus 4.7 | ID completo | `claude-opus-4-7` | Versión anterior, aún listada. |
| Opus 4.6 | ID completo | `claude-opus-4-6` | Versión anterior, aún listada. |
| Opus 4.5 | ID completo | `claude-opus-4-5` | Versión anterior, aún listada. |
| Fable 5.1 | ID completo | `claude-fable-5-1` | Versión anterior, aún listada. |
| Fable 5 | ID completo | `claude-fable-5` | Versión anterior, aún listada. |
| Mythos 5.1 | ID completo | `claude-mythos-5-1` | Acceso limitado, sujeto a aprobación de Anthropic. |
| Mythos 5 | ID completo | `claude-mythos-5` | Acceso limitado, sujeto a aprobación de Anthropic. |

La [guía oficial de Claude Code](https://docs.anthropic.com/en/docs/claude-code/model-config) documenta los alias. La [lista oficial de modelos](https://platform.claude.com/docs/en/models/overview) da los IDs y marca los modelos con acceso limitado o retirados. El login de Claude.ai o Team puede resolver los alias de otra forma que la API de Anthropic. Antes de activar el scheduler, probá con una corrida real.

## Recomendación inicial

Para resúmenes de noticias, empezá con `gpt-6-luna` o con `haiku`. Elegí Codex o Claude por newsletter con `agent`. No hace falta correr los dos sobre el mismo newsletter, salvo que quieras comparar resultados a propósito.
