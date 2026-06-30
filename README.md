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
- Read the last user/assistant exchange and turn status.
- Create a new project-scoped thread with an initial kickoff message.
- Create a Leading Edge thread with the standard `LE | Role | Lane` title and cwd.
- Send a message to an existing thread.
- Rename a thread safely, for example to add a PR number or ownership label.
- Write a small JSON receipt for thread mutations.
- Check the local Codex app-server bridge with `doctor`.
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
- `thread/resume`
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

Check the latest exchange:

```bash
codex-threadctl last --id 019...
```

Create a project-scoped thread with a kickoff:

```bash
codex-threadctl create \
  --cwd /absolute/project/root \
  --title 'LE | Naomi | Project Coordinator Manager' \
  --message-file kickoff.md
```

Create a Leading Edge thread with the standard cwd/title shape:

```bash
codex-threadctl le-create \
  --role Naomi \
  --lane 'Project Coordinator Manager' \
  --message-file kickoff.md
```

Send a follow-up to an existing thread:

```bash
codex-threadctl send \
  --id 019... \
  --expect-title 'LE | Naomi | Project Coordinator Manager' \
  --expect-cwd /absolute/project/root \
  --message-file handoff.md
```

### Write handoffs as single packets

When sending a handoff from a coordinator thread, prefer one actionable packet instead of a broad status dump. This keeps the source thread usable and prevents important side items from getting buried.

A good handoff packet has:

- one packet type: `Dispatch`, `Decision`, `Evidence Review`, `Risk`, or `Housekeeping`
- one owner
- one recommended action
- one completion condition
- one final control move: `Approve dispatch`, `Choose A/B`, `Wait for evidence`, `Keep blocked`, or `No action needed`
- receipts only at the end

Use plain English first:

```text
Current packet:
SQLite merge readiness is blocked on Mara's VM smoke.

Why this matters:
The stack may be locally ready, but it should not merge until the VM proof lands.

Recommended action:
Run one mock-only VM smoke against PR #51 at e506d15c.

Completion condition:
Report PASS/BLOCKED/FAIL with source SHA, dirty state, vm-doctor, SQLCipher readiness, and fixture evidence.

Control move:
Wait for evidence
```

Avoid using `send` as a hidden workflow engine. Dispatch the packet, write a receipt when useful, then return control to the source thread unless the user explicitly asks you to watch the target thread live.

### Run coordinator threads as packet queues

Coordinator threads should behave like a control queue, not a static report archive.

Default loop:

1. Present one packet.
2. Get the control move or make the approved dispatch.
3. Close the packet when its completion condition is true or when it is handed off.
4. Surface the next best packet.
5. If no packet is ready, state what evidence or event is missing.

Use this closeout line when a packet is done:

```text
Packet closed. Next packet: <short title>.
```

Use this waiting line when there is nothing actionable:

```text
No packet ready. Waiting for <owner/evidence/event>.
```

Use event-based heartbeat by default. Do not simulate a timer in the source thread. Bring a new packet when the user asks for status, a target thread reports evidence or a blocker, a dispatch fails, a lane changes, or a risk needs attention.

### Sweep before saying "wait"

If the next packet would only say "wait," run a quick project sweep first. The sweep is not a dashboard for the user; it is how a coordinator chooses the next useful packet.

Sweep for:

- current routing/thread/worktree state
- active PR or branch stack and merge order
- latest target-thread receipt or last-turn summary
- blockers and do-not-touch constraints
- evidence required before merge, deploy, or rebase
- adjacent prep work that does not bypass the blocker

The sweep should produce one of:

- an adjacent prep packet that can safely move now
- a risk packet that needs attention before the blocker clears
- a clear statement that no safe adjacent work should move

Example:

```text
Mara evidence is still pending, so the merge train stays blocked. The useful adjacent packet is merge/rebase preparation: confirm the PR order, name the merge owner, and pre-stage the conflict plan without merging anything.
```

### Update coordinator operating rules consistently

When a project changes how coordinator threads communicate, dispatch, queue packets, run heartbeat, or sweep for next work, make the change in every place that owns the behavior.

Use this workflow:

1. Restate the change as one actionable packet.
2. Update the project-local coordinator template first, if one exists.
3. Decide whether the behavior is generic to `codex-threadctl` handoffs.
4. If generic, update both:
   - `README.md`
   - `skills/codex-threadctl/SKILL.md`
5. Validate any project-local routing or map checks.
6. Review the public repo diff.
7. Commit and push the `codex-threadctl` change.
8. Report back as one housekeeping packet with the commit hash, validation result, and next packet.

Do not leave the public thread-control guidance stale when the change affects `send`, handoff shape, packet queues, heartbeat, or project sweep behavior.

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

## ClinVision Leading Edge usage

ClinVision uses `codex-threadctl` as the mobile-safe bridge for managing Codex Desktop threads when the app thread tools are unavailable from the current surface.

The main project that drove this tool is the Leading Edge worktree group:

```text
/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees
```

In that group, thread names carry routing status in the first four characters so they remain readable in a narrow mobile sidebar:

```text
LE-M = Leading Edge mapped worktree lane
LE-T = Leading Edge thread-only lane
```

