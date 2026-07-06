package keychain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// resetKeyring swaps the OS backend for the in-memory mock. Every test that
// touches Save/Load/Delete must call this or it will try to hit the real
// keychain, which is nondeterministic in CI and pollutes the developer's
// login keyring.
func resetKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
}

func fixtureSession() Session {
	return Session{
		UID:          "uid-abc-123",
		AccessToken:  "acc-token-xyz",
		RefreshToken: "ref-token-xyz",
		Scope:        "self mail",
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	resetKeyring(t)

	in := fixtureSession()
	if err := Save("alice@proton.me", in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := Load("alice@proton.me")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if out.UID != in.UID || out.AccessToken != in.AccessToken || out.RefreshToken != in.RefreshToken || out.Scope != in.Scope {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}

	// SavedAt should have been populated by Save even though we did not
	// set it explicitly. Anything within the last minute is fine — this
	// test does not need clock injection.
	if out.SavedAt.IsZero() {
		t.Error("expected SavedAt to be populated by Save")
	}
	if time.Since(out.SavedAt) > time.Minute {
		t.Errorf("SavedAt looks wrong: %v", out.SavedAt)
	}
}

func TestSavePreservesExplicitSavedAt(t *testing.T) {
	resetKeyring(t)

	// If a caller already stamped SavedAt (e.g. the AuthHandler wanted
	// deterministic timestamps in tests), Save must not overwrite it.
	fixed := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	in := fixtureSession()
	in.SavedAt = fixed
	if err := Save("alice@proton.me", in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := Load("alice@proton.me")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !out.SavedAt.Equal(fixed) {
		t.Errorf("SavedAt overwritten: got %v want %v", out.SavedAt, fixed)
	}
}

func TestSaveOverwrites(t *testing.T) {
	resetKeyring(t)

	first := fixtureSession()
	if err := Save("alice@proton.me", first); err != nil {
		t.Fatal(err)
	}

	// Simulate an AuthHandler refresh event: same UID, new tokens.
	second := first
	second.AccessToken = "acc-token-refreshed"
	second.RefreshToken = "ref-token-refreshed"
	if err := Save("alice@proton.me", second); err != nil {
		t.Fatal(err)
	}

	out, err := Load("alice@proton.me")
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken != "acc-token-refreshed" || out.RefreshToken != "ref-token-refreshed" {
		t.Errorf("Save did not overwrite: got %+v", out)
	}
}

func TestLoadMissingReturnsErrNotFound(t *testing.T) {
	resetKeyring(t)

	_, err := Load("nobody@proton.me")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load: want ErrNotFound, got %v", err)
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	resetKeyring(t)

	if err := Save("alice@proton.me", fixtureSession()); err != nil {
		t.Fatal(err)
	}
	if err := Delete("alice@proton.me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Load("alice@proton.me"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after Delete, Load should return ErrNotFound, got %v", err)
	}
}

func TestDeleteMissingIsIdempotent(t *testing.T) {
	resetKeyring(t)

	// No prior Save. Delete must not error.
	if err := Delete("ghost@proton.me"); err != nil {
		t.Errorf("Delete on missing entry: want nil, got %v", err)
	}
}

func TestSaveRejectsEmptyUsername(t *testing.T) {
	resetKeyring(t)
	err := Save("", fixtureSession())
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Errorf("Save(\"\") should reject empty username, got %v", err)
	}
}

func TestSaveRejectsPartialSession(t *testing.T) {
	resetKeyring(t)

	cases := map[string]Session{
		"missing UID":     {AccessToken: "a", RefreshToken: "r"},
		"missing access":  {UID: "u", RefreshToken: "r"},
		"missing refresh": {UID: "u", AccessToken: "a"},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Save("alice@proton.me", s); err == nil {
				t.Error("Save should reject partial session")
			}
		})
	}
}

func TestIsolationBetweenUsers(t *testing.T) {
	resetKeyring(t)

	a := fixtureSession()
	a.UID = "uid-alice"

	b := fixtureSession()
	b.UID = "uid-bob"
	b.AccessToken = "acc-bob"

	if err := Save("alice@proton.me", a); err != nil {
		t.Fatal(err)
	}
	if err := Save("bob@proton.me", b); err != nil {
		t.Fatal(err)
	}

	gotA, err := Load("alice@proton.me")
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := Load("bob@proton.me")
	if err != nil {
		t.Fatal(err)
	}
	if gotA.UID != "uid-alice" || gotB.UID != "uid-bob" {
		t.Errorf("cross-user contamination: %+v vs %+v", gotA, gotB)
	}
}
