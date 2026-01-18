package presets

import (
	"os"
	"path/filepath"

	"github.com/michaeldyrynda/arbor/internal/config"
)

type PHP struct {
	basePreset
}

func NewPHP() *PHP {
	return &PHP{
		basePreset: basePreset{
			name: "php",
			defaultSteps: []config.StepConfig{
				{Name: "php.composer", Args: []string{"install"}},
			},
			cleanupSteps: nil,
		},
	}
}

func (p *PHP) Detect(path string) bool {
	composerPath := filepath.Join(path, "composer.json")
	_, err := os.Stat(composerPath)
	return err == nil
}
