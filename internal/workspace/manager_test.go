package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestWorktreeProject creates a bare-repo project layout matching the
// worktree mode structure:
//
//	<tmpDir>/
//	  .bare/   ← bare git repo
//	  main/    ← main worktree
func createTestWorktreeProject(t *testing.T) (projectRoot, barePath, mainPath string) {
	t.Helper()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	projectRoot = filepath.Join(tmpDir, "project")
	barePath = filepath.Join(projectRoot, ".bare")
	mainPath = filepath.Join(projectRoot, "main")

	// Set up a source repo with an initial commit
	for _, args := range [][]string{
		{"init", "-b", "main", sourceDir},
	} {
		cmd := exec.Command("git", args...)
		require.NoError(t, cmd.Run())
	}
	for _, args := range [][]string{
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

	// Clone to bare
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	cmd := exec.Command("git", "clone", "--bare", sourceDir, barePath)
	require.NoError(t, cmd.Run())

	// Create main worktree
	cmd = exec.Command("git", "-C", barePath, "worktree", "add", mainPath, "main")
	require.NoError(t, cmd.Run())

	return projectRoot, barePath, mainPath
}

// createTestCowProject creates a CoW-mode project layout:
//
//	<tmpDir>/
//	  .arbor/  ← CoW project marker
//	  main/    ← normal git clone (default branch)
func createTestCowProject(t *testing.T) (projectRoot, mainPath string) {
	t.Helper()

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	projectRoot = filepath.Join(tmpDir, "project")
	mainPath = filepath.Join(projectRoot, "main")

	// Set up a source repo with an initial commit
	for _, args := range [][]string{
		{"init", "-b", "main", sourceDir},
	} {
		cmd := exec.Command("git", args...)
		require.NoError(t, cmd.Run())
	}
	for _, args := range [][]string{
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

	// Normal clone as main workspace
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	cmd := exec.Command("git", "clone", sourceDir, mainPath)
	require.NoError(t, cmd.Run())

	// Create .arbor marker directory
	arborDir := filepath.Join(projectRoot, ".arbor")
	require.NoError(t, os.MkdirAll(arborDir, 0755))

	// Write metadata
	metadata := "workspace_mode: cow\nremote_url: " + sourceDir + "\ndefault_branch: main\n"
	require.NoError(t, os.WriteFile(filepath.Join(arborDir, "metadata.yaml"), []byte(metadata), 0644))

	return projectRoot, mainPath
}

// --- FindProjectRoot ---

func TestFindProjectRoot_WorktreeMode(t *testing.T) {
	projectRoot, barePath, mainPath := createTestWorktreeProject(t)

	// Should find from project root
	info, err := FindProjectRoot(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, ModeWorktree, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)
	assert.Equal(t, barePath, info.BarePath)

	// Should also find from inside a worktree
	info, err = FindProjectRoot(mainPath)
	require.NoError(t, err)
	assert.Equal(t, ModeWorktree, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)

	// Should find from a subdirectory within a worktree
	subDir := filepath.Join(mainPath, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	info, err = FindProjectRoot(subDir)
	require.NoError(t, err)
	assert.Equal(t, ModeWorktree, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)
}

func TestFindProjectRoot_CowMode(t *testing.T) {
	projectRoot, mainPath := createTestCowProject(t)

	// Should find from project root
	info, err := FindProjectRoot(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, ModeCow, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)
	assert.Empty(t, info.BarePath) // CoW mode has no bare path

	// Should also find from inside a workspace clone
	info, err = FindProjectRoot(mainPath)
	require.NoError(t, err)
	assert.Equal(t, ModeCow, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)

	// Should find from a subdirectory within a workspace
	subDir := filepath.Join(mainPath, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	info, err = FindProjectRoot(subDir)
	require.NoError(t, err)
	assert.Equal(t, ModeCow, info.Mode)
	assert.Equal(t, projectRoot, info.ProjectRoot)
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := FindProjectRoot(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arbor project not found")
}

func TestFindProjectRoot_WorktreeTakesPrecedenceOverCow(t *testing.T) {
	// If a directory somehow has both .bare and .arbor, .bare wins
	// (backward compatibility guard)
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	// Also create an .arbor marker
	arborDir := filepath.Join(projectRoot, ".arbor")
	require.NoError(t, os.MkdirAll(arborDir, 0755))

	info, err := FindProjectRoot(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, ModeWorktree, info.Mode)
	assert.Equal(t, barePath, info.BarePath)
}

// --- Manager creation ---

func TestNewManager_WorktreeMode(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	info := &ProjectInfo{
		Mode:        ModeWorktree,
		ProjectRoot: projectRoot,
		BarePath:    barePath,
	}
	m := NewManager(info, "main")

	assert.Equal(t, ModeWorktree, m.Mode)
	assert.Equal(t, projectRoot, m.ProjectRoot)
	assert.Equal(t, barePath, m.BarePath)
	assert.Equal(t, "main", m.DefaultBranch)
}

func TestNewManager_CowMode(t *testing.T) {
	projectRoot, _ := createTestCowProject(t)

	info := &ProjectInfo{
		Mode:        ModeCow,
		ProjectRoot: projectRoot,
		BarePath:    "",
	}
	m := NewManager(info, "main")

	assert.Equal(t, ModeCow, m.Mode)
	assert.Equal(t, projectRoot, m.ProjectRoot)
	assert.Empty(t, m.BarePath)
	assert.Equal(t, "main", m.DefaultBranch)
}

// --- ListWorkspaces ---

func TestManager_ListWorkspaces_WorktreeMode(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	require.Len(t, workspaces, 1)
	assert.Equal(t, "main", workspaces[0].Branch)
}

func TestManager_ListWorkspaces_WorktreeMode_MultipleWorkspaces(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	// Create a feature worktree
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("git", "-C", barePath, "worktree", "add", "-b", "feature/foo", featurePath, "main")
	require.NoError(t, cmd.Run())

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	assert.Len(t, workspaces, 2)

	branches := make(map[string]bool)
	for _, ws := range workspaces {
		branches[ws.Branch] = true
	}
	assert.True(t, branches["main"])
	assert.True(t, branches["feature/foo"])
}

func TestManager_ListWorkspaces_CowMode(t *testing.T) {
	projectRoot, _ := createTestCowProject(t)

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	require.Len(t, workspaces, 1)
	assert.Equal(t, "main", workspaces[0].Branch)
}

func TestManager_ListWorkspaces_CowMode_MultipleWorkspaces(t *testing.T) {
	projectRoot, mainPath := createTestCowProject(t)

	// Simulate a CoW clone by copying main (using regular copy for test portability)
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("cp", "-R", mainPath+"/", featurePath+"/")
	require.NoError(t, cmd.Run())

	// Switch to a new branch in the CoW clone
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"checkout", "-b", "feature/foo"},
	} {
		c := exec.Command("git", args...)
		c.Dir = featurePath
		require.NoError(t, c.Run())
	}

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	assert.Len(t, workspaces, 2)

	branches := make(map[string]bool)
	for _, ws := range workspaces {
		branches[ws.Branch] = true
	}
	assert.True(t, branches["main"])
	assert.True(t, branches["feature/foo"])
}

// --- CreateWorkspace ---

func TestManager_CreateWorkspace_WorktreeMode_NewBranch(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	wsPath, err := m.CreateWorkspace("feature/new", "main", "")
	require.NoError(t, err)

	// Workspace directory should exist
	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace directory should exist")

	// Branch should exist in bare repo
	cmd := exec.Command("git", "-C", barePath, "rev-parse", "--verify", "refs/heads/feature/new")
	assert.NoError(t, cmd.Run(), "feature branch should exist in bare repo")

	// Should be listed
	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	branches := make(map[string]bool)
	for _, ws := range workspaces {
		branches[ws.Branch] = true
	}
	assert.True(t, branches["feature/new"])
}

func TestManager_CreateWorkspace_WorktreeMode_ExistingBranch(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	// Pre-create a branch in the bare repo
	cmd := exec.Command("git", "-C", barePath, "branch", "feature/existing", "main")
	require.NoError(t, cmd.Run())

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	wsPath, err := m.CreateWorkspace("feature/existing", "main", "")
	require.NoError(t, err)

	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace directory should exist")
}

func TestManager_CreateWorkspace_CowMode_NewBranch(t *testing.T) {
	projectRoot, _ := createTestCowProject(t)

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	wsPath, err := m.CreateWorkspace("feature/new", "main", "")
	require.NoError(t, err)

	// Workspace directory should exist
	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace directory should exist")

	// Should contain a .git directory (it's a full clone)
	_, statErr = os.Stat(filepath.Join(wsPath, ".git"))
	assert.NoError(t, statErr, "workspace should have .git directory")

	// Should be on the new branch
	cmd := exec.Command("git", "-C", wsPath, "branch", "--show-current")
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "feature/new", strings.TrimSpace(string(output)))

	// Should be listed
	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	branches := make(map[string]bool)
	for _, ws := range workspaces {
		branches[ws.Branch] = true
	}
	assert.True(t, branches["feature/new"])
}

func TestManager_CreateWorkspace_CowMode_EmptyBaseBranchDefaultsToDefault(t *testing.T) {
	projectRoot, _ := createTestCowProject(t)

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	// Passing an empty baseBranch should default to the manager's DefaultBranch ("main")
	wsPath, err := m.CreateWorkspace("feature/defaultbase", "", "")
	require.NoError(t, err)

	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace directory should exist")

	cmd := exec.Command("git", "-C", wsPath, "branch", "--show-current")
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "feature/defaultbase", strings.TrimSpace(string(output)))
}

func TestManager_CreateWorkspace_CowMode_CustomPath(t *testing.T) {
	projectRoot, _ := createTestCowProject(t)

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	customPath := filepath.Join(projectRoot, "my-custom-dir")
	wsPath, err := m.CreateWorkspace("feature/custom", "main", customPath)
	require.NoError(t, err)

	assert.Equal(t, customPath, wsPath)
	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace at custom path should exist")
}

func TestManager_CreateWorkspace_WorktreeMode_CustomPath(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	customPath := filepath.Join(projectRoot, "my-custom-dir")
	wsPath, err := m.CreateWorkspace("feature/custom", "main", customPath)
	require.NoError(t, err)

	assert.Equal(t, customPath, wsPath)
	_, statErr := os.Stat(wsPath)
	assert.NoError(t, statErr, "workspace at custom path should exist")
}

// --- RemoveWorkspace ---

func TestManager_RemoveWorkspace_WorktreeMode(t *testing.T) {
	projectRoot, barePath, _ := createTestWorktreeProject(t)

	// Create a feature worktree to remove
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("git", "-C", barePath, "worktree", "add", "-b", "feature/foo", featurePath, "main")
	require.NoError(t, cmd.Run())

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	err := m.RemoveWorkspace(featurePath, true)
	require.NoError(t, err)

	// Directory should be gone
	_, statErr := os.Stat(featurePath)
	assert.True(t, os.IsNotExist(statErr), "feature workspace directory should be removed")

	// Should no longer be listed
	workspaces, err := m.ListWorkspaces()
	require.NoError(t, err)
	for _, ws := range workspaces {
		assert.NotEqual(t, "feature/foo", ws.Branch)
	}
}

func TestManager_RemoveWorkspace_CowMode(t *testing.T) {
	projectRoot, mainPath := createTestCowProject(t)

	// Create a CoW clone to remove (using regular copy for portability)
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("cp", "-R", mainPath+"/", featurePath+"/")
	require.NoError(t, cmd.Run())

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"checkout", "-b", "feature/foo"},
	} {
		c := exec.Command("git", args...)
		c.Dir = featurePath
		require.NoError(t, c.Run())
	}

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	err := m.RemoveWorkspace(featurePath, true)
	require.NoError(t, err)

	// Directory should be gone
	_, statErr := os.Stat(featurePath)
	assert.True(t, os.IsNotExist(statErr), "feature workspace directory should be removed")
}

