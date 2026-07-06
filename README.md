# proton

A command-line interface for Proton Mail — manage your account, emails, and more from the terminal.

> **Status:** Early development. Signup helper, SRP sign-in, and OS-keychain
> session persistence are functional. TOTP 2FA and the mail commands are next.

## Features

### ✅ Signup Helper
- **Check username availability** against the Proton API with alternative suggestions (batch mode + `--json`)
- **Password strength validator** matching Proton's own score buckets and penalty labels
- **Generate config template** (`account.yaml`) for signup details
- **Interactive form filler** — copies each field to your clipboard step-by-step while you fill the browser form

### ✅ Authentication
- **SRP-6a sign-in** via [`go-proton-api`](https://github.com/ProtonMail/go-proton-api) — password never leaves the machine
- **Session persistence** in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **Auto-refresh** — background token rotations from the API are re-persisted transparently
- **`signin --status` / `signin --logout`** for saved-session inspection and cleanup

### 🚧 Coming Soon
- **TOTP two-factor authentication** on top of the SRP handshake
- **Two-password mode** mailbox unlock (for encrypted-key accounts)
- **Mail** — fetch, read, reply, send, and search emails from the terminal
- **Contacts** — encrypted address book
- **Session daemon** for headless environments (design sketch in `docs/FUTURE_DAEMON.md`)

## Installation

### Prerequisites
- [Go](https://go.dev/dl/) 1.21+
- A clipboard tool for the interactive fill flow:
  - **macOS** — `pbcopy` (built-in)
  - **Windows** — `clip` (built-in)
  - **WSL** — `clip.exe` (built-in, reachable from Linux)
  - **Linux (Wayland)** — `wl-copy` from [`wl-clipboard`](https://github.com/bugaevc/wl-clipboard)
  - **Linux (X11)** — `xclip` or `xsel`
- An OS keyring for session persistence:
  - **macOS / Windows** — built-in, nothing to install
  - **Linux desktop** — `gnome-keyring` or `kwallet` (Secret Service over D-Bus)
  - **Headless Linux / WSL** — the keyring backend is not available; `signin`
    still works for the current shell but the session is not persisted. See
    `docs/FUTURE_DAEMON.md` for the planned fallback.

### Build from source

```bash
git clone https://github.com/marcosvdn7/proton.git
cd proton
go build -o proton .
```

Optionally, move to your PATH:

```bash
sudo mv proton /usr/local/bin/
```

## Usage

```
proton — Proton Mail CLI tool

Commands:
  signup    Account creation helper
  signin    Sign in to your Proton account (SRP + keychain)
  mail      Manage emails (coming soon)
  help      Show help message
```

### Validate password strength

`proton signup validate` reads the password from `account.yaml` and
reports on it with the same shape as Proton's own signup analyser
(score buckets `Vulnerable` / `Weak` / `Strong`, plus penalty reasons
like `NoUppercase`, `ContainsCommonPassword`, etc.). Exit code is `0`
when the password is `Strong`, otherwise `1`.

```bash
$ proton signup validate
❌ Password strength: Vulnerable (8 chars)
Issues:
  • missing uppercase letter
  • missing number
  • missing symbol
  • appears on a well-known common-passwords list

$ proton signup validate --json
{
  "score": "Vulnerable",
  "penalties": ["NoUppercase", "NoNumbers", "NoSymbols", "ContainsCommonPassword"],
  "length": 8
}
```

The interactive `signup fill` flow also runs this check before copying
the password to the clipboard, and asks for confirmation when the
score is `Vulnerable`.

### Check if a username is available

Check one or many usernames in a single call. Requests run concurrently
(cap 5 in flight) and output preserves input order.

```bash
$ proton signup check LucianoJr
❌ LucianoJr@proton.me      (suggestions: LucianoJr7, LucianoJr6, LucianoJr8)

$ proton signup check LucianoJr LucianoJr7 marcosnsc
❌ LucianoJr@proton.me      (suggestions: LucianoJr8, LucianoJr5, LucianoJr0)
✅ LucianoJr7@proton.me
❌ marcosnsc@proton.me      (suggestions: marcosnsc7, marcosnsc0, marcosnsc6)
```

Exit code is `0` if at least one name was available, otherwise `1` —
handy for scripts:

```bash
if proton signup check luciano lucianojr; then
  echo "pick one!"
fi
```

Machine-readable output with `--json`:

```bash
$ proton signup check --json LucianoJr LucianoJr7
[
  {
    "username": "LucianoJr",
    "available": false,
    "code": 12106,
    "suggestions": ["LucianoJr8", "LucianoJr5", "LucianoJr0"]
  },
  {
    "username": "LucianoJr7",
    "available": true,
    "code": 1000
  }
]
```

### Generate a signup config

```bash
$ proton signup init
✅ Created account.yaml — edit it with your details.
```

This creates an `account.yaml` template:

```yaml
plan: "Free"              # Free | Mail Plus | Proton Unlimited | Proton Family
username: "LucianoJr"     # Will become <username>@proton.me
password: ""              # Min 8 chars
recovery:
  recovery_email: ""      # Optional backup email
  recovery_phone: ""      # Optional phone number
```

### Interactive signup fill

```bash
$ proton signup fill
═══════════════════════════════════════
  Proton Account Signup Helper
═══════════════════════════════════════

Open: https://account.proton.me/mail/signup

── Step 1: Plan ──
📋 Select plan: Free

── Step 2: Credentials ──
📋 Username → copied to clipboard
📋 Password → copied to clipboard

── Step 3: Recovery ──
📋 Recovery Email → copied to clipboard

✅ Done! Complete any CAPTCHA/verification manually.
```

Or copy a single field:

```bash
$ proton signup fill username
📋 username → copied to clipboard
```

### Sign in (SRP + keychain)

```bash
$ proton signin
Proton username or email: luciano
Password: (hidden)
✅ SRP handshake succeeded
   User ID:      xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   Scope:        self mail ...
```

The password is used only to compute a local SRP proof; the plaintext
never leaves your machine. On success the resulting session bundle
(UID + access token + refresh token + scope) is written to the OS
keychain under the service name `proton-cli`. Background token
rotations from the Proton API are picked up by an `AuthHandler` and
re-persisted automatically, so the saved bundle never goes stale.

Inspect the saved session without exposing the tokens:

```bash
$ proton signin --status
Saved session for luciano@proton.me
  UID:         xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  Scope:       self mail
  Saved at:    2026-04-19T15:04:05Z
```

Wipe it (idempotent — safe to run twice):

```bash
$ proton signin --logout
Signed out luciano@proton.me.
```

Both `--status` and `--logout` default to the `username` in
`account.yaml`; use `--user <name>` to target a different account.

TOTP 2FA and two-password mailbox unlock are the next tasks in Phase 2.
Accounts with 2FA enabled still complete the SRP handshake, but the
current CLI cannot finish the login — use a Proton web client for now.

## Project Structure

```
proton/
├── main.go                       # CLI entry point + global flag parsing
├── cmd/
│   ├── signup/                   # ✅ Account creation helper
│   │   ├── signup.go             #    Subcommand router
│   │   ├── check.go              #    Username availability check (batch, --json)
│   │   ├── fill.go               #    Interactive clipboard form filler
│   │   ├── init.go               #    YAML config template generator
│   │   └── validate.go           #    Password strength validator
│   ├── signin/                   # ✅ SRP sign-in + keychain-backed sessions
│   │   ├── signin.go             #    SRP handshake + AuthHandler wiring
│   │   └── status_test.go        #    --status / --logout coverage
│   └── mail/                     # 🚧 Email management (placeholder)
│       └── mail.go
├── internal/
│   ├── keychain/                 # ✅ OS keychain wrapper (go-keyring, JSON bundle)
│   │   ├── keychain.go
│   │   └── keychain_test.go
│   └── log/                      # slog-based verbose/debug logger
│       └── log.go
├── docs/
│   ├── AGENT.md                  # Contract for AI agents working on this repo
│   └── FUTURE_DAEMON.md          # Design sketch for the future headless daemon
├── go.mod
├── go.sum
└── .gitignore
```

## Roadmap

### Phase 1 — Signup Helper ✅
- [x] Username availability checker via Proton public API
- [x] YAML config template generation
- [x] Interactive clipboard form filler
- [x] Cross-platform clipboard (macOS `pbcopy`, Windows `clip`, WSL `clip.exe`, Linux Wayland `wl-copy`, Linux X11 `xclip`/`xsel`)
- [x] Batch username check (multiple names at once, `--json` output)
- [ ] Username variation generator (`--generate` / `suggest` subcommand)
- [x] Password strength validator (Proton-shaped score + penalties, common-password blocklist)

### Phase 2 — Authentication
- [x] SRP authentication using [`go-proton-api`](https://github.com/ProtonMail/go-proton-api) and [`go-srp`](https://github.com/ProtonMail/go-srp)
- [x] Session persistence with OS keychain integration (macOS Keychain, Linux Secret Service, Windows Credential Manager)
- [x] `proton signin` / `proton signin --status` / `proton signin --logout`
- [x] `AuthHandler`-driven auto-refresh so rotated tokens are re-persisted
- [ ] TOTP two-factor authentication support
- [ ] Two-password mailbox unlock (encrypted-key accounts)
- [ ] Encrypted-file fallback for headless environments without a keyring
- [ ] Optional `proton-agent` daemon for headless boxes (see `docs/FUTURE_DAEMON.md`)

### Phase 3 — Mail
- [ ] Fetch inbox with message list (sender, subject, date, read/unread)
- [ ] Read individual messages (decrypt with user's private key via OpenPGP)
- [ ] Reply to messages
- [ ] Compose and send new emails
- [ ] Search by sender, subject, date range
- [ ] Label/folder management
- [ ] Attachment download/upload

### Phase 4 — Extended Features
- [ ] Contacts management (encrypted address book)
- [ ] Calendar integration
- [ ] Drive file listing and download
- [ ] Interactive TUI mode (using [Bubble Tea](https://github.com/charmbracelet/bubbletea))
- [ ] Shell completions (bash, zsh, fish)
- [ ] JSON output mode for scripting (`--json`)

## Technical Notes

### Proton API & Encryption
Proton Mail uses **end-to-end encryption**. Reading and sending emails requires:
1. **SRP Authentication** — Proton uses the [Secure Remote Password](https://en.wikipedia.org/wiki/Secure_Remote_Password_protocol) protocol. No plaintext password ever leaves the client.
2. **OpenPGP Decryption** — Messages are encrypted with the user's public key. The private key is encrypted with the user's mailbox password and stored on Proton's servers. Decryption happens client-side.
3. **Session Tokens** — After SRP auth, the API issues access/refresh tokens for subsequent requests. `proton-cli` stores those tokens in the OS keychain and re-persists them on every `AuthHandler` refresh event, so the CLI stays signed in across invocations without ever writing tokens to disk.

### Key Dependencies
| Library | Status | Purpose |
|---------|--------|---------|
| [`ProtonMail/go-proton-api`](https://github.com/ProtonMail/go-proton-api) | ✅ in use | Proton API client (auth, messages, contacts) |
| [`ProtonMail/go-srp`](https://github.com/ProtonMail/go-srp) | ✅ in use (via `go-proton-api`) | SRP authentication protocol |
| [`zalando/go-keyring`](https://github.com/zalando/go-keyring) | ✅ in use | Cross-platform OS keychain access |
| [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) | ✅ in use | Hidden password input on TTY |
| [`gopkg.in/yaml.v3`](https://github.com/go-yaml/yaml) | ✅ in use | YAML config parsing |
| [`ProtonMail/go-crypto`](https://github.com/ProtonMail/go-crypto) | 🚧 planned | OpenPGP encryption/decryption (needed for `mail`) |

### Related Projects
- [Proton Bridge](https://github.com/ProtonMail/proton-bridge) — Official IMAP/SMTP bridge (reference for API usage)
- [Proton API Bridge](https://github.com/henrybear327/Proton-API-Bridge) — Third-party API bridge library
- [Hydroxide](https://github.com/emersion/hydroxide) — Third-party IMAP/SMTP/CardDAV bridge

## Security

- **Passwords are never transmitted.** Sign-in uses SRP-6a; only a local
  proof crosses the network.
- **Password buffer is wiped after use** (`defer wipe(password)`) so the
  plaintext does not linger in the `[]byte` handed to the SRP client.
- **No credentials in plaintext files.** `account.yaml` is gitignored and
  only holds the *username* + optional recovery hints for the signup
  helper. The password field is only read at signup time and never
  persisted by the CLI.
- **Session tokens live in the OS keychain**, not on disk. One JSON
  bundle per user under service `proton-cli`:
  - macOS → Keychain Services
  - Windows → Credential Manager (DPAPI)
  - Linux/BSD → Secret Service via D-Bus (gnome-keyring / KWallet)
- **`--status` never prints tokens.** Only the UID, scope, and save
  time are shown; the access / refresh strings stay in the keychain.
- **Planned:** all email decryption will happen locally — private keys
  never leave the device.

## Contributing

This project is in early stages. Issues and PRs welcome.

## License

MIT
