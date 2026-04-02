package signup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mockClipboard records what was copied
type mockClipboard struct {
	copied string
	err    error
}

func (m *mockClipboard) Copy(text string) error {
	m.copied = text
	return m.err
}

const validYAML = `
plan: "Free"
username: "TestUser"
password: "s3cret123"
recovery:
  recovery_email: "test@example.com"
  recovery_phone: "+1234567890"
`

const minimalYAML = `
plan: "Free"
username: "TestUser"
password: ""
recovery:
  recovery_email: ""
  recovery_phone: ""
`

// --- loadConfigFrom tests ---

func TestLoadConfigFrom_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	cfg, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Plan != "Free" {
		t.Errorf("expected plan 'Free', got %q", cfg.Plan)
	}
	if cfg.Username != "TestUser" {
		t.Errorf("expected username 'TestUser', got %q", cfg.Username)
	}
	if cfg.Password != "s3cret123" {
		t.Errorf("expected password 's3cret123', got %q", cfg.Password)
	}
	if cfg.Recovery.RecoveryEmail != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", cfg.Recovery.RecoveryEmail)
	}
	if cfg.Recovery.RecoveryPhone != "+1234567890" {
		t.Errorf("expected phone '+1234567890', got %q", cfg.Recovery.RecoveryPhone)
	}
}

func TestLoadConfigFrom_MissingFile(t *testing.T) {
	_, err := loadConfigFrom("/nonexistent/path/account.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("{{not yaml: ["), 0644)

	_, err := loadConfigFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfigFrom_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	os.WriteFile(path, []byte(""), 0644)

	cfg, err := loadConfigFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty YAML should produce zero-value config
	if cfg.Username != "" {
		t.Errorf("expected empty username, got %q", cfg.Username)
	}
}

// --- Config.Fields() tests ---

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		Username: "user1",
		Password: "pass1",
	}
	cfg.Recovery.RecoveryEmail = "a@b.com"
	cfg.Recovery.RecoveryPhone = "+1"

	fields := cfg.Fields()
	if len(fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(fields))
	}

	expected := map[string]string{
		"username":       "user1",
		"password":       "pass1",
		"recovery_email": "a@b.com",
		"recovery_phone": "+1",
	}

	for _, f := range fields {
		want, ok := expected[f.Label]
		if !ok {
			t.Errorf("unexpected field label %q", f.Label)
			continue
		}
		if f.Value != want {
			t.Errorf("field %q: expected %q, got %q", f.Label, want, f.Value)
		}
	}
}

// --- Config.GetField() tests ---

func TestConfig_GetField_Exists(t *testing.T) {
	cfg := &Config{Username: "alice"}

	val, ok := cfg.GetField("username")
	if !ok {
		t.Fatal("expected ok=true for existing field")
	}
	if val != "alice" {
		t.Errorf("expected 'alice', got %q", val)
	}
}

func TestConfig_GetField_NotExists(t *testing.T) {
	cfg := &Config{}

	_, ok := cfg.GetField("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent field")
	}
}

func TestConfig_GetField_EmptyValue(t *testing.T) {
	cfg := &Config{Username: ""}

	val, ok := cfg.GetField("username")
	if !ok {
		t.Fatal("expected ok=true even for empty value")
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

// --- FillWithClipboard tests ---

func TestFillWithClipboard_SingleField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("username", path, clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clip.copied != "TestUser" {
		t.Errorf("expected clipboard to contain 'TestUser', got %q", clip.copied)
	}
}

func TestFillWithClipboard_Password(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("password", path, clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clip.copied != "s3cret123" {
		t.Errorf("expected clipboard to contain 's3cret123', got %q", clip.copied)
	}
}

func TestFillWithClipboard_UnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("bogus_field", path, clip)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestFillWithClipboard_EmptyField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(minimalYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("password", path, clip)
	if err == nil {
		t.Fatal("expected error for empty field")
	}
}

func TestFillWithClipboard_ClipboardError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{err: errors.New("clipboard broken")}
	err := FillWithClipboard("username", path, clip)
	if err == nil {
		t.Fatal("expected error when clipboard fails")
	}
}

func TestFillWithClipboard_MissingConfig(t *testing.T) {
	clip := &mockClipboard{}
	err := FillWithClipboard("username", "/nonexistent/account.yaml", clip)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestFillWithClipboard_RecoveryEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("recovery_email", path, clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clip.copied != "test@example.com" {
		t.Errorf("expected 'test@example.com', got %q", clip.copied)
	}
}

func TestFillWithClipboard_RecoveryPhone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	os.WriteFile(path, []byte(validYAML), 0644)

	clip := &mockClipboard{}
	err := FillWithClipboard("recovery_phone", path, clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clip.copied != "+1234567890" {
		t.Errorf("expected '+1234567890', got %q", clip.copied)
	}
}
