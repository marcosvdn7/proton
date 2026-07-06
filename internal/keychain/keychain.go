// Package keychain persists Proton SRP session tokens in the OS keychain.
//
// Backend: github.com/zalando/go-keyring. That library dispatches to
// Keychain Services (macOS), Credential Manager (Windows) or the Secret
// Service D-Bus API (Linux/BSD). See docs/FUTURE_DAEMON.md for the story on
// headless environments where none of those are available.
//
// Layout: one keyring entry per Proton account.
//
//	service = "proton-cli"
//	user    = <proton username>
//	value   = JSON-encoded Session
//
// Rationale for the bundle-per-user layout (not one entry per field):
// go-proton-api emits a single AuthHandler event when it refreshes tokens,
// carrying UID/access/refresh together. One entry = one atomic write per
// refresh, no partial-update window where UID and access diverge.
package keychain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

// Service is the keyring service name used for every entry this package
// stores. Exposed so tests and CLI plumbing can reference the same constant
// instead of stringly duplicating "proton-cli" in multiple places.
const Service = "proton-cli"

// ErrNotFound is returned by Load when there is no saved session for the
// given username. Callers should treat this as "user needs to sign in",
// not as a hard failure.
var ErrNotFound = errors.New("keychain: no saved session for user")

// Session is the shape persisted per user. It is intentionally a superset
// of what proton.Auth returns so future refresh events can round-trip
// without a schema migration.
//
// Do NOT log this struct verbatim. AccessToken and RefreshToken are
// bearer credentials.
type Session struct {
	// UID is the Proton session identifier. Sent as the x-pm-uid header
	// on every subsequent API call. Not sensitive alone, but paired with
	// the tokens it is.
	UID string `json:"uid"`

	// AccessToken is the short-lived bearer token (~15 min). Sent as
	// Authorization: Bearer <token>.
	AccessToken string `json:"access_token"`

	// RefreshToken is the long-lived credential used to mint new access
	// tokens via /auth/v4/refresh. Loss = full account access until
	// server-side revocation.
	RefreshToken string `json:"refresh_token"`

	// Scope is the permission scope string returned by /auth/v4. Kept
	// for future feature gating.
	Scope string `json:"scope"`

	// SavedAt is when this bundle was written. Displayed by
	// `proton signin --status` so the user can eyeball freshness.
	SavedAt time.Time `json:"saved_at"`
}

// Save writes s to the keyring under (Service, username). Overwrites any
// existing entry — go-keyring's Set is defined as upsert, matching the
// AuthHandler refresh flow where every event replaces the previous bundle.
func Save(username string, s Session) error {
	if username == "" {
		return errors.New("keychain: username must not be empty")
	}
	if s.UID == "" || s.AccessToken == "" || s.RefreshToken == "" {
		// A session without a UID or without both tokens is unusable.
		// Fail loudly instead of persisting garbage that will confuse
		// the next Load.
		return errors.New("keychain: session missing UID, access token, or refresh token")
	}

	if s.SavedAt.IsZero() {
		s.SavedAt = time.Now().UTC()
	}

	blob, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("keychain: encode session: %w", err)
	}

	if err := keyring.Set(Service, username, string(blob)); err != nil {
		return fmt.Errorf("keychain: write session for %q: %w", username, err)
	}
	return nil
}

// Load reads the saved session for username. Returns ErrNotFound if there
// is nothing saved (fresh install, previous logout). Any other error means
// the keyring backend itself failed and the caller should not treat it as
// "user is not signed in".
func Load(username string) (Session, error) {
	if username == "" {
		return Session{}, errors.New("keychain: username must not be empty")
	}

	blob, err := keyring.Get(Service, username)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("keychain: read session for %q: %w", username, err)
	}

	var s Session
	if err := json.Unmarshal([]byte(blob), &s); err != nil {
		// A stored value we cannot parse is a bug or a schema change we
		// missed. Treat it as "no valid session" so the user is asked
		// to sign in again, but surface the parse error so we can see
		// it in --verbose logs.
		return Session{}, fmt.Errorf("keychain: decode session for %q: %w", username, err)
	}
	return s, nil
}

// Delete removes the saved session for username. It is not an error to
// delete a session that does not exist — logout should be idempotent so
// callers can run `signin --logout` twice without a scary message.
func Delete(username string) error {
	if username == "" {
		return errors.New("keychain: username must not be empty")
	}

	if err := keyring.Delete(Service, username); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("keychain: delete session for %q: %w", username, err)
	}
	return nil
}
