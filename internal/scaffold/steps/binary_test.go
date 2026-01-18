package steps

import (
	"os/exec"
	"testing"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
	"github.com/stretchr/testify/assert"
)

func TestBinaryStep_CommandConstruction(t *testing.T) {
	t.Run("php.composer with install", func(t *testing.T) {
		step := Create("php.composer", config.StepConfig{
			Args: []string{"install"},
		})

		assert.NotNil(t, step)
		assert.Equal(t, "php.composer", step.Name())

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "composer", binaryStep.binary)
		assert.Equal(t, []string{"install"}, binaryStep.args)
	})

	t.Run("php binary", func(t *testing.T) {
		step := Create("php", config.StepConfig{
			Args: []string{"-v"},
		})

		assert.NotNil(t, step)
		assert.Equal(t, "php", step.Name())

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "php", binaryStep.binary)
		assert.Equal(t, []string{"-v"}, binaryStep.args)
	})

	t.Run("php.laravel.artisan uses ArtisanStep", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"key:generate", "--no-interaction"},
		})

		assert.NotNil(t, step)
		assert.Equal(t, "php.laravel.artisan", step.Name())

		_, ok := step.(*BinaryStep)
		assert.False(t, ok, "php.laravel.artisan should use ArtisanStep, not BinaryStep")

		artisanStep, ok := step.(*ArtisanStep)
		assert.True(t, ok, "php.laravel.artisan should be ArtisanStep type")
		assert.Equal(t, []string{"key:generate", "--no-interaction"}, artisanStep.args)
	})
}

func TestBinaryStep_Priority(t *testing.T) {
	t.Run("default priority for php.laravel.artisan", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{})
		assert.Equal(t, 20, step.Priority())
	})

	t.Run("default priority for php.composer", func(t *testing.T) {
		step := Create("php.composer", config.StepConfig{})
		assert.Equal(t, 10, step.Priority())
	})

	t.Run("default priority for php", func(t *testing.T) {
		step := Create("php", config.StepConfig{})
		assert.Equal(t, 5, step.Priority())
	})

	t.Run("custom priority override", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Priority: 50,
		})
		assert.Equal(t, 50, step.Priority())
	})
}

func TestBinaryStep_CommandConstructionChecks(t *testing.T) {
	t.Run("php.composer command construction", func(t *testing.T) {
		step := Create("php.composer", config.StepConfig{
			Args: []string{"install", "--no-interaction"},
		})

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "composer", binaryStep.binary)

		allArgs := append([]string{binaryStep.binary}, binaryStep.args...)
		expectedCommand := "composer install --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("php command construction", func(t *testing.T) {
		step := Create("php", config.StepConfig{
			Args: []string{"-v"},
		})

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "php", binaryStep.binary)

		allArgs := append([]string{binaryStep.binary}, binaryStep.args...)
		expectedCommand := "php -v"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})
}

func TestArtisanStep_CommandConstruction(t *testing.T) {
	t.Run("artisan key:generate command", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"key:generate", "--no-interaction"},
		})

		artisanStep, ok := step.(*ArtisanStep)
		assert.True(t, ok, "Expected ArtisanStep type")

		allArgs := append([]string{"php", "artisan"}, artisanStep.args...)
		expectedCommand := "php artisan key:generate --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("artisan migrate:fresh command", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"migrate:fresh", "--seed", "--no-interaction"},
		})

		artisanStep, ok := step.(*ArtisanStep)
		assert.True(t, ok, "Expected ArtisanStep type")

		allArgs := append([]string{"php", "artisan"}, artisanStep.args...)
		expectedCommand := "php artisan migrate:fresh --seed --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("artisan storage:link command", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"storage:link", "--no-interaction"},
		})

		artisanStep, ok := step.(*ArtisanStep)
		assert.True(t, ok, "Expected ArtisanStep type")

		allArgs := append([]string{"php", "artisan"}, artisanStep.args...)
		expectedCommand := "php artisan storage:link --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("artisan step condition", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"storage:link"},
		})

		artisanStep, ok := step.(*ArtisanStep)
		assert.True(t, ok, "Expected ArtisanStep type")

		_, err := exec.LookPath("php")
		hasPHP := err == nil

		ctx := types.ScaffoldContext{
			WorktreePath: "/tmp",
		}

		result := artisanStep.Condition(ctx)
		assert.Equal(t, hasPHP, result, "Condition should match php availability")
	})
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}
