package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/artisanexperiences/arbor/internal/config"
)

// --- setupWorktreeProjectWithFeatures creates a worktree-mode project with
// main and feature workspaces for switch tests.
func setupWorktreeProjectWithFeatures(t *testing.T) (projectRoot, barePath, mainPath string) {
	t.Helper()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	projectRoot = filepath.Join(tmpDir, "project")
	barePath = filepath.Join(projectRoot, ".bare")
	mainPath = filepath.Join(projectRoot, "main")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = sourceDir
		require.NoError(t, cmd.Run())
	}
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("test"), 0644))
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = sourceDir
		require.NoError(t, cmd.Run())
	}

	require.NoError(t, os.MkdirAll(projectRoot, 0755))

	// Bare clone
	cmd := exec.Command("git", "clone", "--bare", sourceDir, barePath)
	require.NoError(t, cmd.Run())

	// Configure remote
	cmd = exec.Command("git", "-C", barePath, "config", "remote.origin.url", sourceDir)
	require.NoError(t, cmd.Run())

	// Main worktree
	cmd = exec.Command("git", "-C", barePath, "worktree", "add", mainPath, "main")
	require.NoError(t, cmd.Run())

	// Write arbor.yaml with worktree mode (the default)
	cfg := &config.Config{
		SiteName:      "test-project",
		DefaultBranch: "main",
	}
	require.NoError(t, config.SaveProject(projectRoot, cfg))

	return projectRoot, barePath, mainPath
}

// --- performSwitch helper ---

func TestPerformSwitch_WorktreeToCow_CreatesArborDir(t *testing.T) {
	projectRoot, _, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "cow", true) // force = true to skip confirmation
	require.NoError(t, err)

	// .arbor/ directory should now exist
	arborDir := filepath.Join(projectRoot, ".arbor")
	assert.DirExists(t, arborDir, ".arbor directory should be created after switching to CoW")
}

func TestPerformSwitch_WorktreeToCow_RemovesBareDir(t *testing.T) {
	projectRoot, barePath, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	_ = barePath // just for clarity

	err = performSwitch(projectRoot, "cow", true)
	require.NoError(t, err)

	// .bare/ directory should be gone
	barePath2 := filepath.Join(projectRoot, ".bare")
	_, statErr := os.Stat(barePath2)
	assert.True(t, os.IsNotExist(statErr), ".bare directory should be removed after switching to CoW")
}

func TestPerformSwitch_WorktreeToCow_MainWorkspaceIsNormalClone(t *testing.T) {
	projectRoot, _, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "cow", true)
	require.NoError(t, err)

	// After switch, main/ should have a .git DIRECTORY (normal clone)
	mainAfterSwitch := filepath.Join(projectRoot, "main")
	gitDir := filepath.Join(mainAfterSwitch, ".git")
	info, statErr := os.Stat(gitDir)
	require.NoError(t, statErr, ".git should exist in main workspace after CoW switch")
	assert.True(t, info.IsDir(), ".git should be a directory (normal clone), not a file (worktree pointer)")
}

func TestPerformSwitch_WorktreeToCow_UpdatesConfig(t *testing.T) {
	projectRoot, _, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "cow", true)
	require.NoError(t, err)

	cfg, err := config.LoadProject(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, "cow", cfg.WorkspaceMode)
}

func TestPerformSwitch_CowToWorktree_CreatesBareDir(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "worktree", true)
	require.NoError(t, err)

	// .bare/ should now exist
	barePath := filepath.Join(projectRoot, ".bare")
	assert.DirExists(t, barePath, ".bare directory should be created after switching to worktree mode")
}

func TestPerformSwitch_CowToWorktree_RemovesArborDir(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "worktree", true)
	require.NoError(t, err)

	// .arbor/ should be gone
	arborDir := filepath.Join(projectRoot, ".arbor")
	_, statErr := os.Stat(arborDir)
	assert.True(t, os.IsNotExist(statErr), ".arbor directory should be removed after switching to worktree mode")
}

func TestPerformSwitch_CowToWorktree_MainWorkspaceIsLinkedWorktree(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "worktree", true)
	require.NoError(t, err)

	// After switch, main/ should have a .git FILE (worktree pointer), not a directory
	mainAfterSwitch := filepath.Join(projectRoot, "main")
	gitPath := filepath.Join(mainAfterSwitch, ".git")
	info, statErr := os.Stat(gitPath)
	require.NoError(t, statErr, ".git should exist in main workspace after worktree switch")
	assert.False(t, info.IsDir(), ".git should be a file (worktree pointer), not a directory (normal clone)")
}

func TestPerformSwitch_CowToWorktree_UpdatesConfig(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "worktree", true)
	require.NoError(t, err)

	cfg, err := config.LoadProject(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, "worktree", cfg.WorkspaceMode)
}

func TestPerformSwitch_AlreadyInTargetMode_Errors(t *testing.T) {
	projectRoot, _, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	// Trying to switch to worktree mode when already in worktree mode
	err = performSwitch(projectRoot, "worktree", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already using worktree mode")
}

func TestPerformSwitch_InvalidMode_Errors(t *testing.T) {
	projectRoot, _, mainPath := setupWorktreeProjectWithFeatures(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()
	require.NoError(t, os.Chdir(mainPath))

	err = performSwitch(projectRoot, "invalid-mode", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target mode")
}
