package signin

import (
	"errors"
	"io"
	"testing"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/go-proton-api/server"
)

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
func newTestServer(t *testing.T) (*server.Server, SigninOptions) {
	t.Helper()
	// Plain HTTP — the fake's self-signed TLS costs ~30s per handshake on
	// some machines and buys us nothing in unit tests.
	s := server.New(server.WithTLS(false))
	t.Cleanup(s.Close)
	return s, SigninOptions{
		HostURL: s.GetHostURL(),
		Timeout: 30 * time.Second,
	}
}

func TestSignIn_SuccessfulHandshake(t *testing.T) {
	s, opts := newTestServer(t)

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
}

func TestSignIn_WrongPasswordFails(t *testing.T) {
	s, opts := newTestServer(t)

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
	_, opts := newTestServer(t)

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
