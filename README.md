# codex-threadctl

`codex-threadctl` is a small local command-line bridge for Codex Desktop thread metadata.

It exists for one narrow case: a Codex surface, often mobile, may show thread-management tool metadata but fail at runtime with `No handler registered`. If shell access to the local Mac is still available, this utility can talk directly to the local `codex app-server --stdio` JSON-RPC protocol.

## What it can do

- List local Codex threads.
- Read one thread's metadata.
- Rename a thread title with explicit guards.

## What it intentionally does not do

- It does not delete, archive, fork, or create threads.
- It does not expose a network service.
- It does not manage remote hosts.
- It does not read turns unless Codex app-server metadata includes preview text.
- It does not bypass Codex authentication or permissions.

## Install

```bash
go install github.com/drpedapati/codex-threadctl/cmd/codex-threadctl@latest
```

For local development:

```bash
go build -o ~/.local/bin/codex-threadctl ./cmd/codex-threadctl
```

## Usage

Start with a read-only probe:

```bash
codex-threadctl list --search "Project" --limit 5
```

Read one thread:

```bash
codex-threadctl read --id 019...
```

Dry-run a rename:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --dry-run
```

Apply the rename only after the dry run looks right:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --confirm
```

## Safety model

`rename` fails closed unless `--confirm` is present. Use `--expect-current` to prevent stale-thread mistakes. Successful renames are read back and verified.

This tool is local-only. It starts `codex app-server --stdio`, sends `initialize`, then sends `thread/list`, `thread/read`, or `thread/name/set`.

## Skill

The `skills/codex-threadctl` folder contains a Codex skill that teaches future agents when and how to use this tool.
