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

When canonical/recovery/probe/stale thread risk matters, run a read-only audit before mutation:

```bash
codex-threadctl audit \
  --search Naomi \
  --expect-title 'LE-T | Naomi | Control Tower' \
  --expect-cwd /absolute/project/root \
  --stale-after 168h \
  --role-map /absolute/project/root/role-worktree-map.json
```

Treat `audit` as an input, not the only truth. With `--role-map`, it can flag `role_current`, `role_previous`, `role_status_<status>`, and `role_unmapped`; still cross-check suspicious threads with `last` before archive/rename decisions.

If `list` fails, do not attempt mutation.

Read a specific thread before mutation:

```bash
codex-threadctl read --id 019...
```

Check the bridge before relying on it:

```bash
codex-threadctl doctor
```

Create a new thread only when the user explicitly asks for a user-owned Codex thread:

```bash
codex-threadctl create \
  --cwd /absolute/project/root \
  --title 'LE | Role | Lane' \
  --message-file kickoff.md
```

For Leading Edge threads, prefer the helper:

```bash
codex-threadctl le-create \
  --role Naomi \
  --lane 'Project Coordinator Manager' \
  --message-file kickoff.md
```

Send to an existing thread when the user asks for a handoff or update:

```bash
codex-threadctl send \
  --id 019... \
  --expect-title 'LE | Role | Lane' \
  --expect-cwd /absolute/project/root \
  --message-file handoff.md \
  --wait-timeout 10m
```

`create` waits for the initial turn to complete before exiting. Normal `send` waits for the target turn up to the timeout and then verifies that the sent message is visible as the target thread's latest user message. Keep kickoff and handoff messages concise when you need a fast mobile-safe coordination update.

For `send`, use bounded waits when the target may do real work:

```bash
codex-threadctl send \
  --id 019... \
  --message-file handoff.md \
  --wait-timeout 10m
```

Use dispatch-only mode when the source thread should return control quickly:

```bash
codex-threadctl send \
  --id 019... \
  --message-file handoff.md \
  --no-wait
```

`--no-wait` is intentionally weak. It returns `request_started`, skips delivery verification, and must not be treated as a completed dispatch.

Use `smoke-send` before critical handoffs or when a thread may be stale:

```bash
codex-threadctl smoke-send \
  --id 019... \
  --expect-title 'LE | Role | Lane' \
  --expect-cwd /absolute/project/root \
  --marker THREADCTL_SMOKE_20260701T003516Z \
  --wait-timeout 2m
```

`smoke-send` passes only when the target thread's latest user message contains the marker and the assistant reply contains `ACK <marker>`.

`send` delivery is not the same as work success. `delivery_verified` means the target thread received the latest user message. It does not mean the requested work succeeded. If a send returns `delivery_unverified`, `request_started`, `wait_timeout`, or `interrupted`, run `last` and verify repo, PR, runtime, or evidence truth before redispatching.

For coordinator handoffs, use a single actionable packet. Do not send a broad dashboard unless the user explicitly asks for one.

Packet shape:

```text
Packet type:
<Dispatch | Decision | Evidence Review | Risk | Housekeeping>

Product alignment:
<final product outcome served, product pillar, work type, why now, and cost of not doing it>

Current packet:
<one related bundle the target can act on>

Why this matters:
<why this is the next useful move>

Recommended action:
<one action, decision, review, or hold>

Completion condition:
<what makes this packet done>

Return path:
<exact command or instruction for sending PASS/BLOCKED/FAIL back to the coordinator thread>

Control move:
<Approve dispatch | Choose A/B | Wait for evidence | Keep blocked | No action needed>

Receipts:
<only the exact facts needed to trust this packet>
```

After sending a packet, return control to the source thread unless the user explicitly asks you to watch the target turn live. Do not leave the source thread blocked on long specialist work. For project coordination, do not close the packet on outbound send alone; close it only when the target returns PASS/BLOCKED/FAIL or independent repo/runtime/evidence truth proves the outcome.

If the project has a local North Star, product charter, PRD replacement, or mission document, align the packet to it before dispatch. Ask what final outcome the packet serves, which product pillar it moves, whether it is direct progress/enabling work/risk reduction/housekeeping, why it matters now, what gets worse if skipped, and whether it needs a simplification/product challenge. If it cannot answer those questions, park it, rewrite it, or convert it into a challenge/review packet.

For coordinator threads, treat packets as a queue:

1. Present one packet.
2. Get the control move or make the approved dispatch.
3. Close the packet when its completion condition is true or when it is handed off.
4. Surface the next best packet.
5. If no packet is ready, state what evidence or event is missing.

Use event-based heartbeat by default. Do not simulate a timer in the source thread. Bring a new packet when the user asks for status, a target thread reports evidence or a blocker, a dispatch fails, a lane changes, or a risk needs attention.

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

