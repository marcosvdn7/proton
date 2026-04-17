package signup

import (
	"fmt"
	"os"
)

// Run dispatches signup subcommands: check, init, fill, help.
// This is the ONLY place in the signup package that calls os.Exit.
func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	var err error

	switch args[0] {
	case "check":
		if len(args) < 2 {
			fmt.Println("Usage: proton signup check <username>")
			os.Exit(1)
		}
		err = Check(args[1])
	case "init":
		err = Init()
	case "fill":
		field := ""
		if len(args) >= 2 {
			field = args[1]
		}
		err = Fill(field)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Printf("Unknown signup command: %s\n\n", args[0])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`proton signup — Account creation helper

Commands:
  check <username>  Check if a Proton username is available
  init              Generate a default account.yaml template
  fill              Interactive mode: copy each field to clipboard
  fill <field>      Copy a single field (username, password, recovery_email, recovery_phone)
  help              Show this help message`)
}
