# Operations and recovery

[Español](OPERATIONS.es.md)

Use Docker Compose and `compose.prod.yaml` for the commands below. Set `NEWSLETTER_DIR` and `FEED_PORT` as described in [Deployment](DEPLOYMENT.md). For the design, see [Architecture](ARCHITECTURE.md).

## Configuration changes

Each newsletter has `config/newsletters/<slug>.json` and `prompts/<slug>.md`.

- **Schedule:** edit `schedule.days` or `schedule.time` in the JSON.
- **Editorial behavior:** edit the matching Markdown prompt.
- **New newsletter:** create both files with the same slug.
- **Disable:** set `"enabled": false`.

Validate before restarting the container:

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot validate-config
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-crontab
```

Invalid configuration prevents scheduler startup. Do not edit a separate crontab or put schedules in prompts.

## Status

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot status
docker compose -f compose.prod.yaml ps
docker compose -f compose.prod.yaml logs -f newsletter-bot
```

`status` shows whether each newsletter is enabled, its agent, schedule, and latest edition date. It does not print titles, summaries, prompts, or credentials.

Each `run` logs JSON start and end events with slug, agent, requested model, result, and duration. If `model` is unset, the log records `provider-default`; that does not identify the model behind a provider alias.

## Manual run

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot run <slug>
```

`--rm` removes the one-off container after the run. A run consumes provider usage. If an edition exists for the date, Go rejects the run before calling the agent. Different slugs can run in parallel; wait for an active run of the same slug to finish.

## History import

```sh
docker compose -f compose.prod.yaml run --rm -T \
  -v "$PWD/import:/import:ro" newsletter-bot \
  import-history /import/<directory> --dry-run
```

Review the plan, then repeat without `--dry-run`. The import does not overwrite an existing date. It regenerates feeds at the end.

## Feeds

`generate-feeds` rebuilds XML from history without calling agents or modifying JSON:

```sh
docker compose -f compose.prod.yaml run --rm newsletter-bot generate-feeds
```

The feed service reads `data/public/` read-only. Rebuild feeds after changing `feed_base_url` or `site_url` to update channel links.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Scheduler does not run jobs | Scheduler status, crontab, timezone, and container logs. |
| Agent not found | Binary, architecture, and image PATH. |
| Authentication error | Session in `auth/`, HOME, and permissions. Reauthenticate using [Deployment](DEPLOYMENT.md). |
| Quota or rate limit | Provider error and account plan. Do not retry in a loop. |
| Timeout | Duration, source availability, and configured timeout. Increase it only with evidence. |
| `invalid content_html` | Unsupported HTML tags or attributes, or a stray `<`. The edition is not saved; inspect the prompt and result. |
| `story N has invalid required fields or URL` | Missing title, source, absolute URL, or summary. Inspect the prompt. Do not relax validation. |
| `edition already exists` | An edition already exists for the date. |
| Empty feed or 404 | Rebuild with `generate-feeds`; check `data/public/` and the feed-service mount. |
| Lock held | Check for another active run. The process releases its lock on exit; do not delete lock files in use. |

## Correct an edition

1. Copy `data/history/<slug>/<date>.json` outside `data/`.
2. Edit or delete the JSON as needed. Check file permissions.
3. Run `generate-feeds`.
4. Validate the resulting XML.

If Go saved the edition but feed regeneration failed, run `generate-feeds`. Go replaces XML files one at a time, so some may have changed before the failure.

## Backups

- **Daily:** `data/history/` and `data/state/`.
- **Credentials:** `auth/` in encrypted storage with restricted access, or reauthenticate after restoration.
- **Regenerable:** `data/public/`, using `generate-feeds`.

Test restoration: restore data, check container-user permissions, regenerate feeds, validate XML, and check the feed service response.

## Operational security

- The feed service only serves XML from `data/public/`.
- Protect JSON, state, and credentials.
- The scheduler publishes no ports. The feed service requires the `feeds` profile.
- The deployment uses neither privileged mode nor the Docker socket.
- Treat external pages as untrusted data.
- Public exposure, DNS, tunnels, or port changes require explicit approval.
