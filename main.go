package main

import (
	"flag"
	"fmt"
	"os"

	"proton/cmd/mail"
	"proton/cmd/signin"
	"proton/cmd/signup"
	"proton/internal/log"
)

func main() {
	// Global flags parsed before the subcommand.
	// Usage: proton [--verbose|-v] <command> [args...]
	fs := flag.NewFlagSet("proton", flag.ExitOnError)
	fs.Usage = printUsage

	var verbose bool
	fs.BoolVar(&verbose, "verbose", false, "enable debug logging")
	fs.BoolVar(&verbose, "v", false, "enable debug logging (shorthand)")

	// Parse stops at first non-flag token, leaving subcommand + its args in fs.Args().
	_ = fs.Parse(os.Args[1:])

	log.Init(verbose)

	rest := fs.Args()
	if len(rest) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := rest[0]
	args := rest[1:]

	switch command {
	case "signup":
		signup.Run(args)
	case "signin":
		signin.Run(args)
	case "mail":
		mail.Run(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`proton — Proton Mail CLI tool

Commands:
  signup    Account creation helper (check username, generate config, fill form)
  signin    Sign in to your Proton account (SRP)
  mail      Manage emails: fetch, read, reply (coming soon)
  help      Show this help message

Run 'proton <command> help' for details on a specific command.`)
}
