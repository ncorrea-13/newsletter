# Agent models

[Español](MODELS.es.md)

This reference covers Codex CLI 0.167.0 and Claude Code 2.1.319 and was reviewed on November 13, 2026. Check your account's access before choosing a model.

In `config/newsletters/<slug>.json`, `agent` selects the CLI and `model` takes the exact value accepted by its `--model` flag. If `model` is omitted, the provider uses its default. Optional `reasoning_effort` accepts `low`, `medium`, `high`, `xhigh`, or `max`; otherwise the CLI default applies.

```json
{
  "agent": "codex",
  "model": "gpt-6-luna",
  "reasoning_effort": "high"
}
```

Codex receives `model_reasoning_effort`; Claude Code receives `--effort`. Available levels may depend on the model. Start without an explicit value and use `high` for newsletters that justify the cost and latency.

## Codex CLI 0.167.0

Values from `codex debug models` in the installed image, checked against the [official Codex catalog](https://developers.openai.com/codex/models).

| Name | `model` value | Suggested use |
| --- | --- | --- |
| GPT-6 Luna | `gpt-6-luna` | Summaries, extraction, and repeatable tasks. Recommended starting point for newsletters. |
| GPT-5.6 Luna | `gpt-5.6-luna` | Compact alternative in the installed catalog. |
| GPT-6 Sol | `gpt-6-sol` | General use and sustained technical work. |
| GPT-6.1 Sol | `gpt-6.1-sol` | More demanding technical work. |
| GPT-5.6 Sol | `gpt-5.6-sol` | Earlier general-purpose alternative. |
| GPT-6.1 Terra | `gpt-6.1-terra` | Writing and everyday tasks. |
| GPT-5.6 Terra | `gpt-5.6-terra` | Earlier writing alternative. |
| GPT-6 Astra | `gpt-6-astra` | Complex cases; likely unnecessary for summaries. |

The local catalog also lists `gpt-5.5`. The [official documentation](https://developers.openai.com/codex/models#gpt-55-retirement) says Codex retired it for ChatGPT sign-in on October 14, 2026. Do not use it with that authentication. Hidden catalog models are not standard options.

## Claude Code 2.1.319

The CLI accepts aliases and full names. An alias points to the model the provider recommends at the time of the request. Use a full ID to pin a version.

| Name / family | `model` alias | Documented full ID | Availability and use |
| --- | --- | --- | --- |
| Haiku 5.5 | `haiku` | `claude-haiku-5-5` | Fast and efficient; recommended for summaries. |
| Sonnet 5.5 | `sonnet` | `claude-sonnet-5-5` | General-purpose balance. |
| Opus 5.5 | `opus` | `claude-opus-5-5` | Complex tasks. |
| Fable 5.2 | `fable` | `claude-fable-5-2` | Demanding reasoning and long tasks; higher cost. |
| Mythos 5.2 | Full ID | `claude-mythos-5-2` | Limited access, subject to Anthropic approval. |
| Haiku 4.5 | Full ID | `claude-haiku-4-5` | Earlier version, still listed. |
| Sonnet 5 | Full ID | `claude-sonnet-5` | Earlier version, still listed. |
| Sonnet 4.6 | Full ID | `claude-sonnet-4-6` | Earlier version, still listed. |
| Sonnet 4.5 | Full ID | `claude-sonnet-4-5` | Earlier version, still listed. |
| Opus 5 | Full ID | `claude-opus-5` | Earlier version, still listed. |
| Opus 4.8 | Full ID | `claude-opus-4-8` | Earlier version, still listed. |
| Opus 4.7 | Full ID | `claude-opus-4-7` | Earlier version, still listed. |
| Opus 4.6 | Full ID | `claude-opus-4-6` | Earlier version, still listed. |
| Opus 4.5 | Full ID | `claude-opus-4-5` | Earlier version, still listed. |
| Fable 5.1 | Full ID | `claude-fable-5-1` | Earlier version, still listed. |
| Fable 5 | Full ID | `claude-fable-5` | Earlier version, still listed. |
| Mythos 5.1 | Full ID | `claude-mythos-5-1` | Limited access, subject to Anthropic approval. |
| Mythos 5 | Full ID | `claude-mythos-5` | Limited access, subject to Anthropic approval. |

The [Claude Code guide](https://docs.anthropic.com/en/docs/claude-code/model-config) documents aliases. The [model list](https://platform.claude.com/docs/en/models/overview) provides IDs and marks limited or retired models. Claude.ai or Team sign-in may resolve aliases differently from the Anthropic API. Test a real run before starting the scheduler.

## Starting point

For news summaries, start with `gpt-6-luna` or `haiku`. Select Codex or Claude per newsletter using `agent`. Run both on the same newsletter only if you want to compare their results.
