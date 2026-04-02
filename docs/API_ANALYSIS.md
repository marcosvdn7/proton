# Proton API Deep Analysis

> Full evaluation of what can be built using Proton's public Go libraries.
>
> Date: 2026-04-02

## Executive Summary

Proton publishes **official MIT-licensed Go libraries** that expose a rich API surface covering Mail, Contacts, Calendar, Drive, and account management. The entire authentication flow (SRP + 2FA), end-to-end encryption (OpenPGP), and real-time event streaming are fully supported. This means a **full-featured CLI client is technically feasible** — not just a helper tool, but a real mail client.

### Key Libraries

| Library | License | Purpose |
|---------|---------|---------|
| [`ProtonMail/go-proton-api`](https://github.com/ProtonMail/go-proton-api) | MIT | Complete API client (auth, mail, contacts, calendar, drive, events) |
| [`ProtonMail/go-srp`](https://github.com/ProtonMail/go-srp) | MIT | SRP authentication protocol |
| [`ProtonMail/go-crypto`](https://github.com/ProtonMail/go-crypto) | BSD-3 | OpenPGP encryption/decryption (fork of x/crypto) |
| [`ProtonMail/gopenpgp`](https://github.com/ProtonMail/gopenpgp) | MIT | High-level PGP operations |
| [`ProtonMail/gluon`](https://github.com/ProtonMail/gluon) | MIT | IMAP server implementation |

### Proof of Feasibility

These aren't theoretical APIs — they power real production software:
- **[Proton Bridge](https://github.com/ProtonMail/proton-bridge)** (GPL-3.0) — Official IMAP/SMTP bridge, uses `go-proton-api` for everything
- **[Proton-API-Bridge](https://github.com/henrybear327/Proton-API-Bridge)** (MIT) — Powers rclone's Proton Drive backend
- **[Hydroxide](https://github.com/emersion/hydroxide)** (MIT) — Third-party IMAP/SMTP/CardDAV bridge

---

## Complete API Surface

### 1. Authentication & Session Management

**What's available:**

```go
// Login with username/password (SRP protocol — password never sent to server)
m := proton.New()
client, auth, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))

// 2FA (TOTP and FIDO2)
if auth.TwoFA.Enabled & proton.HasTOTP != 0 {
    client.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: "123456"})
}

// Session refresh (for long-lived sessions)
client, auth, err := m.NewClientWithRefresh(ctx, savedUID, savedRefreshToken)

// Direct client creation (if tokens are already known)
client := m.NewClient(uid, accessToken, refreshToken)

// Auth event handler (fires on token refresh — save to keychain)
client.AddAuthHandler(func(auth proton.Auth) {
    saveToKeychain(auth.UID, auth.AccessToken, auth.RefreshToken)
})

// Session management
sessions, _ := client.AuthSessions(ctx)          // List all sessions
client.AuthRevoke(ctx, sessionUID)                // Revoke specific session
client.AuthRevokeAll(ctx)                         // Revoke all sessions
client.AuthDelete(ctx)                            // Delete current session
```

**What we can build:**
- ✅ Full login flow with SRP (zero-knowledge password proof)
- ✅ TOTP 2FA support
- ✅ Session persistence with OS keychain
- ✅ Session management (list, revoke)
- ⚠️ FIDO2 — types exist but CLI would need USB/NFC hardware access
- ⚠️ Human verification (CAPTCHA) — `GetCaptcha()` exists but needs visual display

### 2. Key Management & Encryption

**What's available:**

```go
// Get user salts and compute salted key passphrase
salts, _ := client.GetSalts(ctx)
saltedKeyPass, _ := salts.SaltForKey(password, user.Keys.Primary().ID)

// Unlock user keyring + all address keyrings
userKR, addrKRs, err := proton.Unlock(user, addresses, saltedKeyPass, panicHandler)

// Decrypt a message
plaintext, err := message.Decrypt(addrKRs[message.AddressID])

// Get public keys for any email (for sending encrypted mail)
pubKeys, recipientType, _ := client.GetPublicKeys(ctx, "someone@proton.me")
```

**What we can build:**
- ✅ Full keyring unlock (user keys + per-address keys)
- ✅ Message decryption/encryption
- ✅ Public key lookup for any recipient
- ✅ Key management (create, delete, set primary)
- ❌ Private key generation (would need to implement the full key setup flow)

### 3. Mail — Messages

**What's available:**

```go
// Count messages
count, _ := client.CountMessages(ctx)
groupCounts, _ := client.GetGroupedMessageCount(ctx)  // Count per label/folder

// List messages (with filtering)
metadata, _ := client.GetMessageMetadata(ctx, proton.MessageFilter{
    LabelID: proton.InboxLabel,
    Desc:    true,
})

// Paginated listing
page, _ := client.GetMessageMetadataPage(ctx, 0, 50, proton.MessageFilter{
    LabelID: proton.InboxLabel,
})

// Get single message (encrypted body)
msg, _ := client.GetMessage(ctx, messageID)
plaintext, _ := msg.Decrypt(addrKR)  // Decrypt body

// Get full message with attachments
fullMsg, _ := client.GetFullMessage(ctx, messageID, scheduler, allocator)

// Get all message IDs (for sync)
allIDs, _ := client.GetAllMessageIDs(ctx, "")

// Mark read/unread
client.MarkMessagesRead(ctx, "id1", "id2")
client.MarkMessagesUnread(ctx, "id1")

// Label/unlabel (move to folder, add label)
client.LabelMessages(ctx, []string{"id1"}, proton.TrashLabel)
client.UnlabelMessages(ctx, []string{"id1"}, proton.InboxLabel)

// Delete permanently
client.DeleteMessage(ctx, "id1", "id2")
```

**MessageMetadata fields:** ID, AddressID, LabelIDs, Subject, Sender, ToList, CCList, BCCList, ReplyTos, Flags, Time, Size, Unread, IsReplied, IsRepliedAll, IsForwarded, NumAttachments

**What we can build:**
- ✅ Full inbox listing with pagination and filtering
- ✅ Read messages with decrypted body
- ✅ Mark read/unread
- ✅ Move to trash/spam/archive
- ✅ Label management
- ✅ Delete messages
- ✅ Message sync (via IDs + events)
- ✅ Unread counts per folder/label

### 4. Mail — Sending

**What's available:**

```go
// Create a draft (body is auto-encrypted)
draft, _ := client.CreateDraft(ctx, addrKR, proton.CreateDraftReq{
    Message: proton.DraftTemplate{
        Subject:  "Hello",
        Sender:   &mail.Address{Name: "Me", Address: "me@proton.me"},
        ToList:   []*mail.Address{{Name: "You", Address: "you@example.com"}},
        Body:     "<p>Hello world</p>",
        MIMEType: "text/html",
    },
})

// Upload attachment (encrypted)
att, _ := client.UploadAttachment(ctx, addrKR, proton.CreateAttachmentReq{
    MessageID:   draft.ID,
    Filename:    "doc.pdf",
    MIMEType:    "application/pdf",
    Disposition: proton.AttachmentDisposition,
    Body:        fileBytes,
})

// Send the draft (with per-recipient encryption packages)
req := proton.SendDraftReq{}
req.AddTextPackage(kr, body, mimeType, recipientPrefs, attKeys)
sent, _ := client.SendDraft(ctx, draft.ID, req)

// Undo send (if within undo window)
client.UndoActions(ctx, undoToken)
```

**Encryption schemes per recipient:**
- `InternalScheme` — Proton-to-Proton (encrypted with recipient's public key)
- `PGPMIMEScheme` — PGP/MIME for external PGP users
- `PGPInlineScheme` — PGP/Inline for legacy PGP
- `ClearScheme` — Unencrypted to external non-PGP recipients
- `ClearMIMEScheme` — Signed but unencrypted MIME
- `EncryptedOutsideScheme` — Password-protected emails to external users

**What we can build:**
- ✅ Compose and send emails
- ✅ Reply / Reply All / Forward
- ✅ Attachments (upload + encrypt)
- ✅ Per-recipient encryption (auto-detect Proton vs external)
- ✅ Undo send
- ⚠️ Complex — need to build the SendPreferences logic per recipient

### 5. Mail — Import

**What's available:**

```go
// Bulk import messages (e.g., from mbox/eml files)
stream, _ := client.ImportMessages(ctx, addrKR, workers, buffer,
    proton.ImportReq{
        AddressID: addrID,
        LabelIDs:  []string{proton.InboxLabel},
        Message:   emlBytes,
    },
)
```

**What we can build:**
- ✅ Import emails from .eml files
- ✅ Migrate from other email providers
- ✅ Parallel import with automatic chunking (max 10 per batch, 70MB limit)

### 6. Labels & Folders

**What's available:**

```go
// System labels (built-in)
proton.InboxLabel        // "0"
proton.AllDraftsLabel    // "1"
proton.AllSentLabel      // "2"
proton.TrashLabel        // "3"
proton.SpamLabel         // "4"
proton.AllMailLabel       // "5"
proton.ArchiveLabel      // "6"
proton.SentLabel         // "7"
proton.DraftsLabel       // "8"
proton.StarredLabel      // "10"

// Custom labels/folders
labels, _ := client.GetLabels(ctx, proton.LabelTypeLabel, proton.LabelTypeFolder)
newLabel, _ := client.CreateLabel(ctx, proton.CreateLabelReq{
    Name: "Important", Color: "#cf5858", Type: proton.LabelTypeLabel,
})
client.UpdateLabel(ctx, labelID, proton.UpdateLabelReq{Name: "Very Important"})
client.DeleteLabel(ctx, labelID)
```

**What we can build:**
- ✅ List all labels and folders
- ✅ Create/update/delete custom labels
- ✅ Create/update/delete folders (with nesting via ParentID)
- ✅ Apply labels to messages

### 7. Contacts

**What's available:**

```go
// List contacts
contacts, _ := client.GetAllContacts(ctx)
contact, _ := client.GetContact(ctx, contactID)
emails, _ := client.GetAllContactEmails(ctx, "someone@example.com")

// Count
count, _ := client.CountContacts(ctx)

// Create contacts (with encrypted vCards)
results, _ := client.CreateContacts(ctx, proton.CreateContactsReq{
    Contacts: []proton.ContactCards{...},
    Overwrite: 1,
})

// Update / Delete
client.UpdateContact(ctx, contactID, req)
client.DeleteContacts(ctx, proton.DeleteContactsReq{IDs: []string{"id1", "id2"}})

// Contact cards support encrypted vCard fields
// Types: CardPlainText (0), CardEncryptedAndSigned (3), CardSigned (2)
```

**What we can build:**
- ✅ List/search contacts
- ✅ Create/update/delete contacts
- ✅ Encrypted contact cards (vCard with PGP)
- ✅ Contact email lookup
- ✅ Contact groups

### 8. Calendar

**What's available:**

```go
calendars, _ := client.GetCalendars(ctx)
events, _ := client.GetAllCalendarEvents(ctx, calendarID, filter)
event, _ := client.GetCalendarEvent(ctx, calendarID, eventID)
count, _ := client.CountCalendarEvents(ctx, calendarID)

// Key management for calendar encryption
keys, _ := client.GetCalendarKeys(ctx, calendarID)
passphrase, _ := client.GetCalendarPassphrase(ctx, calendarID)
members, _ := client.GetCalendarMembers(ctx, calendarID)
```

**What we can build:**
- ✅ List calendars and events
- ✅ Read event details (decrypt with calendar keys)
- ⚠️ Read-only — no create/update/delete event endpoints exposed yet
- ⚠️ Calendar events are encrypted with separate key hierarchy

### 9. Drive

**What's available:**

```go
// Volumes and shares
volumes, _ := client.ListVolumes(ctx)
shares, _ := client.ListShares(ctx, true)

// Browse files
children, _ := client.ListChildren(ctx, shareID, folderLinkID, true)
link, _ := client.GetLink(ctx, shareID, linkID)

// File operations
client.CreateFile(ctx, shareID, req)
client.CreateFolder(ctx, shareID, req)

// Download (block-based)
revision, _ := client.GetRevision(ctx, shareID, linkID, revisionID, 1, 50)
blockData, _ := client.GetBlock(ctx, block.BareURL, block.Token)

// Upload (block-based)
uploadLinks, _ := client.RequestBlockUpload(ctx, req)
client.UploadBlock(ctx, url, token, blockStream)

// Delete
client.TrashChildren(ctx, shareID, parentID, childIDs...)
client.DeleteChildren(ctx, shareID, parentID, childIDs...)
```

**What we can build:**
- ✅ List volumes, shares, folders, files
- ✅ Download files (block-based with decryption)
- ✅ Upload files (block-based with encryption)
- ✅ Create folders
- ✅ Trash / delete files
- ⚠️ Complex — encryption per-file, block-based chunking, need `Proton-API-Bridge` as reference

### 10. Real-Time Events

**What's available:**

```go
// Poll-based event streaming
eventCh := client.NewEventStream(ctx, 20*time.Second, 20*time.Second, lastEventID)
for event := range eventCh {
    // event.Messages  — message changes (create/update/delete)
    // event.Labels    — label changes
    // event.Addresses — address changes
    // event.User      — user changes
    // event.MailSettings — settings changes
    // event.Refresh   — full refresh needed
}

// Or manual polling
events, more, _ := client.GetEvent(ctx, lastEventID)
```

**Event types contain:**
- Message events (new, updated, deleted messages)
- Label events (new, updated, deleted labels)
- Address events
- User updates
- Mail settings changes
- Drive events (per-volume and per-share)

**What we can build:**
- ✅ Real-time notification of new messages
- ✅ Background sync (keep local state in sync)
- ✅ Push-style notifications via polling
- ✅ Drive change monitoring

### 11. Mail Settings

**What's available:**

```go
settings, _ := client.GetMailSettings(ctx)
// DisplayName, Signature, DraftMIMEType, AttachPublicKey, Sign, PGPScheme

client.SetDisplayName(ctx, req)
client.SetSignature(ctx, req)
client.SetDraftMIMEType(ctx, req)
client.SetAttachPublicKey(ctx, req)
client.SetSignExternalMessages(ctx, req)
client.SetDefaultPGPScheme(ctx, req)
```

**What we can build:**
- ✅ Read/update display name and signature
- ✅ Configure PGP behavior (scheme, sign external, attach public key)
- ✅ Set default MIME type for drafts

### 12. Account Management (Unauthenticated)

**What's available:**

```go
m := proton.New()
m.GetUsernameAvailable(ctx, "username")                    // Check availability
m.SendVerificationCode(ctx, req)                            // Email/SMS verification
m.GetCaptcha(ctx, token)                                    // Get CAPTCHA image
m.CreateUser(ctx, proton.CreateUserReq{...})               // Create account
m.GetDomains(ctx)                                           // List available domains
m.Ping(ctx)                                                 // Health check
```

**What we can build:**
- ✅ Username availability check (already implemented)
- ✅ Domain listing (proton.me, protonmail.com, etc.)
- ✅ Health/connectivity check
- ⚠️ Account creation — possible but requires CAPTCHA solving + verification flow

---

## Feasibility Assessment

### What's Realistic to Build

| Feature | Difficulty | Dependencies | Priority |
|---------|-----------|--------------|----------|
| **Auth (login + 2FA + session)** | Medium | go-proton-api, go-srp, go-keyring | 🔴 Critical |
| **Read inbox** | Medium | Auth + key unlock + decrypt | 🔴 Critical |
| **Read single message** | Medium | Auth + decrypt | 🔴 Critical |
| **Send email** | Hard | Auth + encrypt + per-recipient logic | 🟡 High |
| **Reply/forward** | Hard | Send + draft creation | 🟡 High |
| **Search messages** | Easy | Auth + MessageFilter | 🟡 High |
| **Label/move/delete** | Easy | Auth only | 🟢 Medium |
| **Contacts CRUD** | Medium | Auth + vCard encryption | 🟢 Medium |
| **Attachments** | Medium | Auth + decrypt/encrypt | 🟡 High |
| **Event streaming** | Easy | Auth only | 🟢 Medium |
| **Drive listing** | Medium | Auth + share/link decryption | 🔵 Low |
| **Drive download** | Hard | Auth + block decryption | 🔵 Low |
| **Drive upload** | Very Hard | Full crypto chain | 🔵 Low |
| **Calendar read** | Medium | Auth + calendar key hierarchy | 🔵 Low |
| **Mail import (eml)** | Medium | Auth + encrypt | 🟢 Medium |
| **Account creation** | Very Hard | CAPTCHA + verification + key gen | 🔵 Low |
| **TUI (Bubble Tea)** | Medium | All of the above | 🟢 Medium |

### Key Complexity: The Encryption Chain

The main challenge isn't the API calls — it's the **encryption pipeline**:

```
Login (SRP)
  → Get salts
    → Compute salted key passphrase
      → Unlock user keyring
        → Unlock per-address keyrings
          → Decrypt messages with address keyring
          → Encrypt outgoing with recipient's public key
```

Every message read/send requires this chain. However, `go-proton-api` provides all the primitives — `Unlock()`, `Message.Decrypt()`, `CreateDraft()` (auto-encrypts), etc.

### What Cannot Be Built (Limitations)

1. **Calendar write operations** — No create/update/delete event endpoints in `go-proton-api`
2. **Proton Pass** — No API exposed in the Go library
3. **Proton VPN** — Separate protocol, not in scope
4. **Proton Wallet** — No API exposed
5. **Admin panel features** — Organization management is minimal
6. **Web-only features** — Some UI-specific flows (e.g., key recovery via admin) have no API

---

## Recommended Architecture

```
proton (CLI)
├── internal/
│   ├── auth/           # SRP login, 2FA, session management, keychain
│   ├── crypto/         # Key unlock, encrypt/decrypt wrappers
│   ├── mail/           # Message operations (list, read, send, search)
│   ├── contacts/       # Contact CRUD
│   ├── drive/          # Drive operations (future)
│   ├── events/         # Event streaming / sync
│   └── config/         # YAML config, state management
├── cmd/
│   ├── auth/           # proton auth login/logout/status
│   ├── mail/           # proton mail list/read/send/reply/search
│   ├── contacts/       # proton contacts list/add/delete
│   ├── labels/         # proton labels list/create/delete
│   ├── drive/          # proton drive ls/get/put (future)
│   └── signup/         # proton signup check/init/fill (existing)
└── main.go
```

### Recommended Implementation Order

**Phase 1: Auth foundation**
```
proton auth login              # SRP + optional 2FA → store in keychain
proton auth status             # Show current session info
proton auth logout             # Revoke session + clear keychain
proton auth sessions           # List all active sessions
```

**Phase 2: Read mail**
```
proton mail list               # Inbox listing (subject, sender, date, unread)
proton mail list --folder sent # Other folders
proton mail list --unread      # Unread only
proton mail read <id>          # Decrypt and display message body
proton mail read <id> --raw    # Show raw encrypted message
proton mail count              # Unread counts per folder
```

**Phase 3: Write mail**
```
proton mail send --to x --subject y --body z   # Compose and send
proton mail send --to x --subject y --file z   # Send with attachment
proton mail reply <id>                          # Reply to message
proton mail forward <id> --to x                # Forward message
proton mail move <id> --to trash                # Move to folder
proton mail label <id> --add "Important"        # Add label
proton mail delete <id>                         # Permanent delete
```

**Phase 4: Contacts & Labels**
```
proton contacts list           # List all contacts
proton contacts search <query> # Search contacts
proton contacts add            # Add new contact
proton labels list             # List labels and folders
proton labels create           # Create label/folder
```

**Phase 5: Interactive TUI**
```
proton tui                     # Full interactive terminal UI
```

---

## Security Considerations

1. **Never store passwords** — Use SRP tokens + keychain only
2. **Keyring in memory only** — Unlock on each session, never persist private keys
3. **Session tokens in OS keychain** — Use `zalando/go-keyring` for macOS Keychain, Linux secret-service, Windows Credential Manager
4. **Respect rate limits** — `go-proton-api` handles 429 with exponential backoff
5. **Verify server proofs** — SRP mutual authentication (enabled by default)
6. **Handle token refresh** — `AddAuthHandler` callback to update stored tokens

---

## Dependencies Cost

Adding `go-proton-api` pulls in significant dependencies:

```
github.com/ProtonMail/go-proton-api    → API client
github.com/ProtonMail/go-srp           → SRP auth
github.com/ProtonMail/gopenpgp/v2      → PGP operations
github.com/ProtonMail/go-crypto        → Crypto primitives
github.com/ProtonMail/gluon           → RFC822 parsing
github.com/go-resty/resty/v2          → HTTP client
github.com/bradenaw/juniper           → Parallel utilities
```

Binary size estimate: ~15-20MB (due to crypto libraries).

---

## Conclusion

**The official `go-proton-api` library provides everything needed to build a full-featured Proton Mail CLI client.** The API surface is comprehensive, well-structured, and battle-tested (it powers Proton Bridge). The main engineering challenge is implementing the encryption pipeline correctly, but all cryptographic primitives are provided.

The existing `proton signup` commands remain useful as standalone utilities. The next logical step is implementing authentication (`Phase 1`), which unlocks all other features.
