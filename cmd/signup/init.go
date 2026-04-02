package signup

import (
	"fmt"
	"os"

	"proton/internal/log"
)

// DefaultYAML is the default YAML template for account configuration
const DefaultYAML = `# Proton Account Signup Info
# Fill in the fields below, then run: proton signup fill

# Plan: Free | Mail Plus | Proton Unlimited | Proton Family
plan: "Free"

# Account Credentials
username: "LucianoJr"    # Will become <username>@proton.me
password: ""              # Min 8 chars, use a strong password!

# Recovery (optional, prompted after account creation)
recovery:
  recovery_email: ""      # Backup email for account recovery
  recovery_phone: ""      # Phone number for account recovery
`

// InitConfig creates a configuration file at the specified path.
// If overwrite is false and the file exists, it returns an error.
// If overwrite is true, it will overwrite an existing file.
func InitConfig(path string, overwrite bool) error {
	log.Debug("InitConfig called", "path", path, "overwrite", overwrite)

	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			log.Debug("File already exists and overwrite=false", "path", path)
			return fmt.Errorf("file %s already exists", path)
		}
	}

	log.Info("Creating configuration file", "path", path)
	if err := os.WriteFile(path, []byte(DefaultYAML), 0644); err != nil {
		log.Error("Failed to write configuration file", "path", path, "error", err)
		return fmt.Errorf("error writing %s: %w", path, err)
	}

	log.Info("Configuration file created successfully", "path", path)
	return nil
}

// Init creates an account.yaml configuration file with interactive prompts.
// It handles user confirmation for overwriting existing files.
func Init() {
	path := "account.yaml"
	overwrite := false

	if _, err := os.Stat(path); err == nil {
		fmt.Printf("⚠️  %s already exists. Overwrite? [y/N] ", path)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			log.Info("User aborted configuration file creation")
			return
		}
		overwrite = true
	}

	if err := InitConfig(path, overwrite); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Created %s — edit it with your details.\n", path)
}
