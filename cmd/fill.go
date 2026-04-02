package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Plan     string `yaml:"plan"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Recovery struct {
		RecoveryEmail string `yaml:"recovery_email"`
		RecoveryPhone string `yaml:"recovery_phone"`
	} `yaml:"recovery"`
}

type field struct {
	label string
	value string
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile("account.yaml")
	if err != nil {
		return nil, fmt.Errorf("cannot read account.yaml: %w\n\nRun 'proton-signup init' to create one")
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return &cfg, nil
}

func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (cfg *Config) fields() []field {
	return []field{
		{"username", cfg.Username},
		{"password", cfg.Password},
		{"recovery_email", cfg.Recovery.RecoveryEmail},
		{"recovery_phone", cfg.Recovery.RecoveryPhone},
	}
}

func (cfg *Config) getField(name string) (string, bool) {
	for _, f := range cfg.fields() {
		if f.label == name {
			return f.value, true
		}
	}
	return "", false
}

func Fill(fieldName string) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Single field mode
	if fieldName != "" {
		value, ok := cfg.getField(fieldName)
		if !ok {
			fmt.Fprintf(os.Stderr, "❌ Unknown field: %s\n", fieldName)
			fmt.Println("Available: username, password, recovery_email, recovery_phone")
			os.Exit(1)
		}
		if value == "" {
			fmt.Printf("⚠️  %s is empty in account.yaml\n", fieldName)
			os.Exit(1)
		}
		if err := copyToClipboard(value); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying to clipboard: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📋 %s → copied to clipboard\n", fieldName)
		return
	}

	// Interactive mode
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

	// Plan selection
	fmt.Println("── Step 1: Plan ──")
	fmt.Printf("📌 Select plan: %s\n", cfg.Plan)
	fmt.Print("   Press Enter when selected...")
	scanner.Scan()
	fmt.Println()

	// Credentials
	fmt.Println("── Step 2: Credentials ──")
	copyField(scanner, "Username", cfg.Username)
	copyField(scanner, "Password", cfg.Password)
	fmt.Println()

	// Recovery
	fmt.Println("── Step 3: Recovery (after signup) ──")
	copyField(scanner, "Recovery Email", cfg.Recovery.RecoveryEmail)
	copyField(scanner, "Recovery Phone", cfg.Recovery.RecoveryPhone)
	fmt.Println()

	fmt.Println("✅ Done! Complete any CAPTCHA/verification manually.")
}

func copyField(scanner *bufio.Scanner, label, value string) {
	if value == "" {
		fmt.Printf("⏭  %s: (empty, skipping)\n", label)
		return
	}
	if err := copyToClipboard(value); err != nil {
		fmt.Fprintf(os.Stderr, "   Error copying: %v\n", err)
		return
	}
	fmt.Printf("📋 %s → copied to clipboard\n", label)
	fmt.Print("   Press Enter when pasted, or 's' to skip... ")
	scanner.Scan()
}
