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

		// stdlib flag.Parse stops at the first non-flag token, so mixing
		// positional usernames and flags (e.g. 'check foo bar --json') would
		// silently treat '--json' as a username. Split them up front.
		flagsOnly, positional := splitFlagsAndPositional(args[1:])
		_ = fs.Parse(flagsOnly)
		names := positional
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

// splitFlagsAndPositional separates flag tokens from positional args so we can
// feed only flags to flag.FlagSet.Parse and accept flags in any position.
//
// A token is treated as a flag when it starts with '-' (single or double dash).
// The value that follows a '--name value' style flag is also kept in the flag
// group. '--name=value' is single-token and handled naturally.
// '--' terminates flag parsing: everything after is positional.
func splitFlagsAndPositional(args []string) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			return
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			continue
		}
		// Note: only bool flags and '--name=value' are supported today.
		// If a future flag needs a separate value token ('--out foo'),
		// this splitter must be extended to consume it here.
		positional = append(positional, a)
	}
	return
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
