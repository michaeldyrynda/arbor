package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	arborerrors "github.com/artisanexperiences/arbor/internal/errors"
	"github.com/artisanexperiences/arbor/internal/utils"
)

// Manager manages workspaces in an Arbor project, abstracting over the
// differences between worktree mode and cow mode.
type Manager struct {
	Mode          string
	ProjectRoot   string
	BarePath      string // empty for cow mode
	DefaultBranch string
}

// NewManager constructs a Manager from a discovered ProjectInfo and the
// project's default branch name.
func NewManager(info *ProjectInfo, defaultBranch string) *Manager {
	return &Manager{
		Mode:          info.Mode,
		ProjectRoot:   info.ProjectRoot,
		BarePath:      info.BarePath,
		DefaultBranch: defaultBranch,
	}
}

// FindProjectRoot searches startPath and its parents for an Arbor project
// root. It looks for a .bare directory (worktree mode) or a .arbor directory
// (cow mode). If both are present, .bare takes precedence for backward
// compatibility.
//
// Returns ErrProjectNotFound (wrapping arborerrors.ErrWorktreeNotFound) when
// neither marker is found.
func FindProjectRoot(startPath string) (*ProjectInfo, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	current := absPath
	for {
		// Check for .bare (worktree mode) first — highest precedence
		barePath := filepath.Join(current, ".bare")
		if info, err := os.Stat(barePath); err == nil && info.IsDir() {
			return &ProjectInfo{
				Mode:        ModeWorktree,
				ProjectRoot: current,
				BarePath:    barePath,
			}, nil
		}

		// Check for .arbor (cow mode)
		arborDir := filepath.Join(current, ".arbor")
		if info, err := os.Stat(arborDir); err == nil && info.IsDir() {
			return &ProjectInfo{
				Mode:        ModeCow,
				ProjectRoot: current,
				BarePath:    "",
			}, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding a marker
			return nil, fmt.Errorf("arbor project not found in %s or any parent directory: %w",
				absPath, arborerrors.ErrWorktreeNotFound)
		}
		current = parent
	}
}

// CreateWorkspace creates a new workspace for the given branch. The workspace
// path is derived from the project root and the sanitised branch name unless
// customPath is non-empty, in which case customPath is used directly.
//
// Returns the absolute path of the created workspace.
func (m *Manager) CreateWorkspace(branch, baseBranch, customPath string) (string, error) {
	wsPath := customPath
	if wsPath == "" {
		wsPath = filepath.Join(m.ProjectRoot, utils.SanitisePath(branch))
	}

	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return "", fmt.Errorf("resolving workspace path: %w", err)
	}

	switch m.Mode {
	case ModeWorktree:
		return absPath, m.createWorktree(branch, baseBranch, absPath)
	case ModeCow:
		return absPath, m.createCowWorkspace(branch, baseBranch, absPath)
	default:
		return "", fmt.Errorf("unknown workspace mode: %q", m.Mode)
	}
}

// createWorktree handles workspace creation for worktree mode.
func (m *Manager) createWorktree(branch, baseBranch, absPath string) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	// Check if branch already exists
	checkCmd := exec.Command("git", "-C", m.BarePath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	branchExists := checkCmd.Run() == nil

	var cmd *exec.Cmd
	if branchExists {
		cmd = exec.Command("git", "-C", m.BarePath, "worktree", "add", absPath, branch)
	} else {
		if baseBranch == "" {
			baseBranch = m.DefaultBranch
		}
		cmd = exec.Command("git", "-C", m.BarePath, "worktree", "add", "-b", branch, absPath, baseBranch)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add failed: %w\n%s", err, string(output))
	}
	return nil
}

// createCowWorkspace handles workspace creation for cow mode.
// It copies the default-branch workspace using CoW semantics and then
// switches the clone to the requested branch starting from baseBranch.
// If baseBranch is empty it defaults to m.DefaultBranch.
func (m *Manager) createCowWorkspace(branch, baseBranch, absPath string) error {
	if baseBranch == "" {
		baseBranch = m.DefaultBranch
	}

	srcPath := filepath.Join(m.ProjectRoot, m.DefaultBranch)
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("source workspace %q not found: %w", srcPath, err)
	}

	// Warn if source has uncommitted changes (non-fatal)
	dirtyCmd := exec.Command("git", "-C", srcPath, "status", "--porcelain")
	if output, err := dirtyCmd.Output(); err == nil && len(strings.TrimSpace(string(output))) > 0 {
		fmt.Fprintf(os.Stderr, "warning: source workspace %q has uncommitted changes; "+
			"these will be included in the new workspace\n", srcPath)
	}

	if err := CopyCoW(srcPath, absPath); err != nil {
		return fmt.Errorf("copying workspace: %w", err)
	}

	// Check if the branch already exists in the clone's repo
	checkCmd := exec.Command("git", "-C", absPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	branchExists := checkCmd.Run() == nil

	var branchCmd *exec.Cmd
	if branchExists {
		branchCmd = exec.Command("git", "-C", absPath, "checkout", branch)
	} else {
		// Create the new branch starting from baseBranch. This honours -b <base>
		// from callers (e.g. arbor work feature/foo -b develop).
		branchCmd = exec.Command("git", "-C", absPath, "checkout", "-b", branch, baseBranch)
	}

	if output, err := branchCmd.CombinedOutput(); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(absPath)
		return fmt.Errorf("checking out branch %q: %w\n%s", branch, err, string(output))
	}

	return nil
}

