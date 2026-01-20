package ui

import (
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func ShouldPrompt(cmd *cobra.Command, hasRequiredArgs bool) bool {
	if cmd == nil {
		return IsInteractive()
	}

	noInteractive, _ := cmd.Flags().GetBool("no-interactive")
	if noInteractive {
		return false
	}

	force, _ := cmd.Flags().GetBool("force")
	if force {
		return false
	}

	if os.Getenv("CI") != "" {
		return false
	}

	return IsInteractive() && !hasRequiredArgs
}

func IsInteractive() bool {
	return term.IsTerminal(os.Stdout.Fd())
}
