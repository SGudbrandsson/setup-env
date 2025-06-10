package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(*model); ok {
		if fm.err != nil {
			// If no changes were made, diffSummary will contain "No changes..."
			// and fm.err will be nil if prepareForConfirmation quit early.
			// Only print fm.err if it's a real error.
			if fm.diffSummary != "No changes to apply to .env file." || fm.err.Error() != "" { // Check if it's not the "no changes" message
				fmt.Printf("%v\n", fm.err)
			}
			if strings.Contains(fm.err.Error(), ".env.example file not found") || strings.Contains(fm.err.Error(), "No environment variables found") {
				os.Exit(1)
			}
		} else if fm.diffSummary == "No changes to apply to .env file." && !fm.confirming {
			// If quitting because no changes, print the summary.
			// This ensures "No changes to apply..." is visible if that was the reason for quitting.
			fmt.Println("\n" + fm.diffSummary)
		}
	}
}
