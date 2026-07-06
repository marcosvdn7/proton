package signin

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"proton/internal/keychain"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/go-proton-api/server"
)

// memPersister is an in-memory SessionPersister so tests never touch the
// developer's OS keychain. Concurrency-safe because AuthHandler may fire
// from a goroutine inside go-proton-api.
type memPersister struct {
	mu       sync.Mutex
	sessions map[string]keychain.Session
	saveErr  error // if non-nil, Save returns it (for failure-path tests)
	saveCall int
}

func (m *memPersister) Save(username string, s keychain.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCall++
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.sessions == nil {
		m.sessions = map[string]keychain.Session{}
	}
	m.sessions[username] = s
	return nil
}

func (m *memPersister) get(username string) (keychain.Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[username]
	return s, ok
}

// fakePrompt is a canned PromptReader for tests.
type fakePrompt struct {
	username    string
	password    string
	usernameErr error
	passwordErr error
}

func (f *fakePrompt) ReadUsername(string) (string, error) {
	return f.username, f.usernameErr
}

func (f *fakePrompt) ReadPassword(string) ([]byte, error) {
	if f.passwordErr != nil {
		return nil, f.passwordErr
	}
	return []byte(f.password), nil
}

// discardOut is a plain io.Writer sink used across the sign-in tests to keep
// the fmt.Fprint output out of go test's log stream.
func discardOut(t *testing.T) io.Writer {
	t.Helper()
	return io.Discard
}

// newTestServer boots the in-memory Proton fake and returns SigninOptions
// pointing at it with self-signed TLS bypass. Callers can override fields as
// needed.
func newTestServer(t *testing.T) (*server.Server, SigninOptions, *memPersister) {
	t.Helper()
	// Plain HTTP — the fake's self-signed TLS costs ~30s per handshake on
	// some machines and buys us nothing in unit tests.
	s := server.New(server.WithTLS(false))
	t.Cleanup(s.Close)
	p := &memPersister{}
	return s, SigninOptions{
		HostURL:   s.GetHostURL(),
		Timeout:   30 * time.Second,
		Persister: p,
	}, p
}

func TestSignIn_SuccessfulHandshake(t *testing.T) {
	s, opts, persister := newTestServer(t)

	if _, _, err := s.CreateUser("user", []byte("pass")); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	res, err := signInWith(
		opts,
		&fakePrompt{username: "user", password: "pass"},
		discardOut(t),
	)
	if err != nil {
		t.Fatalf("signInWith: %v", err)
	}
	if res.UserID == "" {
		t.Errorf("UserID should be set: %+v", res)
	}
	if res.UID == "" {
		t.Errorf("UID (session id) should be set")
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Errorf("tokens should be set")
	}
	if res.Requires2FA {
		t.Errorf("plain account should not require 2FA")
	}

	// Persistence contract: exactly one Save on the happy path, and the
	// bundle must match what the API returned.
	if persister.saveCall != 1 {
		t.Errorf("expected exactly one Save call, got %d", persister.saveCall)
	}
	saved, ok := persister.get("user")
	if !ok {
		t.Fatal("expected session to be persisted for user")
	}
	if saved.UID != res.UID || saved.AccessToken != res.AccessToken || saved.RefreshToken != res.RefreshToken {
		t.Errorf("persisted session mismatch: got %+v want UID=%s access=%s refresh=%s",
			saved, res.UID, res.AccessToken, res.RefreshToken)
	}
}

func TestSignIn_PersisterFailureDoesNotAbortSignIn(t *testing.T) {
	s, opts, _ := newTestServer(t)
	opts.Persister = &memPersister{saveErr: errors.New("boom")}

	if _, _, err := s.CreateUser("user", []byte("pass")); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// A keychain that refuses to save is a warning-level event, not a
	// failure — the SRP handshake already succeeded and the process has
	// valid tokens in memory. Users on WSL / headless boxes hit this and
	// still want the sign-in to "work" for the current shell.
	res, err := signInWith(
		opts,
		&fakePrompt{username: "user", password: "pass"},
		discardOut(t),
	)
	if err != nil {
		t.Fatalf("signInWith should tolerate persister failure, got %v", err)
	}
	if res == nil || res.UID == "" {
		t.Error("expected valid result even when persister fails")
	}
}

func TestSignIn_WrongPasswordFails(t *testing.T) {
	s, opts, _ := newTestServer(t)

	if _, _, err := s.CreateUser("user", []byte("correct-pass")); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := signInWith(
		opts,
		&fakePrompt{username: "user", password: "wrong-pass"},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected sign-in with wrong password to fail")
	}
}

func TestSignIn_UnknownUsernameFails(t *testing.T) {
	_, opts, _ := newTestServer(t)

	_, err := signInWith(
		opts,
		&fakePrompt{username: "nobody", password: "pass"},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected sign-in with unknown user to fail")
	}
}

func TestSignIn_EmptyUsernameRejected(t *testing.T) {
	_, err := signInWith(
		SigninOptions{},
		&fakePrompt{username: "   ", password: "pass"},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected empty username to be rejected before touching the network")
	}
}

func TestSignIn_EmptyPasswordRejected(t *testing.T) {
	_, err := signInWith(
		SigninOptions{},
		&fakePrompt{username: "user", password: ""},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected empty password to be rejected before touching the network")
	}
}

func TestSignIn_UsernameReadError(t *testing.T) {
	_, err := signInWith(
		SigninOptions{},
		&fakePrompt{usernameErr: io.ErrUnexpectedEOF},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected username read error to propagate")
	}
}

func TestSignIn_PasswordReadError(t *testing.T) {
	_, err := signInWith(
		SigninOptions{},
		&fakePrompt{username: "user", passwordErr: io.ErrUnexpectedEOF},
		discardOut(t),
	)
	if err == nil {
		t.Fatal("expected password read error to propagate")
	}
}

func TestTranslateAuthError_InvalidProofIsFriendly(t *testing.T) {
	got := translateAuthError(proton.ErrInvalidProof)
	if got == nil || !containsCI(got.Error(), "server proof") {
		t.Errorf("expected friendly server-proof message, got %v", got)
	}
}

func TestTranslateAuthError_GenericErrorWrapped(t *testing.T) {
	base := errors.New("some http error")
	got := translateAuthError(base)
	if !errors.Is(got, base) {
		t.Errorf("expected wrapped error to unwrap to the original, got %v", got)
	}
}

// containsCI is a tiny helper — imports of strings are avoided to keep this
// file's test-only imports focused.
func containsCI(s, sub string) bool {
	// Both server-proof strings are ASCII, cheap lower isn't needed; use a
	// direct scan. Kept as a helper so intent is obvious at call sites.
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
