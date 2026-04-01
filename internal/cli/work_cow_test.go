package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkCmd_CowMode_CreatesWorkspace(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().StringP("base", "b", "", "")
	cmd.Flags().Bool("no-track", false, "")
	cmd.Flags().Bool("skip-scaffold", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = workCmd.RunE(cmd, []string{"feature/cow-test"})
	require.NoError(t, err)

	// CoW workspace directory should exist
	expectedPath := filepath.Join(projectRoot, "feature-cow-test")
	assert.DirExists(t, expectedPath, "CoW workspace directory should exist")

	// Should contain a .git directory (normal clone)
	assert.DirExists(t, filepath.Join(expectedPath, ".git"), "workspace should be a normal git clone")
}

func TestWorkCmd_CowMode_WorkspaceOnCorrectBranch(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().StringP("base", "b", "", "")
	cmd.Flags().Bool("no-track", false, "")
	cmd.Flags().Bool("skip-scaffold", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = workCmd.RunE(cmd, []string{"feature/branch-test"})
	require.NoError(t, err)

	expectedPath := filepath.Join(projectRoot, "feature-branch-test")
	branchCmd := exec.Command("git", "-C", expectedPath, "branch", "--show-current")
	output, err := branchCmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "feature/branch-test", strings.TrimSpace(string(output)))
}

func TestWorkCmd_CowMode_DryRunDoesNotCreate(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().StringP("base", "b", "", "")
	cmd.Flags().Bool("no-track", false, "")
	cmd.Flags().Bool("skip-scaffold", false, "")
	cmd.Flags().Bool("dry-run", true, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = workCmd.RunE(cmd, []string{"feature/dry-run-test"})
	require.NoError(t, err)

	// Directory should NOT be created in dry-run mode
	expectedPath := filepath.Join(projectRoot, "feature-dry-run-test")
	_, statErr := os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(statErr), "workspace should not be created in dry-run mode")
}
