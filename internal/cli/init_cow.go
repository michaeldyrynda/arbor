package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/artisanexperiences/arbor/internal/config"
	"github.com/artisanexperiences/arbor/internal/ui"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

// cloneNormal clones repoURL as a regular (non-bare) git repository to dst.
func cloneNormal(repoURL, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	cmd := exec.Command("git", "clone", repoURL, dst)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}
	return nil
}

// determineWorkspaceMode returns the workspace mode to use for a new project.
//
//   - If modeFlag is non-empty it is validated and returned directly.
//   - If the terminal is interactive the user is prompted.
//   - Otherwise "worktree" is returned as the default.
func determineWorkspaceMode(modeFlag string) (string, error) {
	if modeFlag != "" {
		switch modeFlag {
		case workspace.ModeWorktree, workspace.ModeCow:
			return modeFlag, nil
		default:
			return "", fmt.Errorf("invalid workspace mode %q: must be %q or %q",
				modeFlag, workspace.ModeWorktree, workspace.ModeCow)
		}
	}

	if ui.IsInteractive() {
		return ui.PromptWorkspaceMode()
	}

	// Non-interactive default: worktree mode (backward compatible)
	return workspace.ModeWorktree, nil
}

// performCowInit initialises a new project in copy-on-write mode:
//  1. Clones repoURL as a normal (non-bare) clone to <projectRoot>/<defaultBranch>/
//  2. Creates the <projectRoot>/.arbor/ marker directory with metadata
//  3. Writes arbor.yaml to projectRoot with workspace_mode: cow
func performCowInit(repoURL, projectRoot, defaultBranch, siteName string) error {
	mainPath := filepath.Join(projectRoot, defaultBranch)

	// Clone as a normal (non-bare) repo
	ui.PrintStep(fmt.Sprintf("Cloning %s to %s (copy-on-write mode)...", repoURL, mainPath))
	if err := cloneNormal(repoURL, mainPath); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Cloned to %s", mainPath))

	// Create .arbor/ marker directory
	arborDir := filepath.Join(projectRoot, ".arbor")
	if err := os.MkdirAll(arborDir, 0755); err != nil {
		return fmt.Errorf("creating .arbor directory: %w", err)
	}

	// Write metadata.yaml
	metadata := fmt.Sprintf("workspace_mode: cow\nremote_url: %s\ndefault_branch: %s\n",
		repoURL, defaultBranch)
	metadataPath := filepath.Join(arborDir, "metadata.yaml")
	if err := os.WriteFile(metadataPath, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("writing .arbor/metadata.yaml: %w", err)
	}
	ui.PrintSuccess("Created .arbor/ project marker")

	// Warn if CoW is not natively supported on this filesystem
	if supported, err := workspace.DetectCowSupport(projectRoot); err == nil && !supported {
		ui.PrintWarning(workspace.CowSupportWarning())
	}

	// Write arbor.yaml to project root
	cfg := &config.Config{
		SiteName:      siteName,
		DefaultBranch: defaultBranch,
		WorkspaceMode: workspace.ModeCow,
	}
	if err := config.SaveProject(projectRoot, cfg); err != nil {
		return fmt.Errorf("saving project config: %w", err)
	}
	ui.PrintSuccess("Saved arbor.yaml (workspace_mode: cow)")

	return nil
}
