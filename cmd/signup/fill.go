package signup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the default path for the account configuration file.
var DefaultConfigPath = "account.yaml"

// Clipboard interface for copying text to clipboard.
type Clipboard interface {
	Copy(text string) error
}

// PbCopyClipboard implements Clipboard using macOS pbcopy command.
type PbCopyClipboard struct{}

// Copy copies text to clipboard using pbcopy.
func (c *PbCopyClipboard) Copy(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// Config represents the account configuration.
type Config struct {
	Plan     string `yaml:"plan"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Recovery struct {
		RecoveryEmail string `yaml:"recovery_email"`
		RecoveryPhone string `yaml:"recovery_phone"`
	} `yaml:"recovery"`
}

// Field represents a configuration field with label and value.
type Field struct {
	Label string
	Value string
}

// loadConfigFrom loads configuration from the specified path.
func loadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w\n\nRun 'proton signup init' to create one", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return &cfg, nil
}

// loadConfig loads configuration from the default path.
func loadConfig() (*Config, error) {
	return loadConfigFrom(DefaultConfigPath)
}

// Fields returns all configuration fields as a slice.
func (cfg *Config) Fields() []Field {
	return []Field{
		{"username", cfg.Username},
		{"password", cfg.Password},
		{"recovery_email", cfg.Recovery.RecoveryEmail},
		{"recovery_phone", cfg.Recovery.RecoveryPhone},
	}
}

// GetField returns the value of a field by name.
func (cfg *Config) GetField(name string) (string, bool) {
	for _, f := range cfg.Fields() {
		if f.Label == name {
			return f.Value, true
		}
	}
	return "", false
}

// Fill handles field filling with clipboard functionality.
func Fill(fieldName string) {
	clipboard := &PbCopyClipboard{}
	if err := FillWithClipboard(fieldName, DefaultConfigPath, clipboard); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// FillWithClipboard handles field filling with a configurable clipboard and config path.
func FillWithClipboard(fieldName, configPath string, clipboard Clipboard) error {
	cfg, err := loadConfigFrom(configPath)
	if err != nil {
		return err
	}

	if fieldName != "" {
		return fillSingleField(cfg, fieldName, clipboard)
	}

	return fillInteractive(cfg, clipboard)
}

func fillSingleField(cfg *Config, fieldName string, clipboard Clipboard) error {
	value, ok := cfg.GetField(fieldName)
	if !ok {
		return fmt.Errorf("unknown field: %s (available: username, password, recovery_email, recovery_phone)", fieldName)
	}
	if value == "" {
		return fmt.Errorf("field %s is empty in account.yaml", fieldName)
	}
	if err := clipboard.Copy(value); err != nil {
		return fmt.Errorf("error copying to clipboard: %w", err)
	}
	fmt.Printf("📋 %s → copied to clipboard\n", fieldName)
	return nil
}

func fillInteractive(cfg *Config, clipboard Clipboard) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  Proton Account Signup Helper")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()
	fmt.Println("Open: https://account.proton.me/mail/signup")
	fmt.Println()
	fmt.Print("Press Enter to start...")
	scanner.Scan()
	fmt.Println()

	fmt.Println("── Step 1: Plan ──")
	fmt.Printf("📌 Select plan: %s\n", cfg.Plan)
	fmt.Print("   Press Enter when selected...")
	scanner.Scan()
	fmt.Println()

	fmt.Println("── Step 2: Credentials ──")
	copyFieldInteractive(scanner, "Username", cfg.Username, clipboard)
	copyFieldInteractive(scanner, "Password", cfg.Password, clipboard)
	fmt.Println()

	fmt.Println("── Step 3: Recovery (after signup) ──")
	copyFieldInteractive(scanner, "Recovery Email", cfg.Recovery.RecoveryEmail, clipboard)
	copyFieldInteractive(scanner, "Recovery Phone", cfg.Recovery.RecoveryPhone, clipboard)
	fmt.Println()

	fmt.Println("✅ Done! Complete any CAPTCHA/verification manually.")
	return nil
}

func copyFieldInteractive(scanner *bufio.Scanner, label, value string, clipboard Clipboard) {
	if value == "" {
		fmt.Printf("⏭  %s: (empty, skipping)\n", label)
		return
	}
	if err := clipboard.Copy(value); err != nil {
		fmt.Fprintf(os.Stderr, "   Error copying: %v\n", err)
		return
	}
	fmt.Printf("📋 %s → copied to clipboard\n", label)
	fmt.Print("   Press Enter when pasted, or 's' to skip... ")
	scanner.Scan()
}
