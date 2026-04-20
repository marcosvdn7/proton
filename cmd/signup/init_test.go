package signup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitConfig_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")

	err := InitConfig(path, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("created file is empty")
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}

	if cfg.Plan != "Free" {
		t.Errorf("expected plan 'Free', got %q", cfg.Plan)
	}
	if cfg.Username != "" {
		t.Errorf("expected empty username placeholder, got %q", cfg.Username)
	}
	if cfg.Password != "" {
		t.Errorf("expected empty password, got %q", cfg.Password)
	}
}

func TestInitConfig_FileExistsNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")

	os.WriteFile(path, []byte("existing content"), 0644)

	err := InitConfig(path, false)
	if err == nil {
		t.Fatal("expected error when file exists and overwrite=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "existing content" {
		t.Error("original file content was modified")
	}
}

func TestInitConfig_FileExistsOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")

	os.WriteFile(path, []byte("old content"), 0644)

	err := InitConfig(path, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) == "old content" {
		t.Error("file was not overwritten")
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("overwritten YAML is invalid: %v", err)
	}
}

func TestInitConfig_WritePermissionError(t *testing.T) {
	path := "/nonexistent/dir/account.yaml"

	err := InitConfig(path, false)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestInitConfig_DefaultYAMLContainsExpectedFields(t *testing.T) {
	for _, expected := range []string{"plan:", "username:", "password:", "recovery_email:", "recovery_phone:"} {
		if !strings.Contains(DefaultYAML, expected) {
			t.Errorf("DefaultYAML missing field %q", expected)
		}
	}
}

func TestInitConfig_FilePermissions_OwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")

	InitConfig(path, false)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	perm := info.Mode().Perm()
	// Config file holds passwords — MUST be 0600 (owner-only), NOT 0644.
	if perm != 0600 {
		t.Errorf("expected 0600 (owner read/write only), got %04o — this file holds passwords!", perm)
	}
}
