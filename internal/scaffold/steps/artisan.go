package steps

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
)

type ArtisanStep struct {
	name     string
	args     []string
	priority int
}

func NewArtisanStep(cfg config.StepConfig, priority int) *ArtisanStep {
	p := priority
	if cfg.Priority != 0 {
		p = cfg.Priority
	}
	return &ArtisanStep{
		name:     "php.laravel.artisan",
		args:     cfg.Args,
		priority: p,
	}
}

func (s *ArtisanStep) Name() string {
	return s.name
}

func (s *ArtisanStep) Priority() int {
	return s.priority
}

func (s *ArtisanStep) Condition(ctx types.ScaffoldContext) bool {
	_, err := exec.LookPath("php")
	return err == nil
}

func (s *ArtisanStep) Run(ctx types.ScaffoldContext, opts types.StepOptions) error {
	allArgs := append([]string{"artisan"}, s.args...)
	allArgs = append(allArgs, opts.Args...)

	if opts.Verbose {
		fmt.Printf("  Running: php %s\n", strings.Join(allArgs, " "))
	}

	cmd := exec.Command("php", allArgs...)
	cmd.Dir = ctx.WorktreePath
	return cmd.Run()
}
