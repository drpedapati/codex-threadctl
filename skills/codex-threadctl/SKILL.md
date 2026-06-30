---
name: codex-threadctl
description: Use when Codex needs to list, audit, create, message, or rename local Codex Desktop threads from shell access through the local app-server JSON-RPC protocol, especially when app thread tools are visible but fail with "No handler registered".
---

# Codex Thread Control

Use `codex-threadctl` when normal Codex app thread tools are unavailable in the current surface but shell access exists on a local machine with Codex installed.

If the binary is not installed, run the bundled source directly:

```bash
go run ~/.codex/skills/codex-threadctl/scripts/codex-threadctl.go list --search Project --limit 5
```

## Safety Rule

Run read-only first:

```bash
codex-threadctl list --search Project --limit 5
```

If `list` fails, do not attempt mutation.

Read a specific thread before mutation:

```bash
codex-threadctl read --id 019...
```

Create a new thread only when the user explicitly asks for a user-owned Codex thread:

```bash
codex-threadctl create \
  --cwd /absolute/project/root \
  --title 'LE | Role | Lane' \
  --message-file kickoff.md
```

Send to an existing thread when the user asks for a handoff or update:

```bash
codex-threadctl send --id 019... --message-file handoff.md
```

`create` and `send` wait for the turn to complete before exiting. Keep kickoff and handoff messages concise when you need a fast mobile-safe coordination update.

Rename with dry-run first:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --dry-run
```

Then apply with confirmation:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --confirm
```

Do not use this helper to delete, archive, or fork threads.

## Common Workflows

Find threads by title or preview:

```bash
codex-threadctl list --search 'PR #32' --limit 20
```

Filter by exact cwd:

```bash
codex-threadctl list \
  --cwd /Users/ernie/Documents/GitHub/clinvision-v2-worktrees \
  --limit 100
```

Return JSON for scripts:

```bash
codex-threadctl list --search Vera --json
```

Create a thread in a known project cwd:

```bash
codex-threadctl create \
  --cwd /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees \
  --title 'LE | Naomi | Project Coordinator Manager' \
  --message-file /tmp/kickoff.md
```

Send a handoff:

```bash
codex-threadctl send --id 019... --message-file /tmp/handoff.md
```

## Implementation Notes

The helper starts `codex app-server --stdio`, sends `initialize`, then sends local app-server JSON-RPC methods such as `thread/list`, `thread/read`, `thread/start`, `turn/start`, and `thread/name/set`. It does not rely on the mobile app MCP thread tool handler.

Prefer normal app thread tools when those handlers work. Use this skill as a fallback bridge for shell/mobile-safe thread coordination.
