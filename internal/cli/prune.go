package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/artisanexperiences/arbor/internal/git"
	"github.com/artisanexperiences/arbor/internal/scaffold/types"
	"github.com/artisanexperiences/arbor/internal/ui"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove merged workspaces",
	Long: `Removes merged workspaces automatically.

Lists all workspaces, identifies merged ones, and provides an
interactive review before removal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pc, err := OpenProjectFromCWD()
		if err != nil {
			return err
		}

		force := mustGetBool(cmd, "force")
		dryRun := mustGetBool(cmd, "dry-run")
		verbose := mustGetBool(cmd, "verbose")
		quiet := mustGetBool(cmd, "quiet")

		wsManager := pc.WorkspaceManager()
		workspaces, err := wsManager.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("listing workspaces: %w", err)
		}

		// Convert to git.Worktree for existing UI functions
		worktrees := workspacesToGitWorktrees(workspaces)

		// Determine the git repo path used for merge checks once, based on mode.
		var mergeCheckPath string
		if pc.Mode == workspace.ModeCow {
			mergeCheckPath = filepath.Join(pc.ProjectPath, pc.DefaultBranch)
		} else {
			mergeCheckPath = pc.BarePath
		}

		var removable []git.Worktree

		for _, wt := range worktrees {
			if wt.Branch == pc.DefaultBranch || wt.Branch == "(bare)" {
				ui.PrintInfo(fmt.Sprintf("%s at %s", wt.Branch, wt.Path))
				continue
			}

			merged, err := git.IsMerged(mergeCheckPath, wt.Branch, pc.DefaultBranch)
			if err != nil {
				ui.PrintErrorWithHint(fmt.Sprintf("Error checking %s", wt.Branch), err.Error())
				continue
			}

			if merged {
				removable = append(removable, wt)
				ui.PrintSuccess(fmt.Sprintf("%s is merged", wt.Branch))
			} else {
				ui.PrintInfo(fmt.Sprintf("%s is not merged", wt.Branch))
			}
		}

		if len(removable) == 0 {
			ui.PrintDone("No merged workspaces to remove.")
			return nil
		}

		ui.PrintInfo(fmt.Sprintf("%d merged workspace(s) found.", len(removable)))

		var toRemove []git.Worktree
		if force {
			toRemove = removable
		} else {
			selected, err := ui.SelectWorktreesToPrune(removable)
			if err != nil {
				return fmt.Errorf("selecting workspaces: %w", err)
			}
			toRemove = selected

			if len(toRemove) == 0 {
				ui.PrintInfo("No workspaces selected for removal.")
				return nil
			}

			confirmed, err := ui.ConfirmRemoval(len(toRemove))
			if err != nil {
				return fmt.Errorf("confirmation: %w", err)
			}
			if !confirmed {
				ui.PrintInfo("No workspaces removed.")
				return nil
			}
		}

		ui.PrintInfo(fmt.Sprintf("Removing %d workspace(s):", len(toRemove)))
		for _, wt := range toRemove {
			ui.PrintSuccessPath("Removed", wt.Path)
		}

		for _, wt := range toRemove {
			ui.PrintStep(fmt.Sprintf("Removing %s...", wt.Branch))

			if !dryRun {
				preset := pc.Config.Preset
				if preset == "" {
					preset = pc.PresetManager().Detect(wt.Path)
				}

				siteName := filepath.Base(wt.Path)
				promptMode := types.PromptMode{
					Interactive:   ui.IsInteractive(),
					NoInteractive: false,
					Force:         false,
					CI:            os.Getenv("CI") != "",
				}
				if err := pc.ScaffoldManager().RunCleanup(wt.Path, wt.Branch, "", siteName, preset, pc.Config, pc.BarePath, promptMode, false, verbose, quiet); err != nil {
					ui.PrintErrorWithHint("Cleanup failed", err.Error())
				}

				if err := wsManager.RemoveWorkspace(wt.Path, true); err != nil {
					ui.PrintErrorWithHint(fmt.Sprintf("Error removing %s", wt.Branch), err.Error())
				}
			} else {
				ui.PrintInfo(fmt.Sprintf("[DRY RUN] Would remove %s and run cleanup", wt.Branch))
			}
		}

		ui.PrintDone("Done.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().BoolP("force", "f", false, "Skip interactive confirmation")
}
