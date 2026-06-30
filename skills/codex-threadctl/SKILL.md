---
name: codex-threadctl
description: Use when Codex needs to list, audit, or rename local Codex Desktop threads from shell access through the local app-server JSON-RPC protocol, especially when app thread tools are visible but fail with "No handler registered".
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

Do not use this helper to delete, archive, fork, or create threads.
