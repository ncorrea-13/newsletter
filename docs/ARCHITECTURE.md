# Architecture

[Español](ARCHITECTURE.es.md)

Newsletter Bot runs AI agents, stores editions as JSON, and publishes static RSS 2.0 feeds. The feed service has no write API.

## Components

```text
config/newsletters/*.json + prompts/<slug>.md
  └── validation + crontab generation
      └── Supercronic
          └── newsletter run <slug>
              ├── configuration + editorial prompt
              ├── recent history and per-newsletter lock
              ├── Codex or Claude adapter
              ├── output validation and sanitization
              ├── history by editorial date
              └── atomic replacement of RSS files
```

| Component | Responsibility |
| --- | --- |
| JSON configuration | Newsletter, prompt, agent, model, schedule, and operational settings. |
| Schedule generator | Validates configuration and generates the Supercronic crontab. |
| Supercronic | Runs scheduled jobs. |
| Go CLI | Configuration, locks, validation, storage, logs, and feeds. |
| Codex / Claude | Research sources and draft editions using the editorial prompt. |
| JSON history | Source of truth for editions and feeds. |
| Feed server | Serves static XML from `data/public/` in read-only mode. |

Go generates the crontab from JSON at container startup. The project has no database, mail service, frontend, or write API.

## Configuration

```text
config/
├── defaults.json
└── newsletters/<slug>.json
prompts/<slug>.md
```

Each JSON file describes a newsletter and references its prompt:

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

`config/defaults.json` sets the timezone and common agent, timeout, history, retention, and `feed_base_url` values. Set `reasoning_effort` per newsletter to pass it to the selected CLI. If you omit it, the provider uses its default.

- `history_context_editions`: recent editions passed to the agent. Default 5, maximum 30.
- `feed_retention_editions`: items per feed. Default 30, maximum 200.
- `feed_base_url`: feed base URL. When a newsletter has no `site_url`, its channel links to `<base>/<slug>.xml`.

Each slug has a matching JSON file and prompt. Go checks the names, prompt location, and configuration before starting Supercronic. Restart the container to apply changes; there is no hot reload.

## Edition run

1. Validate the slug and configuration.
2. Acquire the newsletter lock. A concurrent run for the same slug fails.
3. Calculate the editorial date in the configured timezone.
4. Read the prompt and compact recent-edition context.
5. Run the provider as a separate process with a timeout.
6. Validate the output contract.
7. Save the edition with a temporary file and atomic link, without overwriting an existing date.
8. Regenerate configured feeds and `all.xml`, replacing each file separately.
9. Log the result and release the lock.

To correct an existing edition, back it up and follow [Operations](OPERATIONS.md#correct-an-edition).

## Output contract

The agent returns one JSON object without surrounding text:

```json
{
  "title": "Short edition title",
  "content_html": "<h1>Title</h1><p>Edition body.</p>",
  "stories": [
    {
      "title": "Story title",
      "source": "Source name",
      "url": "https://example.com/article",
      "published_at": "2026-11-11",
      "summary": "Short summary of the main fact."
    }
  ]
}
```

- `title`, `content_html`, and `stories` are required. The schema is in `internal/agent/agent.go`.
- Stories provide recent editorial context. The prompt asks the agent to avoid repeats; a URL may support multiple stories or appear in multiple editions.
- `content_html` permits tags `p`, `h1` through `h3`, `ul`, `li`, `a`, `strong`, `em`, `br`, `img`, `table`, `thead`, `tbody`, `tr`, `th`, and `td`, and attributes `href`, `src`, `alt`, and `width`. Other tags, non-HTTP(S) schemes, stray `<` characters, and comments reject the edition.
- Editions saved before `content_html` retain their plain summaries in feeds.

## Storage and feeds

```text
data/history/<slug>/YYYY-MM-DD.json   complete editions, source of truth
data/state/locks/                     flock locks
data/public/<slug>.xml                per-newsletter feed
data/public/all.xml                   combined feed
```

The application keeps all history. Run `newsletter generate-feeds` to rebuild feeds without calling an agent.

- Each edition is an RSS 2.0 item, sorted by date descending. Individual feed GUIDs use `<slug>:<date>`; `all.xml` uses `all:<date>`. Editions on the same date share a GUID in the combined feed.
- `<description>` contains sanitized HTML escaped as XML text. The reader renders it.
- The channel link comes from `site_url`, then `feed_base_url` plus `/<slug>.xml`, then the latest item's link.
- Go replaces each XML file atomically, not the whole group. A failed regeneration can leave some feeds updated and others unchanged. The saved edition remains in history; run `newsletter generate-feeds` to rebuild.
- Regeneration removes XML files for slugs no longer configured.

## HTTP publishing

`newsletter serve` accepts only `GET` and `HEAD` for `.xml` files in the public directory. It does not list directories or serve other files. `newsletter-feeds` mounts `data/public/` read-only. See [Deployment](DEPLOYMENT.md) for interface and port settings.

## History import

`newsletter import-history <directory>` converts an export of earlier newsletter emails into history. It applies the output contract, sanitizes HTML, requires at least one linked story, and does not overwrite an existing date. `--dry-run` reports the plan without writing.

## Concurrency and failures

- Different newsletters can run concurrently; each slug has its own lock.
- Feed generation uses a global lock to avoid overlapping writes.
- Kernel `flock` locks release when a process exits, including after a crash.
- The application does not retry agent runs.
- The scheduler does not catch up missed jobs in bulk after restart.

## Trust

Agent output and external pages are untrusted data. Go validates the output before saving.

Codex runs with `--sandbox read-only` and `--disable shell_tool`. Claude Code receives `--safe-mode`, `--restricted`, `plan` mode, and only the `WebSearch` and `WebFetch` tools. CLI processes share the bot container and its mounts, including `auth/`; these settings do not replace verification of each provider's boundaries. Go validates and publishes the output.
