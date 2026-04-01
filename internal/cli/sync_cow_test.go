package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCowProjectWithRemote creates a CoW-mode project where main/
// has a configured origin remote so sync can fetch.
func createTestCowProjectWithRemote(t *testing.T) (projectRoot, mainPath string) {
	t.Helper()

	sourceDir := filepath.Join(t.TempDir(), "source")
	createTestSourceRepo(t, sourceDir)

	projectRoot = filepath.Join(t.TempDir(), "project")
	mainPath = filepath.Join(projectRoot, "main")

	require.NoError(t, os.MkdirAll(projectRoot, 0755))

	// Clone — this sets up origin automatically
	cmd := exec.Command("git", "clone", sourceDir, mainPath)
	require.NoError(t, cmd.Run())

	// Create .arbor/ marker
	arborDir := filepath.Join(projectRoot, ".arbor")
	require.NoError(t, os.MkdirAll(arborDir, 0755))
	metadata := "workspace_mode: cow\nremote_url: " + sourceDir + "\ndefault_branch: main\n"
	require.NoError(t, os.WriteFile(filepath.Join(arborDir, "metadata.yaml"), []byte(metadata), 0644))

	// Write arbor.yaml
	cfg := "site_name: test\ndefault_branch: main\nworkspace_mode: cow\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "arbor.yaml"), []byte(cfg), 0644))

	return projectRoot, mainPath
}

func TestSyncCmd_CowMode_FetchUsesWorkspaceDir(t *testing.T) {
	_, mainPath := createTestCowProjectWithRemote(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().StringP("upstream", "u", "main", "")
	cmd.Flags().StringP("strategy", "s", "rebase", "")
	cmd.Flags().StringP("remote", "r", "origin", "")
	cmd.Flags().Bool("save", false, "")
	cmd.Flags().BoolP("yes", "y", true, "") // skip all prompts
	cmd.Flags().Bool("no-auto-stash", false, "")
	cmd.Flags().Bool("dry-run", true, "") // dry-run so we don't actually change anything
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	// In CoW mode, syncing the main branch with itself should work without error
	err = syncCmd.RunE(cmd, []string{})
	// Dry-run should always succeed
	assert.NoError(t, err)
}

func TestSyncCmd_CowMode_MustBeInWorkspace(t *testing.T) {
	projectRoot, _ := createTestCowProjectWithRemote(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	// Run from the project root — not inside a workspace
	require.NoError(t, os.Chdir(projectRoot))

	cmd := &cobra.Command{}
	cmd.Flags().StringP("upstream", "u", "main", "")
	cmd.Flags().StringP("strategy", "s", "rebase", "")
	cmd.Flags().StringP("remote", "r", "origin", "")
	cmd.Flags().Bool("save", false, "")
	cmd.Flags().BoolP("yes", "y", true, "")
	cmd.Flags().Bool("no-auto-stash", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = syncCmd.RunE(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sync must be run from within a worktree")
}
