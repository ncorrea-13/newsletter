# Modelos de agentes

Referencia revisada el 13 de noviembre de 2026 para Codex CLI 0.167.0 y Claude Code 2.1.319. Los catálogos y permisos dependen de la cuenta; que un modelo figure acá no garantiza acceso.

En cada `config/newsletters/<slug>.json`, `agent` selecciona el CLI y `model` contiene exactamente el valor que ese CLI recibe en `--model`. Si se omite `model`, el proveedor elige su default.

```json
{
  "agent": "codex",
  "model": "gpt-6-luna"
}
```

## Codex CLI 0.167.0

Valores visibles en `codex debug models` de la imagen instalada, contrastados con el [catálogo oficial de modelos Codex](https://developers.openai.com/codex/models).

| Nombre | Valor para `model` | Uso orientativo |
| --- | --- | --- |
| GPT-6 Luna | `gpt-6-luna` | Resúmenes, extracción y tareas repetibles; opción recomendada para newsletters. |
| GPT-5.6 Luna | `gpt-5.6-luna` | Alternativa compacta que aún aparece en el catálogo instalado. |
| GPT-6 Sol | `gpt-6-sol` | Uso general y trabajo técnico sostenido. |
| GPT-6.1 Sol | `gpt-6.1-sol` | Trabajo técnico más exigente. |
| GPT-5.6 Sol | `gpt-5.6-sol` | Alternativa anterior de propósito general. |
| GPT-6.1 Terra | `gpt-6.1-terra` | Redacción y tareas cotidianas. |
| GPT-5.6 Terra | `gpt-5.6-terra` | Alternativa anterior para redacción. |
| GPT-6 Astra | `gpt-6-astra` | Casos más complejos; probablemente innecesario para resúmenes. |

El catálogo local también muestra `gpt-5.5`, pero la [documentación oficial](https://developers.openai.com/codex/models#gpt-55-retirement) indica que se retiró de Codex con inicio de sesión ChatGPT el 14 de octubre de 2026. No usarlo con esta autenticación. Los modelos ocultos del catálogo tampoco son opciones normales.

## Claude Code 2.1.319

La CLI acepta alias y nombres completos. Los alias siguen el modelo recomendado disponible para el proveedor; elegir el ID completo para fijar una versión.

| Nombre / familia | Alias para `model` | ID completo documentado | Disponibilidad / uso |
| --- | --- | --- | --- |
| Haiku 5.5 | `haiku` | `claude-haiku-5-5` | Rápido y eficiente; recomendado para resúmenes. |
| Sonnet 5.5 | `sonnet` | `claude-sonnet-5-5` | Buen balance general. |
| Opus 5.5 | `opus` | `claude-opus-5-5` | Tareas complejas. |
| Fable 5.2 | `fable` | `claude-fable-5-2` | Razonamiento exigente y tareas largas; mayor costo. |
| Mythos 5.2 | ID completo | `claude-mythos-5-2` | Acceso limitado; sujeto a aprobación de Anthropic. |
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
| Mythos 5.1 | ID completo | `claude-mythos-5-1` | Acceso limitado; sujeto a aprobación de Anthropic. |
| Mythos 5 | ID completo | `claude-mythos-5` | Acceso limitado; sujeto a aprobación de Anthropic. |

La [guía oficial de configuración de modelos de Claude Code](https://docs.anthropic.com/en/docs/claude-code/model-config) documenta los alias. La [lista oficial de modelos](https://platform.claude.com/docs/en/models/overview) da los IDs y señala disponibilidad limitada o retiro. El login de Claude.ai/Team puede resolver alias de forma distinta a Anthropic API; validar acceso con una corrida real antes de habilitar scheduler.

## Recomendación inicial

Para resúmenes de noticias, empezar con `gpt-6-luna` o `haiku`. Asignar Codex o Claude por newsletter usando `agent`; no hace falta ejecutar ambos para el mismo newsletter salvo que se quiera comparar resultados deliberadamente.
