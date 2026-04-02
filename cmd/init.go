package cmd

import (
	"fmt"
	"os"
)

const defaultYAML = `# Proton Account Signup Info
# Fill in the fields below, then run: proton-signup fill

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

func Init() {
	path := "account.yaml"
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("⚠️  %s already exists. Overwrite? [y/N] ", path)
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	if err := os.WriteFile(path, []byte(defaultYAML), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("✅ Created %s — edit it with your details.\n", path)
}
