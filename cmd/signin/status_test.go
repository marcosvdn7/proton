package signin

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"proton/internal/keychain"

	"github.com/zalando/go-keyring"
)

// Every test in this file must call keyring.MockInit() before touching
// runStatus/runLogout — those go through the real internal/keychain package
// which in turn goes through go-keyring, and we don't want CI or dev
// machines' actual keyrings involved.
func mockKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
}

// withAccountYAMLDir chdirs into a temp dir and drops an account.yaml so
// resolveUsername has something to read. Returns the dir for extra writes.
func withAccountYAMLDir(t *testing.T, username string) string {
	t.Helper()
	dir := t.TempDir()
	if username != "" {
		body := "username: \"" + username + "\"\n"
		if err := os.WriteFile(filepath.Join(dir, "account.yaml"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

func TestResolveUsername_FlagWins(t *testing.T) {
	withAccountYAMLDir(t, "from-file@proton.me")
	got, err := resolveUsername("explicit@proton.me")
	if err != nil {
		t.Fatal(err)
	}
	if got != "explicit@proton.me" {
		t.Errorf("want explicit, got %q", got)
	}
}

func TestResolveUsername_FallsBackToAccountYAML(t *testing.T) {
	withAccountYAMLDir(t, "yaml-user@proton.me")
	got, err := resolveUsername("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "yaml-user@proton.me" {
		t.Errorf("want yaml-user, got %q", got)
	}
}

func TestResolveUsername_NoFlagNoFileFails(t *testing.T) {
	// Chdir into an empty temp dir so account.yaml is guaranteed absent.
	withAccountYAMLDir(t, "")
	if _, err := resolveUsername(""); err == nil {
		t.Error("expected error when both flag and account.yaml are empty")
	}
}

func TestRunStatus_NoSavedSession(t *testing.T) {
	mockKeyring(t)
	withAccountYAMLDir(t, "alice@proton.me")

	var out bytes.Buffer
	if err := runStatus("", &out); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	if !strings.Contains(out.String(), "No saved session") {
		t.Errorf("expected 'No saved session' line, got %q", out.String())
	}
}

func TestRunStatus_ShowsSummaryButNotTokens(t *testing.T) {
	mockKeyring(t)
	withAccountYAMLDir(t, "alice@proton.me")

	sess := keychain.Session{
		UID:          "uid-visible",
		AccessToken:  "SUPER-SECRET-ACCESS",
		RefreshToken: "SUPER-SECRET-REFRESH",
		Scope:        "self mail",
		SavedAt:      time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := keychain.Save("alice@proton.me", sess); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runStatus("", &out); err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	got := out.String()

	// Positive: UID and scope should appear.
	for _, want := range []string{"alice@proton.me", "uid-visible", "self mail", "2025-01-02T03:04:05Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
	// Negative: tokens must never be printed.
	for _, forbidden := range []string{"SUPER-SECRET-ACCESS", "SUPER-SECRET-REFRESH"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("output leaked token %q:\n%s", forbidden, got)
		}
	}
}

func TestRunLogout_DeletesEntry(t *testing.T) {
	mockKeyring(t)
	withAccountYAMLDir(t, "alice@proton.me")

	if err := keychain.Save("alice@proton.me", keychain.Session{
		UID: "u", AccessToken: "a", RefreshToken: "r",
	}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runLogout("", &out); err != nil {
		t.Fatalf("runLogout: %v", err)
	}
	if _, err := keychain.Load("alice@proton.me"); !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("after runLogout, keychain.Load should return ErrNotFound, got %v", err)
	}
	if !strings.Contains(out.String(), "Signed out") {
		t.Errorf("expected 'Signed out' line, got %q", out.String())
	}
}

func TestRunLogout_IdempotentOnMissing(t *testing.T) {
	mockKeyring(t)
	withAccountYAMLDir(t, "ghost@proton.me")

	// Nothing was ever saved. Logout should still succeed silently so
	// scripts can `signin --logout` without conditionals.
	var out bytes.Buffer
	if err := runLogout("", &out); err != nil {
		t.Errorf("runLogout on missing session should be no-op, got %v", err)
	}
}
