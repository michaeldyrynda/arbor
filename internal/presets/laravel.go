package presets

import (
	"os"
	"path/filepath"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/utils"
)

type Laravel struct {
	basePreset
}

func NewLaravel() *Laravel {
	return &Laravel{
		basePreset: basePreset{
			name: "laravel",
			defaultSteps: []config.StepConfig{
				{Name: "file.copy", From: ".env.example", To: ".env", Priority: 5},
				{Name: "database.create", Priority: 8, Condition: map[string]interface{}{"env_file_contains": map[string]interface{}{"file": ".env", "key": "DB_CONNECTION"}}},
				{Name: "php.composer", Args: []string{"install"}, Priority: 10, Condition: map[string]interface{}{"file_exists": "composer.lock"}},
				{Name: "php.composer", Args: []string{"update"}, Priority: 10, Condition: map[string]interface{}{"not": map[string]interface{}{"file_exists": "composer.lock"}}},
				{Name: "node.npm", Args: []string{"ci"}, Priority: 10, Condition: map[string]interface{}{"file_exists": "package-lock.json"}},
				{Name: "node.npm", Args: []string{"run", "build"}, Priority: 15, Condition: map[string]interface{}{"file_exists": "package-lock.json"}},
				{Name: "php.laravel.artisan", Args: []string{"key:generate", "--no-interaction"}, Priority: 18, Condition: map[string]interface{}{"env_file_missing": "APP_KEY"}},
				{Name: "php.laravel.artisan", Args: []string{"migrate:fresh", "--seed", "--no-interaction"}, Priority: 20},
				{Name: "php.laravel.artisan", Args: []string{"storage:link", "--no-interaction"}, Priority: 25},
				{Name: "herd", Args: []string{"link", "--secure", "{{ .SiteName }}"}, Priority: 60},
			},
			cleanupSteps: []config.CleanupStep{
				{Name: "herd", Condition: nil},
				{Name: "bash.run", Condition: map[string]interface{}{
					"command":    "echo \"Consider cleaning up database: {{ .DB_DATABASE }}\"",
					"env_exists": "DB_CONNECTION",
				}},
			},
		},
	}
}

func (p *Laravel) Detect(path string) bool {
	composerPath := filepath.Join(path, "composer.json")
	if _, err := os.Stat(composerPath); err != nil {
		return false
	}

	artisanPath := filepath.Join(path, "artisan")
	if _, err := os.Stat(artisanPath); err != nil {
		return false
	}

	return true
}

func (p *Laravel) Suggest(path string) string {
	env := utils.ReadEnvFile(path, ".env")
	if env["DB_CONNECTION"] != "" {
		return "laravel"
	}
	return ""
}
