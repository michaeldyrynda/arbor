package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	arborerrors "github.com/michaeldyrynda/arbor/internal/errors"
	"github.com/michaeldyrynda/arbor/internal/git"
	"github.com/michaeldyrynda/arbor/internal/ui"
)

var removeCmd = &cobra.Command{
	Use:   "remove [FOLDER]",
	Short: "Remove a worktree with cleanup",
	Long: `Removes a worktree and runs preset-defined cleanup steps.

Arguments:
  FOLDER  Name of the worktree folder to remove (e.g., feature-test-change)

Cleanup steps may include:
  - Removing Herd site links
  - Database cleanup prompts`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pc, err := OpenProjectFromCWD()
		if err != nil {
			return err
		}

		force := mustGetBool(cmd, "force")
		dryRun := mustGetBool(cmd, "dry-run")
		verbose := mustGetBool(cmd, "verbose")

		worktrees, err := git.ListWorktrees(pc.BarePath)
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}

		var targetWorktree *git.Worktree

		if len(args) > 0 {
			folderName := args[0]
			for _, wt := range worktrees {
				if filepath.Base(wt.Path) == folderName {
					targetWorktree = &wt
					break
				}
			}
			if targetWorktree == nil {
				return fmt.Errorf("worktree '%s' not found: %w", folderName, arborerrors.ErrWorktreeNotFound)
			}
		} else if ui.ShouldPrompt(cmd, false) {
			selected, err := ui.SelectWorktreeToRemove(worktrees)
			if err != nil {
				return fmt.Errorf("selecting worktree: %w", err)
			}
			targetWorktree = selected
		} else {
			return fmt.Errorf("worktree folder name required")
		}

		if targetWorktree.IsMain {
			return fmt.Errorf("cannot remove main worktree")
		}

		ui.PrintInfo(fmt.Sprintf("Removing %s at %s", targetWorktree.Branch, targetWorktree.Path))

		if !force && ui.ShouldPrompt(cmd, true) {
			ui.PrintInfo("This will run cleanup steps.")
			confirmed, err := ui.Confirm(fmt.Sprintf("Remove worktree '%s'?", targetWorktree.Branch))
			if err != nil {
				return fmt.Errorf("confirmation: %w", err)
			}
			if !confirmed {
				ui.PrintInfo("Cancelled.")
				return nil
			}
		}

		ui.PrintStep("Removing worktree")

		if !dryRun {
			preset := pc.Config.Preset
			if preset == "" {
				preset = pc.PresetManager().Detect(targetWorktree.Path)
			}

			ui.PrintInfo(fmt.Sprintf("Running cleanup for preset: %s", preset))

			if err := pc.ScaffoldManager().RunCleanup(targetWorktree.Path, targetWorktree.Branch, "", preset, pc.Config, false, verbose); err != nil {
				ui.PrintErrorWithHint("Cleanup failed", err.Error())
			}

			if err := git.RemoveWorktree(targetWorktree.Path, true); err != nil {
				return fmt.Errorf("removing worktree: %w", err)
			}
			ui.PrintSuccessPath("Removed", targetWorktree.Path)

			parentDir := filepath.Dir(targetWorktree.Path)
			entries, err := os.ReadDir(parentDir)
			if err == nil && len(entries) == 0 {
				if err := os.Remove(parentDir); err != nil {
					ui.PrintErrorWithHint(fmt.Sprintf("Could not remove empty directory %s", parentDir), err.Error())
				}
			}
		} else {
			ui.PrintInfo("[DRY RUN] Would run cleanup and remove worktree")
		}

		ui.PrintDone("Worktree removed")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)

	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation and cleanup prompts")
}
