package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/artisanexperiences/arbor/internal/config"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

// createTestCowProject creates a CoW-mode project in tmpDir:
//
//	<tmpDir>/
//	  .arbor/metadata.yaml
//	  arbor.yaml
//	  main/   (normal clone)
//
// Returns (projectRoot, mainWorkspacePath).
func createTestCowProjectForCLI(t *testing.T) (projectRoot, mainPath string) {
	t.Helper()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	projectRoot = filepath.Join(tmpDir, "project")
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

	// Clone as a normal repo
	cmd := exec.Command("git", "clone", sourceDir, mainPath)
	require.NoError(t, cmd.Run())

	// Create .arbor/ marker
	arborDir := filepath.Join(projectRoot, ".arbor")
	require.NoError(t, os.MkdirAll(arborDir, 0755))
	metadata := "workspace_mode: cow\nremote_url: " + sourceDir + "\ndefault_branch: main\n"
	require.NoError(t, os.WriteFile(filepath.Join(arborDir, "metadata.yaml"), []byte(metadata), 0644))

	// Write arbor.yaml
	cfg := &config.Config{
		SiteName:      "test-project",
		DefaultBranch: "main",
		WorkspaceMode: "cow",
	}
	require.NoError(t, config.SaveProject(projectRoot, cfg))

	return projectRoot, mainPath
}

// --- OpenProjectFromCWD in CoW mode ---

func TestOpenProjectFromCWD_CowMode_Success(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)
	assert.NotNil(t, pc)
}

func TestOpenProjectFromCWD_CowMode_WorkspaceModeIsCow(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)
	assert.Equal(t, "cow", pc.Config.WorkspaceMode)
}

func TestOpenProjectFromCWD_CowMode_BarePathIsEmpty(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)
	// CoW mode has no bare path
	assert.Empty(t, pc.BarePath)
}

func TestOpenProjectFromCWD_CowMode_ProjectPathIsCorrect(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)

	expectedRoot, _ := filepath.EvalSymlinks(projectRoot)
	actualRoot, _ := filepath.EvalSymlinks(pc.ProjectPath)
	assert.Equal(t, expectedRoot, actualRoot)
}

func TestOpenProjectFromCWD_CowMode_DefaultBranchIsMain(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)
	assert.Equal(t, "main", pc.DefaultBranch)
}

// --- IsInWorktree in CoW mode ---

func TestProjectContext_IsInWorktree_CowMode_InsideWorkspace(t *testing.T) {
	projectRoot, mainPath := createTestCowProjectForCLI(t)

	pc := &ProjectContext{
		Mode:        workspace.ModeCow,
		CWD:         mainPath,
		ProjectPath: projectRoot,
	}

	// Should return true when inside a CoW workspace
	assert.True(t, pc.IsInWorktree(), "IsInWorktree() should be true when inside a CoW workspace")
}

func TestProjectContext_IsInWorktree_CowMode_AtProjectRoot(t *testing.T) {
	projectRoot, _ := createTestCowProjectForCLI(t)

	pc := &ProjectContext{
		Mode:        workspace.ModeCow,
		CWD:         projectRoot,
		ProjectPath: projectRoot,
	}

	// Should return false at the project root itself
	assert.False(t, pc.IsInWorktree(), "IsInWorktree() should be false at project root")
}

func TestProjectContext_IsInWorktree_CowMode_InsideArborDir(t *testing.T) {
	projectRoot, _ := createTestCowProjectForCLI(t)

	pc := &ProjectContext{
		Mode:        workspace.ModeCow,
		CWD:         filepath.Join(projectRoot, ".arbor"),
		ProjectPath: projectRoot,
	}

	// Should return false inside the .arbor marker directory
	assert.False(t, pc.IsInWorktree(), "IsInWorktree() should be false inside .arbor directory")
}

// --- WorkspaceManager ---

func TestProjectContext_WorkspaceManager_CowMode(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)

	wsm := pc.WorkspaceManager()
	require.NotNil(t, wsm)
	assert.Equal(t, "cow", wsm.Mode)
}

func TestProjectContext_WorkspaceManager_WorktreeMode(t *testing.T) {
	worktreePath, _ := createTestWorktree(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(worktreePath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)

	wsm := pc.WorkspaceManager()
	require.NotNil(t, wsm)
	assert.Equal(t, "worktree", wsm.Mode)
}

func TestProjectContext_WorkspaceManager_IsSingleton(t *testing.T) {
	_, mainPath := createTestCowProjectForCLI(t)

	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalCWD) }()

	require.NoError(t, os.Chdir(mainPath))

	pc, err := OpenProjectFromCWD()
	require.NoError(t, err)

	wsm1 := pc.WorkspaceManager()
	wsm2 := pc.WorkspaceManager()
	assert.Same(t, wsm1, wsm2, "WorkspaceManager() should return the same instance on multiple calls")
}