When a project changes how coordinator threads communicate, dispatch, queue packets, run heartbeat, or sweep for next work, update the project-local coordinator template first if one exists. If the behavior is generic to `codex-threadctl` handoffs, update both `README.md` and this skill file in the public repo, validate project-local routing checks, review the diff, commit, push, and report back as one housekeeping packet. Do not leave the public guidance stale when the change affects `send`, handoff shape, packet queues, heartbeat, or project sweep behavior. If the change affects product direction or alignment gates, update the project-local North Star first or explicitly say why it does not change.

Closeout examples:

```text
Packet closed. Next packet: <short title>.
```

```text
No packet ready. Waiting for <owner/evidence/event>.
```

Use `last` after a handoff when you need turn-level readback:

```bash
codex-threadctl last --id 019...
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

Audit likely coordinator threads before relying on one canonical target:

```bash
codex-threadctl audit \
  --search Naomi \
  --limit 50 \
  --expect-title 'LE-T | Naomi | Control Tower' \
  --expect-cwd /absolute/path/to/project \
  --stale-after 168h
```

Plain audit output is tab-separated:

```text
<thread-id>    <title>    <cwd>    <source>    <last-activity>    <flags>    <preview>
```

Flags include `canonical_title`, `canonical_cwd`, `title_mismatch`, `cwd_mismatch`, `recovery`, `probe`, `stale`, `missing_title`, `missing_cwd`, and `ok`. Use this before guarded sends when multiple similarly named threads exist or when a coordinator thread may be stale.

Create a thread in a known project cwd:

```bash
codex-threadctl create \
  --cwd /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees \
  --title 'LE | Naomi | Project Coordinator Manager' \
  --message-file /tmp/kickoff.md
```

Create a Leading Edge thread:

```bash
codex-threadctl le-create \
  --role Naomi \
  --lane 'Project Coordinator Manager' \
  --message-file /tmp/kickoff.md
```

Send a handoff:

```bash
codex-threadctl send \
  --id 019... \
  --expect-title 'LE | Naomi | Project Coordinator Manager' \
  --expect-cwd /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees \
  --message-file /tmp/handoff.md \
  --receipt /tmp/threadctl-handoff-receipt.json
```

Write coordinator handoffs as one actionable packet, not a broad dashboard. If the project has a North Star, product charter, or mission document, include a behavior-centered goal chain:

```text
Goal chain:
<current action>
  -> <proximal product capability>
  -> <user behavior enabled>
  -> <North Star outcome>
```

The user-behavior line is required for meaningful alignment. If the handoff cannot name what a user, reviewer, operator, developer, or coordinator can do differently because of the packet, park it, rewrite it, or convert it into a challenge/review packet.

When packet selection is getting too local, run a production-distance reconstruction before sending another packet. The reconstruction should map current action to proximal capability, user behavior, and North Star outcome; list production components, evidence, gaps, blockers, sequencing, and the candidate next packet. Do not paste the full map by default.

## ClinVision Leading Edge Pattern

For ClinVision Leading Edge work, thread names use a compact routing prefix:

```text
LE-M = mapped worktree lane
LE-T = thread-only coordination/advisory/roundtable lane
```

Read this before creating or renaming Leading Edge threads:

```text
http://data1.netbird.selfhosted:8890/c/206-leading-edge-thread-routing-control-tower/
```

Current Control Tower thread:

```text
title: LE-T | Naomi | Control Tower
id: 019f1932-5f10-7933-abb2-8acb8b324dec
cwd: /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees
```

Use guarded sends for Control Tower:

```bash
codex-threadctl send \
  --id 019f1932-5f10-7933-abb2-8acb8b324dec \
  --expect-title 'LE-T | Naomi | Control Tower' \
  --expect-cwd /Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees \
  --message-file /tmp/handoff.md \
  --receipt /tmp/threadctl-handoff-receipt.json
```

After creating a Leading Edge thread, decide immediately:

```text
Mapped implementation lane -> rename to LE-M and update role-worktree-map.json with kind: mapped-worktree.
Thread-only lane -> rename to LE-T and update role-worktree-map.json with kind: thread-only.
```

Validate the map:

```bash
/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees/verify-role-map.py --json
```

## Implementation Notes

The helper starts `codex app-server --stdio`, sends `initialize`, then sends local app-server JSON-RPC methods such as `thread/list`, `thread/read`, `thread/resume`, `thread/start`, `turn/start`, and `thread/name/set`. It does not rely on the mobile app MCP thread tool handler.

Prefer normal app thread tools when those handlers work. Use this skill as a fallback bridge for shell/mobile-safe thread coordination.
