package main

import (
	"fmt"
	"os"

	"proton/cmd/mail"
	"proton/cmd/signin"
	"proton/cmd/signup"
	"proton/internal/log"
)

func main() {
	// Parse --verbose flag and remove it from args
	verbose := false
	args := os.Args[1:]
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--verbose" || arg == "-v" {
			verbose = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Initialize logger with verbose setting
	log.Init(verbose)

	if len(filteredArgs) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := filteredArgs[0]
	args = filteredArgs[1:]

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
  signin    Sign in to your Proton account (coming soon)
  mail      Manage emails: fetch, read, reply (coming soon)
  help      Show this help message

Run 'proton <command> help' for details on a specific command.`)
}