// RemoveWorkspace removes the workspace at the given path. In worktree mode
// this deregisters the worktree from git; in cow mode it removes the directory.
// The force flag is passed to git worktree remove in worktree mode.
func (m *Manager) RemoveWorkspace(wsPath string, force bool) error {
	switch m.Mode {
	case ModeWorktree:
		return m.removeWorktree(wsPath, force)
	case ModeCow:
		return os.RemoveAll(wsPath)
	default:
		return fmt.Errorf("unknown workspace mode: %q", m.Mode)
	}
}

// removeWorktree handles workspace removal for worktree mode.
func (m *Manager) removeWorktree(wsPath string, force bool) error {
	args := []string{"-C", m.BarePath, "worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, wsPath)

	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w\n%s", err, string(output))
	}
	return nil
}

// ListWorkspaces returns all workspaces in the project.
func (m *Manager) ListWorkspaces() ([]Workspace, error) {
	switch m.Mode {
	case ModeWorktree:
		return m.listWorktrees()
	case ModeCow:
		return m.listCowWorkspaces()
	default:
		return nil, fmt.Errorf("unknown workspace mode: %q", m.Mode)
	}
}

// listWorktrees lists git worktrees for worktree mode.
func (m *Manager) listWorktrees() ([]Workspace, error) {
	cmd := exec.Command("git", "-C", m.BarePath, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	parentDir := filepath.Dir(m.BarePath)

	var workspaces []Workspace
	var currentPath string
	var currentBranch string

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if !filepath.IsAbs(currentPath) && parentDir != "" {
				currentPath = filepath.Join(parentDir, currentPath)
			}
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			currentBranch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
			if currentPath != "" && currentBranch != "" {
				workspaces = append(workspaces, Workspace{
					Path:   currentPath,
					Branch: currentBranch,
				})
				currentPath = ""
			}
		}
	}

	return workspaces, nil
}

// listCowWorkspaces scans the project root for subdirectories that are git
// repos (contain a .git entry) and reads their current branch.
func (m *Manager) listCowWorkspaces() ([]Workspace, error) {
	entries, err := os.ReadDir(m.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("reading project root: %w", err)
	}

	var workspaces []Workspace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(m.ProjectRoot, entry.Name())

		// Skip the .arbor marker directory
		if entry.Name() == ".arbor" {
			continue
		}

		// Check if it is a git repository
		gitPath := filepath.Join(dirPath, ".git")
		if _, err := os.Stat(gitPath); err != nil {
			continue
		}

		// Get the current branch
		branchCmd := exec.Command("git", "-C", dirPath, "branch", "--show-current")
		branchOutput, err := branchCmd.Output()
		if err != nil {
			continue
		}
		branch := strings.TrimSpace(string(branchOutput))
		if branch == "" {
			continue
		}

		workspaces = append(workspaces, Workspace{
			Path:   dirPath,
			Branch: branch,
		})
	}

	return workspaces, nil
}

// ListWorkspacesDetailed returns workspaces enriched with IsMain, IsCurrent,
// and IsMerged metadata. currentPath is the path of the caller's workspace
// (used to set IsCurrent).
func (m *Manager) ListWorkspacesDetailed(currentPath string) ([]Workspace, error) {
	workspaces, err := m.ListWorkspaces()
	if err != nil {
		return nil, err
	}

	currentPathEval, _ := filepath.EvalSymlinks(currentPath)

	for i := range workspaces {
		ws := &workspaces[i]
		ws.IsMain = ws.Branch == m.DefaultBranch

		wsPathEval, _ := filepath.EvalSymlinks(ws.Path)
		ws.IsCurrent = wsPathEval == currentPathEval

		if ws.Branch != m.DefaultBranch {
			ws.IsMerged = m.isMerged(ws.Branch)
		}
	}

	return workspaces, nil
}

// isMerged checks whether branch has been merged into m.DefaultBranch and is
// not the same as the default branch tip.
func (m *Manager) isMerged(branch string) bool {
	var gitDir string
	switch m.Mode {
	case ModeWorktree:
		gitDir = m.BarePath
	case ModeCow:
		// Use the default-branch workspace's repo for merge checks
		gitDir = filepath.Join(m.ProjectRoot, m.DefaultBranch)
	default:
		return false
	}

	// feature is an ancestor of default (i.e., merged)
	ancestorCmd := exec.Command("git", "-C", gitDir, "merge-base", "--is-ancestor", branch, m.DefaultBranch)
	if err := ancestorCmd.Run(); err != nil {
		return false
	}

	// default is NOT an ancestor of feature (i.e., feature has no extra commits beyond default)
	reverseCmd := exec.Command("git", "-C", gitDir, "merge-base", "--is-ancestor", m.DefaultBranch, branch)
	if err := reverseCmd.Run(); err != nil {
		return true // default is not ancestor of feature → feature was merged but differs
	}

	// Both are ancestors of each other → same commit → not "merged" in the PR sense
	return false
}
