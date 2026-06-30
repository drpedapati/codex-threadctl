# codex-threadctl

`codex-threadctl` is a small local CLI for managing Codex Desktop threads when you are working from a surface that cannot call the normal app thread tools.

The motivating case is mobile: Codex may show tools such as `list_threads` or `set_thread_title`, but the call fails with:

```text
No handler registered for tool: list_threads
```

If you still have shell access to the same Mac that runs Codex Desktop, `codex-threadctl` talks to the local Codex app-server over stdio and performs a narrow set of thread operations.

## What this is for

Use this when you need to:

- Find a Codex thread by title or preview text.
- Read one thread's current title and project cwd.
- Create a new project-scoped thread with an initial kickoff message.
- Send a message to an existing thread.
- Rename a thread safely, for example to add a PR number or ownership label.
- Do the above from a mobile-driven Codex session by running shell commands on the local Mac.

Do not use it as a general Codex automation platform. It is deliberately small and local-only.

## What it does not do

`codex-threadctl` intentionally does not:

- Fork threads.
- Archive or delete threads.
- Expose a network service.
- Manage remote hosts.
- Bypass Codex authentication.
- Read full turn history.

It only uses the local Codex app-server methods:

- `thread/list`
- `thread/read`
- `thread/start`
- `turn/start`
- `thread/name/set`

## Requirements

- macOS or another environment where the Codex CLI is installed.
- `codex` available on `PATH`.
- A working local Codex app-server protocol via `codex app-server --stdio`.
- Go, if building from source.

Check the basics:

```bash
command -v codex
codex --version
codex-threadctl version
```

## Install

Install from GitHub:

```bash
go install github.com/drpedapati/codex-threadctl/cmd/codex-threadctl@latest
```

Make sure your Go bin directory is on `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Build from a local checkout:

```bash
git clone https://github.com/drpedapati/codex-threadctl.git
cd codex-threadctl
go build -o ~/.local/bin/codex-threadctl ./cmd/codex-threadctl
```

## Fast path

Always start with read-only:

```bash
codex-threadctl list --search "Project" --limit 5
```

Read the exact thread:

```bash
codex-threadctl read --id 019...
```

Create a project-scoped thread with a kickoff:

```bash
codex-threadctl create \
  --cwd /absolute/project/root \
  --title 'LE | Naomi | Project Coordinator Manager' \
  --message-file kickoff.md
```

Send a follow-up to an existing thread:

```bash
codex-threadctl send \
  --id 019... \
  --message-file handoff.md
```

Dry-run the rename:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --dry-run
```

Apply only after the dry run is correct:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'V2 | Role | PR #123 - Short Lane' \
  --expect-current 'V2 | Role | Old Lane' \
  --confirm
```

## Commands

### `list`

List threads by title or preview substring:

```bash
codex-threadctl list --search "PR #32" --limit 20
```

Filter to one project cwd:

```bash
codex-threadctl list --cwd /absolute/path/to/project --limit 100
```

Emit JSON for scripts:

```bash
codex-threadctl list --search "runtime" --json
```

Plain output is tab-separated:

```text
<thread-id>    <title>    <cwd>    <preview>
```

### `read`

Read one thread's metadata:

```bash
codex-threadctl read --id 019...
```

Typical output:

```text
id          019...
title       V2 | Role | Old Lane
cwd         /absolute/path/to/project
source      vscode
updatedAt   1782828739
preview     ...
```

Use JSON when another script needs the result:

```bash
codex-threadctl read --id 019... --json
```

### `create`

Create a new thread, set its title, and start a kickoff turn:

```bash
codex-threadctl create \
  --cwd /absolute/path/to/project \
  --title 'LE | Role | Lane Name' \
  --message-file kickoff.md
```

Use inline text for short messages:

```bash
codex-threadctl create \
  --cwd /absolute/path/to/project \
  --title 'LE | Roundtable | SQLite Planning' \
  --message 'Please orient read-only and report current project state.'
