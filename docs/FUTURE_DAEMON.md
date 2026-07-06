# Future work: session daemon

> Status: **not implemented**. Placeholder record so this idea is not lost.
> Owner: TBD. Target: post-v1, only if real users hit the fallback UX pain
> described below.

## Why this exists

`proton-cli` persists SRP session tokens in the OS keychain via
`internal/keychain` (see `docs/AGENT.md` and the `feat/keychain-persist`
history). That covers desktop OSes cleanly:

- **macOS** → Keychain Services
- **Windows** → Credential Manager (DPAPI)
- **Linux desktop** → Secret Service (gnome-keyring / KWallet) via D-Bus

It does **not** cover:

- Headless Linux servers with no `gnome-keyring` / no D-Bus session bus.
- SSH sessions into a machine where nobody is graphically logged in.
- WSL without a running Secret Service.
- CI runners.

Today those environments get one of:

1. A hard error from `keyring.Set` (`org.freedesktop.secrets` not available).
2. A fallback file (not yet implemented — see `docs/FUTURE_FALLBACK.md` when it lands).
3. A per-command re-login prompt, which is unusable for scripting.

## The daemon idea

Model: `ssh-agent`.

- A long-running process (`proton-agent`) holds decrypted session tokens
  in **memory only**. Never writes them to disk.
- CLI invocations (`proton mail ls`, etc.) talk to the agent over a
  **Unix domain socket** with tight file-mode permissions (`0600`, owned
  by the invoking user).
- Startup: user runs `proton-agent` (or a systemd user unit / launchd
  plist starts it), then does `proton signin` once. Tokens live in the
  agent for as long as it runs.
- Agent handles the `AuthHandler` refresh loop internally — CLI clients
  just ask for "give me a currently-valid access token for user X" and
  get a fresh one.
- Shutdown: agent zeroes token memory on `SIGTERM` / `SIGINT`. No on-disk
  state to leak.

Socket path convention: `$XDG_RUNTIME_DIR/proton-cli/agent.sock`, falling
back to `/tmp/proton-cli-$UID/agent.sock` with `0700` on the parent dir.

## Why we are not building it now

- Solving keychain persistence (Option A + JSON bundle B2) covers the
  common case (developer laptops, desktop Linux). We do not have
  evidence of headless users yet.
- A daemon is a whole new deploy surface: install scripts, systemd
  units, launchd plists, upgrade story, log location, crash recovery,
  IPC protocol design (protobuf? plain JSON? length-prefixed?),
  authentication of the client (SO_PEERCRED on Linux, `xucred` on
  macOS), replay protection.
- The keychain solution already gets us "type password once per week,
  never again". A daemon is only better for "headless boxes" and for
  "never touch disk". Both are refinements, not requirements.

## Prerequisites before building this

1. Keychain persistence shipped and stable (`feat/keychain-persist`).
2. Encrypted-file fallback shipped for headless (`FUTURE_FALLBACK.md`).
3. Evidence in issues that (2) is not enough — real users asking for
   the daemon.

## Rough scope estimate

- New binary `cmd/agent/main.go`.
- New package `internal/agent/` with socket server + client library.
- IPC protocol: probably line-delimited JSON over Unix socket, with
  fixed request/response schemas (`GET_TOKEN`, `SET_SESSION`,
  `DELETE_SESSION`, `LIST_USERS`, `PING`).
- Peer credential check on every request (reject if PID/UID mismatch).
- Client-side change in `internal/keychain`: try agent socket first,
  fall back to OS keychain, fall back to encrypted file.
- systemd user unit + launchd plist + Windows service manifest (or
  drop Windows: DPAPI already solves it there).
- Tests: in-process socket + mock client, plus a fuzz target on the
  IPC parser because it is the primary attack surface.

## Non-goals

- **Not** a full mail daemon. Only session tokens. Message fetch and
  cache stay in the CLI process.
- **Not** a networked service. Local Unix socket only.
- **Not** a keyring replacement on OSes where the OS keyring already
  works — agent is opt-in for headless.

## References

- ssh-agent protocol (`ssh-agent(1)`, `sshd(8)`, RFC draft `draft-miller-ssh-agent`)
- gpg-agent architecture
- `SO_PEERCRED` (Linux) / `LOCAL_PEERCRED` (macOS) for peer identity
- go-proton-api `AuthHandler` (see `internal/keychain/README.md`)
