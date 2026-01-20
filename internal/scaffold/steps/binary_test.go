package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("php.laravel.artisan uses BinaryStep with 'php artisan' binary", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"key:generate", "--no-interaction"},
		})

		assert.NotNil(t, step)
		assert.Equal(t, "php.laravel.artisan", step.Name())

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "php artisan", binaryStep.binary)
		assert.Equal(t, []string{"key:generate", "--no-interaction"}, binaryStep.args)
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

		allArgs := append(strings.Fields(binaryStep.binary), binaryStep.args...)
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

		allArgs := append(strings.Fields(binaryStep.binary), binaryStep.args...)
		expectedCommand := "php -v"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("php.laravel.artisan command construction", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"key:generate", "--no-interaction"},
		})

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")
		assert.Equal(t, "php artisan", binaryStep.binary)

		allArgs := append(strings.Fields(binaryStep.binary), binaryStep.args...)
		expectedCommand := "php artisan key:generate --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("php.laravel.artisan migrate:fresh command", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"migrate:fresh", "--seed", "--no-interaction"},
		})

		binaryStep, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")

		allArgs := append(strings.Fields(binaryStep.binary), binaryStep.args...)
		expectedCommand := "php artisan migrate:fresh --seed --no-interaction"
		assert.Equal(t, expectedCommand, joinArgs(allArgs))
	})

	t.Run("binary step condition checks first part of multi-part binary", func(t *testing.T) {
		step := Create("php.laravel.artisan", config.StepConfig{
			Args: []string{"storage:link"},
		})

		_, ok := step.(*BinaryStep)
		assert.True(t, ok, "Expected BinaryStep type")

		_, err := exec.LookPath("php")
		hasPHP := err == nil

		ctx := types.ScaffoldContext{
			WorktreePath: "/tmp",
		}

		result := step.Condition(ctx)
		assert.Equal(t, hasPHP, result, "Condition should check if 'php' exists")
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

func TestStepConditionEvaluator(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := types.ScaffoldContext{
		WorktreePath: tmpDir,
		Branch:       "test-branch",
		Preset:       "php",
		Env:          make(map[string]string),
	}

	evaluator := NewStepConditionEvaluator(ctx)

	t.Run("empty conditions returns true", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("nil conditions returns true", func(t *testing.T) {
		result, err := evaluator.Evaluate(nil)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("file_exists - file exists", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_exists": "test.txt",
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("file_exists - file does not exist", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_exists": "nonexistent.txt",
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("file_contains - file contains pattern", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(`{"name": "test/package"}`), 0644); err != nil {
			t.Fatalf("writing composer.json: %v", err)
		}

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_contains": map[string]interface{}{
				"file":    "composer.json",
				"pattern": "test/package",
			},
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("file_contains - file does not contain pattern", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(`{"name": "other/package"}`), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_contains": map[string]interface{}{
				"file":    "composer.json",
				"pattern": "missing/pattern",
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("command_exists - command exists", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"command_exists": "go",
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("command_exists - command does not exist", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"command_exists": "nonexistentcommand123",
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("os matches current OS", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"os": runtime.GOOS,
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("os does not match current OS", func(t *testing.T) {
		var otherOS string
		switch runtime.GOOS {
		case "darwin":
			otherOS = "linux"
		case "linux":
			otherOS = "darwin"
		case "windows":
			otherOS = "linux"
		default:
			otherOS = "windows"
		}
		result, err := evaluator.Evaluate(map[string]interface{}{
			"os": otherOS,
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("env_exists - env variable exists", func(t *testing.T) {
		t.Setenv("TEST_VAR_STEP", "value")

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_exists": "TEST_VAR_STEP",
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("env_exists - env variable does not exist", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_exists": "NONEXISTENT_VAR_456",
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("env_not_exists - env variable does not exist", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_not_exists": "NONEXISTENT_VAR_789",
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("env_not_exists - env variable exists", func(t *testing.T) {
		t.Setenv("TEST_VAR_STEP_EXISTS", "value")

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_not_exists": "TEST_VAR_STEP_EXISTS",
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("env_file_contains - key exists in .env file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("DB_CONNECTION=sqlite\nAPP_KEY=base64:value\n"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_contains": map[string]interface{}{
				"file": ".env",
				"key":  "DB_CONNECTION",
			},
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("env_file_contains - key does not exist in .env file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("APP_KEY=base64:value\n"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_contains": map[string]interface{}{
				"file": ".env",
				"key":  "DB_CONNECTION",
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("env_file_contains - .env file does not exist", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_contains": map[string]interface{}{
				"file": ".env",
				"key":  "DB_CONNECTION",
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("env_file_missing - .env file does not exist", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("OTHER_KEY=other_value\n"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_missing": map[string]interface{}{
				"file": ".env",
				"key":  "APP_KEY",
			},
		})
		assert.NoError(t, err)
		assert.True(t, result, "APP_KEY should be missing from .env file")
	})

	t.Run("env_file_missing - key does not exist", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("APP_KEY=base64:value\n"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_missing": map[string]interface{}{
				"file": ".env",
				"key":  "DB_CONNECTION",
			},
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("env_file_missing - key exists with value", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("APP_KEY=base64:value\n"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"env_file_missing": map[string]interface{}{
				"file": ".env",
				"key":  "APP_KEY",
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("not condition - negates true condition", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"not": map[string]interface{}{
				"file_exists": "test.txt",
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("not condition - negates false condition", func(t *testing.T) {
		result, err := evaluator.Evaluate(map[string]interface{}{
			"not": map[string]interface{}{
				"file_exists": "nonexistent.txt",
			},
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("multiple conditions - all true", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_exists":    "test.txt",
			"command_exists": "go",
		})
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("multiple conditions - one false", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_exists":    "test.txt",
			"command_exists": "nonexistentcommand123",
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("array condition via not - all conditions true", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644))

		result, err := evaluator.Evaluate(map[string]interface{}{
			"not": []interface{}{
				map[string]interface{}{"file_exists": "test.txt"},
				map[string]interface{}{"command_exists": "go"},
			},
		})
		assert.NoError(t, err)
		assert.False(t, result)
	})
}

func TestStepConditionEvaluator_DifferencesFromConditionEvaluator(t *testing.T) {
	ctx := types.ScaffoldContext{
		WorktreePath: "/tmp",
		Branch:       "test",
	}

	t.Run("file_has_script - NOT supported in StepConditionEvaluator", func(t *testing.T) {
		evaluator := NewStepConditionEvaluator(ctx)

		result, err := evaluator.Evaluate(map[string]interface{}{
			"file_has_script": "build",
		})

		assert.NoError(t, err)
		assert.True(t, result, "file_has_script falls through to default (true) in StepConditionEvaluator since it's not implemented")

		t.Log("Difference: file_has_script condition is supported in ConditionEvaluator but NOT in StepConditionEvaluator")
		t.Log("This functionality must be preserved when unifying evaluators in Phase 2.1")
	})
}
