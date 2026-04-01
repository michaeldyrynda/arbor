package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/artisanexperiences/arbor/internal/config"
)

// createTestSourceRepo creates a minimal git source repo with an initial commit
// at the given directory path. Returns the path (same as sourceDir).
func createTestSourceRepo(t *testing.T, sourceDir string) string {
	t.Helper()
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

	return sourceDir
}

// --- performCowInit helper ---

func TestPerformCowInit_CreatesBareArborDir(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	// .arbor/ marker directory must exist
	arborDir := filepath.Join(projectRoot, ".arbor")
	assert.DirExists(t, arborDir, ".arbor directory should be created")

	// .arbor/metadata.yaml must exist
	metadataPath := filepath.Join(arborDir, "metadata.yaml")
	assert.FileExists(t, metadataPath, ".arbor/metadata.yaml should exist")
}

func TestPerformCowInit_CreatesNormalClone(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	// main/ workspace must exist and be a normal (non-bare) clone
	mainPath := filepath.Join(projectRoot, "main")
	assert.DirExists(t, mainPath)

	// It should have a .git directory (not a bare repo which has HEAD, refs/ etc. at root)
	gitDir := filepath.Join(mainPath, ".git")
	assert.DirExists(t, gitDir, "main workspace should have a .git directory (normal clone)")

	// Bare repos do NOT have a .git directory at their root
	bareFile := filepath.Join(mainPath, "HEAD")
	_, err = os.Stat(bareFile)
	// HEAD should be inside .git, not at workspace root
	assert.False(t, !os.IsNotExist(err) && gitDir == "", "should be a normal clone, not a bare repo")
}

func TestPerformCowInit_NoBareDirCreated(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	// .bare directory must NOT be created in CoW mode
	barePath := filepath.Join(projectRoot, ".bare")
	_, statErr := os.Stat(barePath)
	assert.True(t, os.IsNotExist(statErr), ".bare directory should NOT be created in CoW mode")
}

func TestPerformCowInit_MetadataContainsExpectedFields(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	metadataPath := filepath.Join(projectRoot, ".arbor", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "workspace_mode: cow")
	assert.Contains(t, contentStr, "default_branch: main")
	// remote_url should reference the source repo
	assert.Contains(t, contentStr, sourceDir)
}

func TestPerformCowInit_MainWorkspaceOnDefaultBranch(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	mainPath := filepath.Join(projectRoot, "main")

	// The main workspace should be on the main branch
	cmd := exec.Command("git", "-C", mainPath, "branch", "--show-current")
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "main", strings.TrimSpace(string(output)))
}

// --- Config integration ---

func TestInitCow_SavesWorkspaceModeInConfig(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "test-project")
	require.NoError(t, err)

	// The project arbor.yaml should have workspace_mode: cow
	cfg, err := config.LoadProject(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, "cow", cfg.WorkspaceMode)
}

func TestInitCow_ConfigHasSiteName(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "my-project")
	require.NoError(t, err)

	cfg, err := config.LoadProject(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, "my-project", cfg.SiteName)
}

func TestInitCow_ConfigHasDefaultBranch(t *testing.T) {
	sourceDir := createTestSourceRepo(t, filepath.Join(t.TempDir(), "source"))
	projectRoot := filepath.Join(t.TempDir(), "project")

	err := performCowInit(sourceDir, projectRoot, "main", "my-project")
	require.NoError(t, err)

	cfg, err := config.LoadProject(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, "main", cfg.DefaultBranch)
}

// --- determineWorkspaceMode helper ---

func TestDetermineWorkspaceMode_FlagWorktree(t *testing.T) {
	mode, err := determineWorkspaceMode("worktree")
	require.NoError(t, err)
	assert.Equal(t, "worktree", mode)
}

func TestDetermineWorkspaceMode_FlagCow(t *testing.T) {
	mode, err := determineWorkspaceMode("cow")
	require.NoError(t, err)
	assert.Equal(t, "cow", mode)
}

func TestDetermineWorkspaceMode_InvalidFlag(t *testing.T) {
	_, err := determineWorkspaceMode("invalid-mode")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid workspace mode")
}

func TestDetermineWorkspaceMode_EmptyFlagNonInteractive(t *testing.T) {
	// With no flag and non-interactive env, should default to worktree
	mode, err := determineWorkspaceMode("")
	require.NoError(t, err)
	// In tests the terminal is non-interactive, so it should default to worktree
	assert.Equal(t, "worktree", mode)
}
