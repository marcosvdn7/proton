package signup

import (
	"flag"
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
		fs := flag.NewFlagSet("signup check", flag.ExitOnError)
		fs.Usage = func() {
			fmt.Println("Usage: proton signup check [--json] <username> [username...]")
		}
		jsonOut := fs.Bool("json", false, "emit results as a JSON array")
		_ = fs.Parse(args[1:])
		names := fs.Args()
		if len(names) == 0 {
			fs.Usage()
			os.Exit(1)
		}
		var any bool
		any, err = CheckBatch(names, *jsonOut)
		if err == nil && !any {
			// Non-zero exit lets scripts test 'if proton signup check a b; then ...'
			os.Exit(1)
		}
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
  check <username> [username...] [--json]
                    Check if one or more Proton usernames are available
  init              Generate a default account.yaml template
  fill              Interactive mode: copy each field to clipboard
  fill <field>      Copy a single field (username, password, recovery_email, recovery_phone)
  help              Show this help message`)
}
