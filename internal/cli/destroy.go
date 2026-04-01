package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/artisanexperiences/arbor/internal/config"
	"github.com/artisanexperiences/arbor/internal/git"
	"github.com/artisanexperiences/arbor/internal/presets"
	"github.com/artisanexperiences/arbor/internal/scaffold"
	"github.com/artisanexperiences/arbor/internal/scaffold/steps"
	"github.com/artisanexperiences/arbor/internal/scaffold/types"
	"github.com/artisanexperiences/arbor/internal/ui"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [PROJECT_PATH]",
	Short: "Completely destroy an arbor project",
	Long: `Destroys an arbor project by:
  1. Finding all workspaces
  2. Running cleanup for each (features first, then main)
  3. Removing all workspaces and branches
  4. Deleting the project folder

This operation cannot be undone.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun := mustGetBool(cmd, "dry-run")
		verbose := mustGetBool(cmd, "verbose")
		quiet := mustGetBool(cmd, "quiet")
		force := mustGetBool(cmd, "force")

		var projectPath string
		if len(args) > 0 {
			projectPath = args[0]
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			selected, err := ui.SelectProjectToDestroy(cwd)
			if err != nil {
				return err
			}
			projectPath = selected
		}

		absProjectPath, err := filepath.Abs(projectPath)
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		if cwd == absProjectPath || strings.HasPrefix(cwd, absProjectPath+string(filepath.Separator)) {
			return fmt.Errorf("cannot destroy project from within it; cd out first")
		}

		cfg, err := config.LoadProject(absProjectPath)
		if err != nil {
			return fmt.Errorf("not an arbor project: %w", err)
		}

		// Determine mode and barePath
		info, err := workspace.FindProjectRoot(absProjectPath)
		if err != nil {
			return fmt.Errorf("finding project root: %w", err)
		}

		defaultBranch := cfg.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		wsManager := workspace.NewManager(info, defaultBranch)

		workspaces, err := wsManager.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("listing workspaces: %w", err)
		}

		worktrees := workspacesToGitWorktrees(workspaces)
		worktrees = sortWorktreesForDestroy(worktrees, defaultBranch)

		projectName := cfg.SiteName
		if projectName == "" {
			projectName = filepath.Base(absProjectPath)
		}

		if !force && !dryRun {
			confirmed, err := ui.ConfirmDestroy(projectName, worktrees)
			if err != nil {
				return err
			}
			if !confirmed {
				ui.PrintInfo("Cancelled.")
				return nil
			}
		}

		if dryRun {
			ui.PrintInfo(fmt.Sprintf("Would destroy project %q with %d workspace(s):", projectName, len(worktrees)))
			for _, wt := range worktrees {
				ui.PrintInfo(fmt.Sprintf("  - %s", wt.Branch))
			}
			return nil
		}

		preset := cfg.Preset
		presetManager := presets.NewManager()

		// Create explicit step registry with default steps
		stepRegistry := steps.NewRegistry()
		stepRegistry.RegisterDefaults()
		scaffoldManager := scaffold.NewScaffoldManagerWithRegistry(stepRegistry)
		presets.RegisterAllWithScaffold(scaffoldManager)

		allCleanupFailed := true
		repoName := filepath.Base(absProjectPath)
		promptMode := types.PromptMode{
			Interactive:   ui.IsInteractive(),
			NoInteractive: false,
			Force:         force,
			CI:            os.Getenv("CI") != "",
		}
		for _, wt := range worktrees {
			ui.PrintStep("Removing workspace: " + wt.Branch)

			wtPreset := preset
			if wtPreset == "" {
				wtPreset = presetManager.Detect(wt.Path)
			}

			if wtPreset != "" {
				siteName := filepath.Base(wt.Path)
				if wt.Branch == defaultBranch && cfg.SiteName != "" {
					siteName = cfg.SiteName
				}
				if err := scaffoldManager.RunCleanup(wt.Path, wt.Branch, repoName, siteName, wtPreset, cfg, info.BarePath, promptMode, false, verbose, quiet); err != nil {
					ui.PrintWarning(fmt.Sprintf("Cleanup failed for %s: %v", wt.Branch, err))
				} else {
					allCleanupFailed = false
				}
			} else {
				allCleanupFailed = false
			}

			if err := wsManager.RemoveWorkspace(wt.Path, true); err != nil {
				ui.PrintWarning(fmt.Sprintf("Failed to remove workspace %s: %v", wt.Branch, err))
			}

			// Branch deletion: only in worktree mode (CoW clones own their branches)
			if info.Mode == workspace.ModeWorktree {
				if err := git.DeleteBranch(info.BarePath, wt.Branch, true); err != nil {
					ui.PrintWarning(fmt.Sprintf("Failed to delete branch %s: %v", wt.Branch, err))
				}
			}

			ui.PrintSuccess(fmt.Sprintf("Removed %s", wt.Branch))
		}

		if allCleanupFailed && len(worktrees) > 0 {
			ui.PrintWarning("All cleanup steps failed. This may indicate a serious issue. Aborting.")
			return fmt.Errorf("all cleanup operations failed")
		}

		// Prune stale worktree refs: only applicable in worktree mode
		if info.Mode == workspace.ModeWorktree {
			if err := git.PruneWorktrees(info.BarePath); err != nil {
				ui.PrintWarning(fmt.Sprintf("Failed to prune worktrees: %v", err))
			}
		}

		ui.PrintStep("Deleting project folder...")
		if err := os.RemoveAll(absProjectPath); err != nil {
			return fmt.Errorf("deleting project folder: %w", err)
		}

		ui.PrintDone(fmt.Sprintf("Destroyed project %q", projectName))
		return nil
	},
}

func sortWorktreesForDestroy(worktrees []git.Worktree, defaultBranch string) []git.Worktree {
	sort.SliceStable(worktrees, func(i, j int) bool {
		iIsMain := worktrees[i].Branch == defaultBranch
		jIsMain := worktrees[j].Branch == defaultBranch
		if iIsMain != jIsMain {
			return !iIsMain
		}
		return worktrees[i].Branch < worktrees[j].Branch
	})
	return worktrees
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
