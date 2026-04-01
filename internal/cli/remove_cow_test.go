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

// setupCowProjectWithFeature creates a CoW project with a feature workspace
// already created. Returns (projectRoot, mainPath, featurePath).
func setupCowProjectWithFeature(t *testing.T) (projectRoot, mainPath, featurePath string) {
	t.Helper()

	projectRoot, mainPath = createTestCowProjectForCLI(t)
	featurePath = filepath.Join(projectRoot, "feature-to-remove")

	// Create the feature workspace by copying main
	cmd := exec.Command("cp", "-R", mainPath+"/", featurePath+"/")
	require.NoError(t, cmd.Run())

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"checkout", "-b", "feature/to-remove"},
	} {
		c := exec.Command("git", args...)
		c.Dir = featurePath
		require.NoError(t, c.Run())
	}

	return projectRoot, mainPath, featurePath
}

func TestRemoveCmd_CowMode_RemovesWorkspace(t *testing.T) {
	_, mainPath, featurePath := setupCowProjectWithFeature(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().BoolP("force", "f", true, "")
	cmd.Flags().Bool("delete-branch", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = removeCmd.RunE(cmd, []string{"feature-to-remove"})
	require.NoError(t, err)

	// The workspace directory should be gone
	_, statErr := os.Stat(featurePath)
	assert.True(t, os.IsNotExist(statErr), "feature workspace should be removed")
}

func TestRemoveCmd_CowMode_PreventsMainRemoval(t *testing.T) {
	_, mainPath, _ := setupCowProjectWithFeature(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().BoolP("force", "f", true, "")
	cmd.Flags().Bool("delete-branch", false, "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = removeCmd.RunE(cmd, []string{"main"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove main")
}

func TestRemoveCmd_CowMode_DryRunDoesNotRemove(t *testing.T) {
	_, mainPath, featurePath := setupCowProjectWithFeature(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	cmd := &cobra.Command{}
	cmd.Flags().BoolP("force", "f", true, "")
	cmd.Flags().Bool("delete-branch", false, "")
	cmd.Flags().Bool("dry-run", true, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")

	err = removeCmd.RunE(cmd, []string{"feature-to-remove"})
	require.NoError(t, err)

	// Directory should still exist after dry-run
	assert.DirExists(t, featurePath, "feature workspace should still exist after dry-run")
}
