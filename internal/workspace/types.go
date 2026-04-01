// Package workspace provides a unified abstraction over git worktree projects
// (worktree mode) and copy-on-write clone projects (cow mode), allowing Arbor
// CLI commands to remain mode-agnostic.
package workspace

const (
	// ModeWorktree is the traditional git worktree mode.
	// Projects have a .bare/ directory and linked worktrees.
	ModeWorktree = "worktree"

	// ModeCow is the copy-on-write clone mode.
	// Projects have a .arbor/ marker directory and full git clones.
	ModeCow = "cow"
)

// Workspace represents a single workspace inside an Arbor project.
// In worktree mode this corresponds to a git worktree; in cow mode it is
// a full git clone.
type Workspace struct {
	// Path is the absolute path to the workspace directory.
	Path string
	// Branch is the git branch currently checked out in this workspace.
	Branch string
	// IsMain is true when the branch matches the project default branch.
	IsMain bool
	// IsCurrent is true when this workspace matches the caller's CWD.
	IsCurrent bool
	// IsMerged is true when the branch has been merged into the default branch.
	IsMerged bool
}

// ProjectInfo holds the location and mode of an Arbor project, as discovered
// by FindProjectRoot.
type ProjectInfo struct {
	// Mode is either ModeWorktree or ModeCow.
	Mode string
	// ProjectRoot is the absolute path of the project root directory.
	// In worktree mode this is the parent of .bare/; in cow mode it is the
	// parent of .arbor/.
	ProjectRoot string
	// BarePath is the absolute path to the .bare directory.
	// This is empty in cow mode.
	BarePath string
}
