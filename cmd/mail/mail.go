package mail

import "fmt"

// Run handles the mail subcommand (placeholder).
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}
	fmt.Println("🚧 proton mail — coming soon!")
	fmt.Println()
	printUsage()
}

func printUsage() {
	fmt.Println(`proton mail — Manage your Proton emails

Planned features:
  proton mail fetch          Fetch recent emails
  proton mail read <id>      Read a specific email
  proton mail reply <id>     Reply to an email
  proton mail send           Compose and send a new email
  proton mail search <query> Search emails`)
}