Use that distinction before deciding how to work:

| Prefix | Meaning | Developer behavior |
| --- | --- | --- |
| `LE-M` | The thread maps to a branch and worktree folder. | Find the entry in `role-worktree-map.json`, verify the folder, then edit only in that mapped worktree. |
| `LE-T` | The thread is coordination/advisory/roundtable only. | Use it for discussion, handoff, planning, and status. Do not assume it owns a code folder. |

The current coordinator thread is:

```text
LE-T | Naomi | Control Tower
019f1932-5f10-7933-abb2-8acb8b324dec
/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees
```

Naomi Control Tower routes work across the role lanes. Control Tower can request status, hand off work, and coordinate sequencing, but it does not replace the specialist owners. Mara still owns build/deploy truth, Vivian owns role/worktree governance, Vera owns control-panel UX, Cal owns simplification review, Rafi owns observability/profiling lanes, and Julian is currently thread-only loop engineering.

Before sending to Control Tower, use guarded send:

```bash
codex-threadctl send \
  --id 019f1932-5f10-7933-abb2-8acb8b324dec \
  --expect-title 'LE-T | Naomi | Control Tower' \
  --expect-cwd /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees \
  --message-file handoff.md \
  --receipt threadctl-receipt.json
```

Before creating a new Leading Edge thread, use `le-create`:

```bash
codex-threadctl le-create \
  --role Vivian \
  --lane 'Role Map Cleanup' \
  --message-file kickoff.md
```

Then immediately decide whether the thread is mapped or thread-only:

```text
Mapped implementation lane -> rename to LE-M and add kind: mapped-worktree
Coordination-only lane      -> rename to LE-T and add kind: thread-only
```

The source-controlled map is:

```text
/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees/role-worktree-map.json
```

Validate it with:

```bash
/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees/verify-role-map.py --json
```

The developer correspondence explaining this system is published at:

```text
http://data1.netbird.selfhosted:8890/c/206-leading-edge-thread-routing-control-tower/
```

The practical rule is simple: `codex-threadctl` can create and send messages, but it should not become a hidden workflow engine. Use it to keep the visible Codex thread map coherent, guarded, and auditable.

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

### `last`

Read the last turn summary:

```bash
codex-threadctl last --id 019...
```

This shows the thread title, cwd, last turn status, last user message, and last assistant message. Use JSON for scripts:

```bash
codex-threadctl last --id 019... --json
```

### `create`

Create a new thread, set its title, and start a kickoff turn:

```bash
codex-threadctl create \
  --cwd /absolute/path/to/project \
  --title 'LE | Role | Lane Name' \
  --message-file kickoff.md \
  --receipt thread-create-receipt.json
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
codex-threadctl send \
  --id 019... \
  --expect-title 'LE | Role | Lane Name' \
  --expect-cwd /absolute/path/to/project \
  --message-file handoff.md
```

Use `--expect-title` and `--expect-cwd` whenever possible. They fail closed when mobile context is stale.

When `--cwd` is omitted, `send` resumes the target thread and uses that thread's cwd. Override it only when you know the thread metadata is incomplete:

```bash
codex-threadctl send \
  --id 019... \
  --cwd /absolute/path/to/project \
  --message 'Status request'
```

Like `create`, `send` waits for completion before exiting.

### `le-create`

Create a thread in the standard Leading Edge project hub:

```bash
codex-threadctl le-create \
  --role Naomi \
  --lane 'Project Coordinator Manager' \
  --message-file kickoff.md
```

Defaults:

- cwd: `/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees`
- title: `LE | <Role> | <Lane>`

This does not edit `role-worktree-map.json`; it only creates the Codex thread. Use Vivian/role-map workflow for source-controlled map updates.

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

Then apply with `--confirm`; add `--receipt` when you want an audit file:

```bash
codex-threadctl rename \
  --id 019... \
  --name 'LE | Role | PR #124 - Deployment Proof' \
  --expect-current 'LE | Role | Runtime Evidence' \
  --receipt rename-receipt.json \
  --confirm
```

If the current title differs, the command fails:

```text
error: current title mismatch: expected "...", got "..."
```

That is intentional. It prevents stale mobile context from renaming the wrong thing.

### `doctor`

Check the local bridge:

```bash
codex-threadctl doctor
```

JSON output:

```bash
codex-threadctl doctor --json
```

The doctor checks whether `codex` is on `PATH`, whether the local app-server initializes, and whether `thread/list` works.

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

5. Create or send only when you intend to start a real Codex turn. Prefer guarded sends:

   ```bash
   codex-threadctl create --cwd /absolute/project/root --title 'LE | Role | Lane' --message-file kickoff.md
   ```

   ```bash
   codex-threadctl send \
     --id 019... \
     --expect-title 'LE | Role | Lane' \
     --expect-cwd /absolute/project/root \
     --message-file handoff.md \
     --receipt handoff-receipt.json
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

The tool can create threads, send messages, and rename local Codex thread metadata. Treat those as real mutations. Use read-only commands first. Use `send --expect-title --expect-cwd`; use rename `--dry-run` plus `--expect-current`.

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
