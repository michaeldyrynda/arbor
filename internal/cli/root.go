package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "arbor",
	Short: "Git worktree manager for agentic development",
	Long: `Arbor is a self-contained binary for managing git worktrees
to assist with agentic development of applications.
It is cross-project, cross-language, and cross-environment compatible.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview operations without executing")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
}

func mustGetString(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: flag %q not defined: %v", name, err))
	}
	return value
}

func mustGetBool(cmd *cobra.Command, name string) bool {
	value, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: flag %q not defined: %v", name, err))
	}
	return value
}
