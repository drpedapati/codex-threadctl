# Security

`codex-threadctl` is intended for local operator use on a machine that already has Codex installed and authenticated.

## Supported security posture

- Local stdio process only.
- No listener, daemon, webhook, HTTP server, or remote control surface.
- No credential storage.
- No direct filesystem mutation outside normal Codex thread metadata calls.
- Rename mutations require `--confirm`.
- Optional `--expect-current` guards against stale-title mistakes.

## Not supported

- Do not expose this command as a network service.
- Do not run it in shared shells where untrusted users can invoke local Codex app-server state.
- Do not use it for thread deletion or archival; those operations are intentionally not implemented.

## Reporting issues

Open a private issue or contact the repository owner if this repository is private.
