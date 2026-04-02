package main

import (
	"fmt"
	"os"

	"proton/cmd/mail"
	"proton/cmd/signin"
	"proton/cmd/signup"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

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
