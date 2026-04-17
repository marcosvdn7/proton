package signin

import "fmt"

// Run handles the signin subcommand (placeholder).
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}
	fmt.Println("🚧 proton signin — coming soon!")
	fmt.Println()
	printUsage()
}

func printUsage() {
	fmt.Println(`proton signin — Sign in to your Proton account

Planned features:
  proton signin              Authenticate with Proton
  proton signin status       Check current session
  proton signin logout       End current session`)
}
