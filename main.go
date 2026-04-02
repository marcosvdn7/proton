package main

import (
	"fmt"
	"os"

	"proton-signup/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		cmd.Init()
	case "check":
		if len(os.Args) < 3 {
			fmt.Println("Usage: proton-signup check <username>")
			os.Exit(1)
		}
		cmd.Check(os.Args[2])
	case "fill":
		field := ""
		if len(os.Args) >= 3 {
			field = os.Args[2]
		}
		cmd.Fill(field)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`proton-signup — Proton account signup helper

Commands:
  init             Generate a default account.yaml template
  check <username> Check if a Proton username is available
  fill             Interactive mode: copy each field to clipboard
  fill <field>     Copy a single field (username, password, recovery_email, recovery_phone)
  help             Show this help message`)
}