```

`create` waits for the kickoff turn to complete before exiting. This is intentional: the local app-server process owns the turn lifecycle, so fire-and-forget exits can leave no durable thread.

JSON output is available for scripts:

```bash
codex-threadctl create \
  --cwd /absolute/path/to/project \
  --title 'LE | Role | Lane Name' \
  --message-file kickoff.md \
  --json
```

### `send`

Send a message to an existing thread:

```bash
codex-threadctl send --id 019... --message-file handoff.md
```

When `--cwd` is omitted, `send` reads the target thread and uses that thread's cwd. Override it only when you know the thread metadata is incomplete:

```bash
codex-threadctl send \
  --id 019... \
  --cwd /absolute/path/to/project \
  --message 'Status request'
```

Like `create`, `send` waits for completion before exiting.

### `rename`

Rename has three safeguards:

1. It reads the thread before mutating.
2. It refuses to mutate without `--confirm`.
3. It reads the thread after mutating and verifies the new title.

Use `--expect-current` whenever possible:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'LE | Role | PR #124 - Deployment Proof' \
  --expect-current 'LE | Role | Runtime Evidence' \
  --dry-run
```

If the current title differs, the command fails:

```text
error: current title mismatch: expected "...", got "..."
```

That is intentional. It prevents stale mobile context from renaming the wrong thing.

## Mobile workflow

When mobile Codex cannot execute app thread tools:

1. Run a read-only probe from shell:

   ```bash
   codex-threadctl list --search "Project" --limit 1
   ```

2. If that fails, stop. The local app-server path is not healthy.

3. If it works, find the target thread:

   ```bash
   codex-threadctl list --search "PR #123" --limit 20
   ```

4. Read the target thread:

   ```bash
   codex-threadctl read --id 019...
   ```

5. Create or send only when you intend to start a real Codex turn:

   ```bash
   codex-threadctl create --cwd /absolute/project/root --title 'LE | Role | Lane' --message-file kickoff.md
   ```

   ```bash
   codex-threadctl send --id 019... --message-file handoff.md
   ```

6. Dry-run the rename with `--expect-current`.

7. Apply with `--confirm`.

## Installing the Codex skill

This repo includes a Codex skill in `skills/codex-threadctl`.

To install it locally:

```bash
mkdir -p ~/.codex/skills
cp -R skills/codex-threadctl ~/.codex/skills/
```

After installation, future Codex sessions can discover the skill when you ask for thread management from a mobile/shell fallback path.

## Troubleshooting

### `No handler registered for tool: list_threads`

This is the failure this tool is meant to work around. Use `codex-threadctl list ...` from the local shell instead of the app tool.

### `error: no response from codex app-server`

The local Codex app-server did not return a response before timeout.

Try:

```bash
codex --version
codex app-server --stdio
```

Then press `Ctrl+C`; that second command should start rather than immediately fail.

### `json-rpc error -32600: Not initialized`

This should not happen when using `codex-threadctl`; the tool sends `initialize` before thread calls. If you see it, file an issue with the command and Codex version.

### Rename refused without `--confirm`

Expected. Use `--dry-run` first, then repeat with `--confirm`.

### `create` or `send` waits longer than expected

Expected for long-running prompts. The command waits so the started turn can persist correctly. Use shorter kickoff/handoff messages when you need fast mobile-safe thread coordination.

### Current title mismatch

Expected when the thread was already renamed or you copied stale context. Run `read` again and decide whether the new current title is the one you intended to mutate.

## Security model

`codex-threadctl` is local-only. It starts `codex app-server --stdio` as a child process and communicates through stdin/stdout. It does not open ports, store tokens, read credentials, or run a daemon.

The tool can create threads, send messages, and rename local Codex thread metadata. Treat those as real mutations. Use read-only commands first, and use rename `--dry-run` plus `--expect-current`.

## Development

Run checks:

```bash
go test ./...
go build ./cmd/codex-threadctl
```

Run locally without installing:

```bash
go run ./cmd/codex-threadctl list --search Project --limit 5
```

Validate the bundled skill if you have the Codex skill creator tools installed:

```bash
python3 ~/.codex/skills/.system/skill-creator/scripts/quick_validate.py skills/codex-threadctl
```
