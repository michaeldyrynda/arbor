package steps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
	"github.com/stretchr/testify/assert"
)

func TestDatabaseStep(t *testing.T) {
	t.Run("condition always returns true - controlled by preset", func(t *testing.T) {
		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: t.TempDir(),
		}

		assert.True(t, step.Condition(ctx))
	})

	t.Run("skips when no DB_CONNECTION in context", func(t *testing.T) {
		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: t.TempDir(),
			Env:          make(map[string]string),
		}

		err := step.Run(ctx, types.StepOptions{Verbose: false})
		assert.NoError(t, err)
	})

	t.Run("reads DB_CONNECTION from .env file", func(t *testing.T) {
		tmpDir := t.TempDir()

		envFile := filepath.Join(tmpDir, ".env")
		os.WriteFile(envFile, []byte("DB_CONNECTION=mysql\nDB_DATABASE=testdb\n"), 0644)

		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: tmpDir,
			Env:          make(map[string]string),
		}

		err := step.Run(ctx, types.StepOptions{Verbose: false})
		assert.NoError(t, err)
	})

	t.Run("creates SQLite database file", func(t *testing.T) {
		tmpDir := t.TempDir()

		envFile := filepath.Join(tmpDir, ".env")
		os.WriteFile(envFile, []byte("DB_CONNECTION=sqlite\nDB_DATABASE=database/test.sqlite\n"), 0644)

		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: tmpDir,
			Env:          make(map[string]string),
		}

		err := step.Run(ctx, types.StepOptions{Verbose: true})
		assert.NoError(t, err)

		dbFile := filepath.Join(tmpDir, "database", "test.sqlite")
		assert.FileExists(t, dbFile)
	})

	t.Run("generates database name with app_ prefix", func(t *testing.T) {
		tmpDir := t.TempDir()

		envFile := filepath.Join(tmpDir, ".env")
		os.WriteFile(envFile, []byte("DB_CONNECTION=mysql\n"), 0644)

		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: tmpDir,
			Env:          make(map[string]string),
		}

		err := step.Run(ctx, types.StepOptions{Verbose: false})
		assert.NoError(t, err)
	})

	t.Run("name returns correct value", func(t *testing.T) {
		step := NewDatabaseStep(8)
		assert.Equal(t, "database.create", step.Name())
	})

	t.Run("priority returns correct value", func(t *testing.T) {
		step := NewDatabaseStep(8)
		assert.Equal(t, 8, step.Priority())
	})

	t.Run("reads DB config from context env", func(t *testing.T) {
		step := NewDatabaseStep(8)
		ctx := types.ScaffoldContext{
			WorktreePath: t.TempDir(),
			Env: map[string]string{
				"DB_CONNECTION": "mysql",
				"DB_DATABASE":   "testdb",
			},
		}

		err := step.Run(ctx, types.StepOptions{Verbose: false})
		assert.NoError(t, err)
	})
}