// --- ListWorkspacesDetailed ---

func TestManager_ListWorkspacesDetailed_WorktreeMode(t *testing.T) {
	projectRoot, barePath, mainPath := createTestWorktreeProject(t)

	// Create a feature worktree with a commit so we can check merge status
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("git", "-C", barePath, "worktree", "add", "-b", "feature/foo", featurePath, "main")
	require.NoError(t, cmd.Run())

	info := &ProjectInfo{Mode: ModeWorktree, ProjectRoot: projectRoot, BarePath: barePath}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspacesDetailed(mainPath)
	require.NoError(t, err)
	assert.Len(t, workspaces, 2)

	for _, ws := range workspaces {
		switch ws.Branch {
		case "main":
			assert.True(t, ws.IsMain)
			assert.True(t, ws.IsCurrent) // we passed mainPath as current
		case "feature/foo":
			assert.False(t, ws.IsMain)
			assert.False(t, ws.IsCurrent)
			assert.False(t, ws.IsMerged)
		}
	}
}

func TestManager_ListWorkspacesDetailed_CowMode(t *testing.T) {
	projectRoot, mainPath := createTestCowProject(t)

	// Create a CoW clone
	featurePath := filepath.Join(projectRoot, "feature-foo")
	cmd := exec.Command("cp", "-R", mainPath+"/", featurePath+"/")
	require.NoError(t, cmd.Run())

	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"checkout", "-b", "feature/foo"},
	} {
		c := exec.Command("git", args...)
		c.Dir = featurePath
		require.NoError(t, c.Run())
	}

	info := &ProjectInfo{Mode: ModeCow, ProjectRoot: projectRoot}
	m := NewManager(info, "main")

	workspaces, err := m.ListWorkspacesDetailed(mainPath)
	require.NoError(t, err)
	assert.Len(t, workspaces, 2)

	for _, ws := range workspaces {
		switch ws.Branch {
		case "main":
			assert.True(t, ws.IsMain)
			assert.True(t, ws.IsCurrent)
		case "feature/foo":
			assert.False(t, ws.IsMain)
			assert.False(t, ws.IsCurrent)
		}
	}
}
