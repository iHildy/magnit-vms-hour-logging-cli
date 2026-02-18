---
name: magnit-vms-cli-non-interactive
description: Use this skill to run the `magnit` CLI in fully non-interactive mode for authentication, logging, and verification. Trigger when an agent must log work hours deterministically via command flags (not natural language), avoid prompts, emit JSON for automation, or operate in CI/headless execution.
---

# Magnit VMS CLI Non-Interactive

## Overview

Execute `magnit` commands with explicit flags so the run is deterministic, scriptable, and prompt-free.
Prefer `--json`, `--yes`, `--engagement`, and `--password-stdin` where needed to avoid interactive input.

## Non-Interactive Workflow

1. Resolve executable:
- Prefer `./magnit` if present in the repo root.
- If missing, run `go build ./cmd/magnit` first.

2. Authenticate non-interactively:
- Run `./magnit auth login --username <email> --password '<password>'`.
- Alternative for complex passwords: `printf '%s' '<password>' | ./magnit auth login --username <email> --password-stdin`.

3. Verify authentication:
- Run `./magnit auth status --json`.
- Proceed only when `"authenticated": true`.

4. Configure defaults (optional but recommended):
- Set engagement once: `./magnit config set-default-engagement --id <engagement_id>`.
- Set timezone once: `./magnit config set-timezone --tz <IANA_TZ>`.

5. Run logging operations:
- Set day spans (authoritative replace):
`./magnit set --date YYYY-MM-DD --span labor:09:00-12:00 --span lunch:12:00-12:30 --span labor:12:30-17:00 --engagement <id> --yes --json`
- Mark did-not-work day:
`./magnit mark-dnw --date YYYY-MM-DD --engagement <id> --yes --json`
- Read back day state:
`./magnit show --date YYYY-MM-DD --engagement <id> --json`

6. Use dry-run before write when safety is required:
- `./magnit set ... --dry-run --json`
- `./magnit mark-dnw ... --dry-run --json`

## Command Patterns

Authentication:

```bash
./magnit auth login --username "$MAGNIT_USERNAME" --password "$MAGNIT_PASSWORD"
or
printf '%s' "$MAGNIT_PASSWORD" | ./magnit auth login --username "$MAGNIT_USERNAME" --password-stdin
./magnit auth status --json
```

Set hours for one day:

```bash
./magnit set \
  --date 2026-02-18 \
  --span labor:09:00-12:00 \
  --span lunch:12:00-12:30 \
  --span labor:12:30-17:00 \
  --engagement "$MAGNIT_ENGAGEMENT_ID" \
  --yes \
  --json
```

Mark day as did-not-work:

```bash
./magnit mark-dnw --date 2026-02-19 --engagement "$MAGNIT_ENGAGEMENT_ID" --yes --json
```

Verify day:

```bash
./magnit show --date 2026-02-18 --engagement "$MAGNIT_ENGAGEMENT_ID" --json
```

## Reliability Rules

- Always pass `--json` for machine-readable output.
- Always pass `--yes` for non-interactive writes to bypass confirmation prompts.
- Always pass `--engagement <id>` unless a default engagement is already configured.
- Always use `YYYY-MM-DD` dates and `type:HH:MM-HH:MM` spans.
- Treat non-zero exit status as failure.
- Prefer `--password-stdin` for passwords with shell-sensitive characters and to reduce shell-history/process-list exposure:
`printf '%s' "$MAGNIT_PASSWORD" | ./magnit auth login --username "$MAGNIT_USERNAME" --password-stdin`.

## Failure Handling

- If auth fails (`authenticated: false`), run `auth login` again; for complex passwords prefer `--password-stdin` over `--password`.
- If a command errors on engagement resolution, pass `--engagement <id>` or configure a default.
- If span validation fails, fix overlaps/order/format and rerun.
- If uncertain about payload impact, rerun with `--dry-run --json`.
