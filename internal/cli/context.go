package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/artisanexperiences/arbor/internal/config"
	arborerrors "github.com/artisanexperiences/arbor/internal/errors"
	"github.com/artisanexperiences/arbor/internal/git"
	"github.com/artisanexperiences/arbor/internal/presets"
	"github.com/artisanexperiences/arbor/internal/scaffold"
	"github.com/artisanexperiences/arbor/internal/scaffold/steps"
	"github.com/artisanexperiences/arbor/internal/workspace"
)

type ProjectContext struct {
	CWD           string
	Mode          string // workspace.ModeWorktree or workspace.ModeCow
	BarePath      string
	ProjectPath   string
	Config        *config.Config
	DefaultBranch string

	presetManager    *presets.Manager
	scaffoldManager  *scaffold.ScaffoldManager
	workspaceManager *workspace.Manager
	managersInit     sync.Once
}

func OpenProjectFromCWD() (*ProjectContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}

	info, err := workspace.FindProjectRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("finding arbor project: %w", err)
	}

	projectPath := info.ProjectRoot
	cfg, err := config.LoadProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("loading project config: %w", err)
	}

	defaultBranch := cfg.DefaultBranch
	if defaultBranch == "" {
		switch info.Mode {
		case workspace.ModeWorktree:
			defaultBranch, _ = git.GetDefaultBranch(info.BarePath)
		case workspace.ModeCow:
			// Fall back to "main"; cfg.DefaultBranch from arbor.yaml takes precedence above.
		}
		if defaultBranch == "" {
			defaultBranch = config.DefaultBranch
		}
	}

	return &ProjectContext{
		CWD:           cwd,
		Mode:          info.Mode,
		BarePath:      info.BarePath,
		ProjectPath:   projectPath,
		Config:        cfg,
		DefaultBranch: defaultBranch,
	}, nil
}

func (pc *ProjectContext) IsInWorktree() bool {
	cwdAbs, err := filepath.Abs(pc.CWD)
	if err != nil {
		return false
	}

	projectAbs, err := filepath.Abs(pc.ProjectPath)
	if err != nil {
		return false
	}

	// At project root — not a workspace
	if cwdAbs == projectAbs {
		return false
	}

	// Inside the mode-specific internal directory — not a workspace
	switch pc.Mode {
	case workspace.ModeWorktree:
		if cwdAbs == filepath.Join(projectAbs, ".bare") {
			return false
		}
	case workspace.ModeCow:
		if cwdAbs == filepath.Join(projectAbs, ".arbor") {
			return false
		}
	}

	return true
}

func (pc *ProjectContext) MustBeInWorktree() error {
	if !pc.IsInWorktree() {
		return arborerrors.ErrWorktreeNotFound
	}
	return nil
}

func (pc *ProjectContext) PresetManager() *presets.Manager {
	pc.managersInit.Do(func() {
		pc.initManagers()
	})
	return pc.presetManager
}

func (pc *ProjectContext) ScaffoldManager() *scaffold.ScaffoldManager {
	pc.managersInit.Do(func() {
		pc.initManagers()
	})
	return pc.scaffoldManager
}

func (pc *ProjectContext) WorkspaceManager() *workspace.Manager {
	pc.managersInit.Do(func() {
		pc.initManagers()
	})
	return pc.workspaceManager
}

func (pc *ProjectContext) initManagers() {
	// Create explicit step registry with default steps
	stepRegistry := steps.NewRegistry()
	stepRegistry.RegisterDefaults()

	// Initialize managers with dependency injection
	pc.presetManager = presets.NewManager()
	pc.scaffoldManager = scaffold.NewScaffoldManagerWithRegistry(stepRegistry)
	presets.RegisterAllWithScaffold(pc.scaffoldManager)

	// Determine workspace mode from config, defaulting to worktree
	wsMode := pc.Config.WorkspaceMode
	if wsMode == "" {
		wsMode = workspace.ModeWorktree
	}

	pc.workspaceManager = workspace.NewManager(&workspace.ProjectInfo{
		Mode:        wsMode,
		ProjectRoot: pc.ProjectPath,
		BarePath:    pc.BarePath,
	}, pc.DefaultBranch)
}
