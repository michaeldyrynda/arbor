package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/artisanexperiences/arbor/internal/git"
	"github.com/artisanexperiences/arbor/internal/scaffold/types"
	"github.com/artisanexperiences/arbor/internal/ui"
	"github.com/artisanexperiences/arbor/internal/utils"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

var workCmd = &cobra.Command{
	Use:   "work [BRANCH] [PATH]",
	Short: "Create or checkout a feature worktree",
	Long: `Creates or checks out a new worktree for a feature branch.

Arguments:
  BRANCH  Name of the feature branch
  PATH    Optional custom path (defaults to sanitised branch name)

If no branch is provided, interactive mode allows selection from
available branches or entering a new branch name.`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		pc, err := OpenProjectFromCWD()
		if err != nil {
			return err
		}

		baseBranch := mustGetString(cmd, "base")
		dryRun := mustGetBool(cmd, "dry-run")
		verbose := mustGetBool(cmd, "verbose")
		quiet := mustGetBool(cmd, "quiet")
		skipScaffold := mustGetBool(cmd, "skip-scaffold")

		var branch string
		if len(args) > 0 {
			branch = args[0]
		} else if ui.IsInteractive() {
			// For branch listing we need a git repo path.
			// In worktree mode use .bare; in CoW mode use the default-branch workspace.
			var gitRefPath string
			if pc.Mode == workspace.ModeCow {
				gitRefPath = filepath.Join(pc.ProjectPath, pc.DefaultBranch)
			} else {
				gitRefPath = pc.BarePath
			}

			localBranches, err := git.ListAllBranches(gitRefPath)
			if err != nil {
				return fmt.Errorf("listing local branches: %w", err)
			}

			remoteBranches, _ := git.ListRemoteBranches(gitRefPath)

			selected, err := ui.SelectBranchInteractive(gitRefPath, localBranches, remoteBranches)
			if err != nil {
				return fmt.Errorf("selecting branch: %w", err)
			}
			branch = selected
		}

		if branch == "" {
			return fmt.Errorf("branch name required (run interactively or provide branch as argument)")
		}

		// If the selected branch is a remote ref (e.g. "origin/feature/foo"), strip the
		// remote prefix to derive the local branch name and use the remote ref as the
		// base so that CreateWorktree creates a proper local tracking branch rather than
		// a detached-HEAD worktree.
		if idx := strings.IndexByte(branch, '/'); idx != -1 {
			remote := branch[:idx]
			localBranch := branch[idx+1:]
			// Only treat it as a remote ref when the prefix matches a known remote.
			// In worktree mode use .bare; in CoW mode use the default-branch workspace.
			var gitRefPath string
			if pc.Mode == workspace.ModeCow {
				gitRefPath = filepath.Join(pc.ProjectPath, pc.DefaultBranch)
			} else {
				gitRefPath = pc.BarePath
			}
			remotes, _ := git.ListRemotes(gitRefPath)
			for _, r := range remotes {
				if r == remote {
					if baseBranch == "" {
						baseBranch = branch // use the full remote ref as the base
					}
					branch = localBranch
					break
				}
			}
		}

		if baseBranch == "" {
			baseBranch = pc.DefaultBranch
		}

		worktreePath := ""
		if len(args) > 1 {
			worktreePath = args[1]
		} else {
			worktreePath = filepath.Join(pc.ProjectPath, utils.SanitisePath(branch))
		}

		absWorktreePath, err := filepath.Abs(worktreePath)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}

		// Check whether a workspace already exists for this branch
		existingWorkspaces, err := pc.WorkspaceManager().ListWorkspaces()
		if err != nil {
			return fmt.Errorf("listing workspaces: %w", err)
		}
		for _, ws := range existingWorkspaces {
			if ws.Branch == branch {
				ui.PrintInfo(fmt.Sprintf("Workspace already exists at %s", ws.Path))
				return nil
			}
		}

		ui.PrintStep(fmt.Sprintf("Creating workspace for branch '%s' from '%s'", branch, baseBranch))
		ui.PrintInfo(fmt.Sprintf("Path: %s", absWorktreePath))

		if !dryRun {
			if _, err := pc.WorkspaceManager().CreateWorkspace(branch, baseBranch, absWorktreePath); err != nil {
				return fmt.Errorf("creating workspace: %w", err)
			}
		} else {
			ui.PrintInfo("[DRY RUN] Would create workspace")
		}

		// Set up branch tracking for worktree mode only (CoW clones track independently)
		noTrack := mustGetBool(cmd, "no-track")
		if !dryRun && !noTrack && pc.Mode == workspace.ModeWorktree {
			if err := git.SetBranchUpstream(pc.BarePath, branch, "origin"); err != nil {
				// Non-fatal - just inform user if verbose
				if verbose {
					ui.PrintInfo(fmt.Sprintf("Could not set up tracking for branch '%s': %v", branch, err))
				}
			} else {
				ui.PrintSuccess(fmt.Sprintf("Set up tracking for branch '%s' on origin", branch))
			}
		}

		if !dryRun {
			if !skipScaffold {
				preset := pc.Config.Preset
				if preset == "" {
					preset = pc.PresetManager().Detect(absWorktreePath)
				}

				if verbose && preset != "" {
					ui.PrintInfo(fmt.Sprintf("Running scaffold for preset: %s", preset))
				}

				repoName := filepath.Base(filepath.Dir(absWorktreePath))
				folderName := filepath.Base(absWorktreePath)

				// For the default branch, use the saved SiteName from project config
				// For feature branches, use the worktree folder name
				siteName := folderName
				if branch == pc.DefaultBranch && pc.Config.SiteName != "" {
					siteName = pc.Config.SiteName
				}

				promptMode := types.PromptMode{
					Interactive:   ui.IsInteractive(),
					NoInteractive: false,
					Force:         false,
					CI:            os.Getenv("CI") != "",
				}
				if err := pc.ScaffoldManager().RunScaffold(absWorktreePath, branch, repoName, siteName, preset, pc.Config, pc.BarePath, promptMode, false, verbose, quiet); err != nil {
					ui.PrintErrorWithHint("Scaffold steps failed", err.Error())
				}
			} else {
				ui.PrintInfo("Skipped scaffold (use 'arbor scaffold <branch>' to scaffold manually)")
			}

			// Check if .arbor.local should be gitignored
			if !quiet {
				checkArborLocalGitignore(absWorktreePath)
			}
		} else {
			ui.PrintInfo("[DRY RUN] Would run scaffold steps")
		}

		ui.PrintDone(fmt.Sprintf("Worktree ready at %s", absWorktreePath))
		return nil
	},
}

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func init() {
	rootCmd.AddCommand(workCmd)

	workCmd.Flags().StringP("base", "b", "", "Base branch for new worktree")
	workCmd.Flags().Bool("no-track", false, "Skip setting up remote tracking for new branches")
	workCmd.Flags().Bool("skip-scaffold", false, "Skip scaffold steps during work")
}
