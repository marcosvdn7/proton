# proton

A command-line interface for Proton Mail — manage your account, emails, and more from the terminal.

> **Status:** Early development. Signup helper is functional; authentication and mail features are in progress.

## Features

### ✅ Signup Helper
- **Check username availability** against the Proton API with alternative suggestions
- **Generate config template** (`account.yaml`) for signup details
- **Interactive form filler** — copies each field to your clipboard step-by-step while you fill the browser form

### 🚧 Coming Soon
- **Authentication** — SRP login works (`proton signin`); 2FA, session
  persistence, and keychain integration are next.
- **Mail** — Fetch, read, reply, send, and search emails from the terminal
- **Contacts** — Manage your encrypted address book

## Installation

### Prerequisites
- [Go](https://go.dev/dl/) 1.21+
- A clipboard tool for the interactive fill flow:
  - **macOS** — `pbcopy` (built-in)
  - **Windows** — `clip` (built-in)
  - **WSL** — `clip.exe` (built-in, reachable from Linux)
  - **Linux (Wayland)** — `wl-copy` from [`wl-clipboard`](https://github.com/bugaevc/wl-clipboard)
  - **Linux (X11)** — `xclip` or `xsel`

### Build from source

```bash
git clone https://github.com/iamlucianojr/proton.git
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
  signin    Sign in to your Proton account (SRP)
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
📌 Select plan: Free

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

### Sign in (SRP)

```bash
$ proton signin
Proton username or email: luciano
Password: (hidden)
✅ SRP handshake succeeded
   User ID:      xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   Scope:        self mail ...
```

The password is used only to compute a local SRP proof; the plaintext
never leaves your machine. This step verifies credentials and then
drops the session tokens — 2FA prompts, keychain persistence, and
long-lived sessions are the next tasks in Phase 2.

## Project Structure

```
proton/
├── main.go                 # CLI entry point and command router
├── cmd/
│   ├── signup/             # ✅ Account creation helper
│   │   ├── signup.go       #    Subcommand router
│   │   ├── check.go        #    Username availability check (Proton API)
│   │   ├── fill.go         #    Interactive clipboard form filler
│   │   └── init.go         #    YAML config template generator
│   ├── signin/             # 🚧 Authentication (placeholder)
│   │   └── signin.go
│   └── mail/               # 🚧 Email management (placeholder)
│       └── mail.go
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
- [ ] TOTP two-factor authentication support
- [ ] Session persistence with OS keychain integration (macOS Keychain, Linux `secret-service`, Windows Credential Manager)
- [ ] `proton signin` / `proton signin status` / `proton signin logout`
- [ ] Encrypted local config (never store passwords in plaintext)

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
3. **Session Tokens** — After SRP auth, the API issues access/refresh tokens for subsequent requests.

### Key Dependencies (planned)
| Library | Purpose |
|---------|---------|
| [`ProtonMail/go-proton-api`](https://github.com/ProtonMail/go-proton-api) | Official Proton API client (auth, messages, contacts) |
| [`ProtonMail/go-srp`](https://github.com/ProtonMail/go-srp) | SRP authentication protocol |
| [`ProtonMail/go-crypto`](https://github.com/ProtonMail/go-crypto) | OpenPGP encryption/decryption |
| [`gopkg.in/yaml.v3`](https://github.com/go-yaml/yaml) | YAML config parsing |
| [`zalando/go-keyring`](https://github.com/zalando/go-keyring) | Cross-platform OS keychain access |

### Related Projects
- [Proton Bridge](https://github.com/ProtonMail/proton-bridge) — Official IMAP/SMTP bridge (reference for API usage)
- [Proton API Bridge](https://github.com/henrybear327/Proton-API-Bridge) — Third-party API bridge library
- [Hydroxide](https://github.com/emersion/hydroxide) — Third-party IMAP/SMTP/CardDAV bridge

## Security

- **No credentials stored in plaintext.** The `account.yaml` is gitignored and only used for the signup helper flow.
- **Planned:** Session tokens will be stored in the OS keychain, not in files.
- **Planned:** All email decryption happens locally — private keys never leave the device.

## Contributing

This project is in early stages. Issues and PRs welcome.

## License

MIT
