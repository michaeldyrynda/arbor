package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/artisanexperiences/arbor/internal/config"
	"github.com/artisanexperiences/arbor/internal/git"
	"github.com/artisanexperiences/arbor/internal/ui"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

var switchCmd = &cobra.Command{
	Use:   "switch [MODE]",
	Short: "Switch workspace mode between worktree and copy-on-write",
	Long: `Converts the current project between worktree mode and copy-on-write (CoW) mode.

  arbor switch cow        — convert a worktree project to CoW mode
  arbor switch worktree   — convert a CoW project back to worktree mode

The switch operation:
  1. Identifies all feature workspaces
  2. Confirms removal (feature workspaces cannot be migrated; recreate with 'arbor work')
  3. Converts the project layout to the target mode
  4. Updates arbor.yaml with the new workspace_mode

Feature workspaces are removed during the switch. After switching, use
'arbor work <branch>' to recreate them in the new mode.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetMode := args[0]
		force := mustGetBool(cmd, "force")

		pc, err := OpenProjectFromCWD()
		if err != nil {
			return err
		}

		return performSwitch(pc.ProjectPath, targetMode, force)
	},
}

func init() {
	rootCmd.AddCommand(switchCmd)
	switchCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
}

// performSwitch converts projectRoot from its current workspace mode to targetMode.
// force=true skips user confirmation prompts.
func performSwitch(projectRoot, targetMode string, force bool) error {
	if targetMode != workspace.ModeWorktree && targetMode != workspace.ModeCow {
		return fmt.Errorf("invalid target mode %q: must be %q or %q",
			targetMode, workspace.ModeWorktree, workspace.ModeCow)
	}

	// Detect current mode
	info, err := workspace.FindProjectRoot(projectRoot)
	if err != nil {
		return fmt.Errorf("finding project: %w", err)
	}

	if info.Mode == targetMode {
		return fmt.Errorf("already using %s mode", targetMode)
	}

	cfg, err := config.LoadProject(projectRoot)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
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

	// Identify feature workspaces that will be removed
	var featureWorkspaces []workspace.Workspace
	for _, ws := range workspaces {
		if ws.Branch != defaultBranch {
			featureWorkspaces = append(featureWorkspaces, ws)
		}
	}

	if len(featureWorkspaces) > 0 {
		ui.PrintWarning(fmt.Sprintf("The following %d feature workspace(s) will be removed during the switch:", len(featureWorkspaces)))
		for _, ws := range featureWorkspaces {
			ui.PrintInfo(fmt.Sprintf("  - %s (%s)", ws.Branch, ws.Path))
		}
		ui.PrintInfo("You can recreate them with 'arbor work <branch>' after switching.")

		if !force {
			confirmed, err := ui.Confirm(fmt.Sprintf("Remove %d feature workspace(s) and switch to %s mode?", len(featureWorkspaces), targetMode))
			if err != nil {
				return fmt.Errorf("confirmation: %w", err)
			}
			if !confirmed {
				ui.PrintInfo("Switch cancelled.")
				return nil
			}
		}

		// Remove feature workspaces
		for _, ws := range featureWorkspaces {
			ui.PrintStep(fmt.Sprintf("Removing workspace %s...", ws.Branch))
			if err := wsManager.RemoveWorkspace(ws.Path, true); err != nil {
				ui.PrintWarning(fmt.Sprintf("Failed to remove %s: %v", ws.Branch, err))
			}
		}
	}

	// Get the remote URL before we tear down the current structure
	remoteURL, err := getRemoteURLForSwitch(info, defaultBranch)
	if err != nil {
		return fmt.Errorf("getting remote URL: %w", err)
	}
	if remoteURL == "" {
		return fmt.Errorf("could not determine remote URL; ensure 'origin' is configured")
	}

	switch targetMode {
	case workspace.ModeCow:
		return switchToCoW(projectRoot, info, defaultBranch, remoteURL, cfg)
	case workspace.ModeWorktree:
		return switchToWorktree(projectRoot, info, defaultBranch, remoteURL, cfg)
	default:
		panic("unreachable: mode already validated above")
	}
}

// getRemoteURLForSwitch retrieves the origin URL depending on the current mode.
func getRemoteURLForSwitch(info *workspace.ProjectInfo, defaultBranch string) (string, error) {
	switch info.Mode {
	case workspace.ModeWorktree:
		url, err := git.GetRemoteURL(info.BarePath, "origin")
		if err != nil {
			return "", fmt.Errorf("reading remote from .bare: %w", err)
		}
		return url, nil
	case workspace.ModeCow:
		mainPath := filepath.Join(info.ProjectRoot, defaultBranch)
		url, err := git.GetRemoteURLFromWorktree(mainPath)
		if err != nil {
			return "", fmt.Errorf("reading remote from workspace: %w", err)
		}
		return url, nil
	default:
		return "", fmt.Errorf("unknown mode: %q", info.Mode)
	}
}

// switchToCoW converts a worktree-mode project to CoW mode:
//  1. Remove the main git worktree (git worktree remove)
//  2. Create a normal clone in its place
//  3. Remove .bare
//  4. Create .arbor/ marker
//  5. Update arbor.yaml
func switchToCoW(projectRoot string, info *workspace.ProjectInfo, defaultBranch, remoteURL string, cfg *config.Config) error {
	mainPath := filepath.Join(projectRoot, defaultBranch)
	barePath := info.BarePath
	tmpMainPath := mainPath + "-switch-tmp"

	ui.PrintStep(fmt.Sprintf("Switching to copy-on-write mode (remote: %s)...", remoteURL))

	// Clone a normal repo to a temp path (don't overwrite mainPath yet)
	ui.PrintInfo("Cloning normal repository...")
	cmd := exec.Command("git", "clone", remoteURL, tmpMainPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cloning repository: %w\n%s", err, string(output))
	}

	// Remove the worktree-mode main worktree from git
	ui.PrintInfo("Removing worktree-mode main workspace...")
	if err := git.RemoveWorktree(mainPath, true); err != nil {
		// Try force removal of the directory if git worktree remove fails
		_ = os.RemoveAll(mainPath)
	}

	// Move the normal clone into place
	if err := os.Rename(tmpMainPath, mainPath); err != nil {
		return fmt.Errorf("moving clone into place: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Created normal clone at %s", mainPath))

	// Remove .bare
	if err := os.RemoveAll(barePath); err != nil {
		return fmt.Errorf("removing .bare: %w", err)
	}
	ui.PrintSuccess("Removed .bare directory")

	// Create .arbor/ marker
	arborDir := filepath.Join(projectRoot, ".arbor")
	if err := os.MkdirAll(arborDir, 0755); err != nil {
		return fmt.Errorf("creating .arbor directory: %w", err)
	}
	metadata := fmt.Sprintf("workspace_mode: cow\nremote_url: %s\ndefault_branch: %s\n",
		remoteURL, defaultBranch)
	if err := os.WriteFile(filepath.Join(arborDir, "metadata.yaml"), []byte(metadata), 0644); err != nil {
		return fmt.Errorf("writing .arbor/metadata.yaml: %w", err)
	}
	ui.PrintSuccess("Created .arbor/ project marker")

	// Update arbor.yaml
	cfg.WorkspaceMode = workspace.ModeCow
	if err := config.SaveProject(projectRoot, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.PrintSuccess("Updated arbor.yaml (workspace_mode: cow)")

	// Warn about CoW filesystem support
	if supported, err := workspace.DetectCowSupport(projectRoot); err == nil && !supported {
		ui.PrintWarning(workspace.CowSupportWarning())
	}

	ui.PrintDone("Switched to copy-on-write mode. Use 'arbor work <branch>' to create workspaces.")
	return nil
}

// switchToWorktree converts a CoW-mode project to worktree mode:
//  1. Create a bare clone in .bare/
//  2. Configure fetch refspec on the bare repo
//  3. Remove the CoW main clone
//  4. Create a git worktree for the default branch
//  5. Remove .arbor/
//  6. Update arbor.yaml
func switchToWorktree(projectRoot string, info *workspace.ProjectInfo, defaultBranch, remoteURL string, cfg *config.Config) error {
	mainPath := filepath.Join(projectRoot, defaultBranch)
	barePath := filepath.Join(projectRoot, ".bare")
	arborDir := filepath.Join(projectRoot, ".arbor")

	ui.PrintStep(fmt.Sprintf("Switching to worktree mode (remote: %s)...", remoteURL))

	// Create bare clone
	ui.PrintInfo("Creating bare repository...")
	if err := git.CloneRepo(remoteURL, barePath); err != nil {
		return fmt.Errorf("creating bare repository: %w", err)
	}
	ui.PrintSuccess("Created bare repository at .bare/")

	// Configure fetch refspec (needed for branch tracking in bare repos)
	if err := git.ConfigureFetchRefspec(barePath, remoteURL); err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not configure fetch refspec: %v", err))
	}

	// Remove the CoW main clone
	ui.PrintInfo("Removing CoW main workspace...")
	if err := os.RemoveAll(mainPath); err != nil {
		return fmt.Errorf("removing CoW main workspace: %w", err)
	}

	// Create the main git worktree
	ui.PrintInfo(fmt.Sprintf("Creating main worktree at %s...", mainPath))
	if err := git.CreateWorktree(barePath, mainPath, defaultBranch, ""); err != nil {
		return fmt.Errorf("creating main worktree: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Created main worktree at %s", mainPath))

	// Remove .arbor/
	if err := os.RemoveAll(arborDir); err != nil {
		return fmt.Errorf("removing .arbor directory: %w", err)
	}
	ui.PrintSuccess("Removed .arbor/ directory")

	// Update arbor.yaml
	cfg.WorkspaceMode = workspace.ModeWorktree
	if err := config.SaveProject(projectRoot, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.PrintSuccess("Updated arbor.yaml (workspace_mode: worktree)")

	ui.PrintDone("Switched to worktree mode. Use 'arbor work <branch>' to create workspaces.")
	return nil
}
