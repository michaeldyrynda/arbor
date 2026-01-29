package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/michaeldyrynda/arbor/internal/config"
	"github.com/michaeldyrynda/arbor/internal/scaffold/steps"
	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
)

func TestIntegration_TemplateReplacementChain(t *testing.T) {
	t.Run("env.read sets variable used by env.write", func(t *testing.T) {
		tmpDir := t.TempDir()

		envContent := `DB_CONNECTION=mysql
APP_NAME=original_app
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
			SiteName:     "newapp",
			Branch:       "test",
		}

		readStep := steps.Create("env.read", config.StepConfig{Key: "APP_NAME", StoreAs: "OriginalApp"})
		require.NotNil(t, readStep)
		err := readStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)
		assert.Equal(t, "original_app", ctx.GetVar("OriginalApp"))

		writeStep := steps.Create("env.write", config.StepConfig{Key: "NEW_APP", Value: "{{ .SiteName }}"})
		require.NotNil(t, writeStep)
		err = writeStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "NEW_APP=newapp")
	})
}

func TestIntegration_DatabaseCreationWithEnv(t *testing.T) {
	t.Run("db.create generates suffix and persists to worktree config", func(t *testing.T) {
		if _, err := exec.LookPath("mysql"); err != nil {
			t.Skip("mysql client not found")
		}

		tmpDir := t.TempDir()

		envContent := `DB_CONNECTION=mysql
DB_HOST=localhost
APP_NAME=myapp
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
			SiteName:     "myapp",
			Branch:       "test",
		}

		dbStep := steps.Create("db.create", config.StepConfig{})
		require.NotNil(t, dbStep)
		err := dbStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		suffix := ctx.GetDbSuffix()
		assert.NotEmpty(t, suffix, "DbSuffix should be set after db.create")

		cfg, err := config.ReadWorktreeConfig(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, suffix, cfg.DbSuffix, "DbSuffix should be persisted to worktree arbor.yaml")

		parts := strings.Split(suffix, "_")
		assert.Len(t, parts, 2, "Suffix should be in format {adjective}_{noun}")
	})
}

func TestIntegration_EnvReadWriteFlow(t *testing.T) {
	t.Run("env.read → env.write → binary step with template variables", func(t *testing.T) {
		tmpDir := t.TempDir()

		envContent := `APP_NAME=original
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
			SiteName:     "newapp",
			Path:         "feature-auth",
		}

		readStep := steps.Create("env.read", config.StepConfig{Key: "APP_NAME", StoreAs: "OriginalName"})
		require.NotNil(t, readStep)
		err := readStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		writeStep := steps.Create("env.write", config.StepConfig{Key: "NEW_NAME", Value: "{{ .SiteName }}_{{ .Path }}"})
		require.NotNil(t, writeStep)
		err = writeStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "NEW_NAME=newapp_feature-auth")
		assert.Contains(t, string(content), "APP_NAME=original")
	})
}

func TestIntegration_DatabaseCreateEnvWriteMigrate(t *testing.T) {
	t.Run("db.create → env.write → template in write step", func(t *testing.T) {
		if _, err := exec.LookPath("mysql"); err != nil {
			t.Skip("mysql client not found")
		}

		tmpDir := t.TempDir()

		envContent := `DB_CONNECTION=mysql
APP_NAME=myapp
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
			SiteName:     "myapp",
			Branch:       "test",
		}

		dbStep := steps.Create("db.create", config.StepConfig{})
		require.NotNil(t, dbStep)
		err := dbStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		suffix := ctx.GetDbSuffix()
		assert.NotEmpty(t, suffix)

		writeStep := steps.Create("env.write", config.StepConfig{Key: "DB_DATABASE", Value: "{{ .SiteName }}_{{ .DbSuffix }}"})
		require.NotNil(t, writeStep)
		err = writeStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		expectedDbName := "myapp_" + suffix
		assert.Contains(t, string(content), "DB_DATABASE="+expectedDbName)
	})
}

func TestIntegration_DatabaseDestroyCleanup(t *testing.T) {
	t.Run("db.destroy reads suffix from worktree config and cleans up", func(t *testing.T) {
		tmpDir := t.TempDir()

		envContent := `DB_CONNECTION=mysql
APP_NAME=myapp
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		err := config.WriteWorktreeConfig(tmpDir, map[string]string{"db_suffix": "swift_runner"})
		require.NoError(t, err)

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
		}

		destroyStep := steps.Create("db.destroy", config.StepConfig{})
		require.NotNil(t, destroyStep)
		err = destroyStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		suffix := ctx.GetDbSuffix()
		assert.Equal(t, "swift_runner", suffix, "DbSuffix should be read from worktree config")
	})
}

func TestIntegration_BunIntegration(t *testing.T) {
	t.Run("node.bun step is registered and functional", func(t *testing.T) {
		step := steps.Create("node.bun", config.StepConfig{
			Args: []string{"--version"},
		})

		assert.NotNil(t, step)
		assert.Equal(t, "node.bun", step.Name())
		assert.Equal(t, 10, step.Priority())
	})
}

func TestIntegration_FullLifecycle(t *testing.T) {
	t.Run("simulate full workflow: create db, write env, cleanup", func(t *testing.T) {
		if _, err := exec.LookPath("mysql"); err != nil {
			t.Skip("mysql client not found")
		}

		tmpDir := t.TempDir()

		envContent := `DB_CONNECTION=mysql
APP_NAME=myapp
`
		envFile := filepath.Join(tmpDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

		ctx := &types.ScaffoldContext{
			WorktreePath: tmpDir,
			SiteName:     "myapp",
			Branch:       "test",
			Path:         "feature-auth",
		}

		dbStep := steps.Create("db.create", config.StepConfig{})
		require.NotNil(t, dbStep)
		err := dbStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		suffix := ctx.GetDbSuffix()
		assert.NotEmpty(t, suffix)

		writeDbStep := steps.Create("env.write", config.StepConfig{Key: "DB_DATABASE", Value: "{{ .SiteName }}_{{ .DbSuffix }}"})
		require.NotNil(t, writeDbStep)
		err = writeDbStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		content, err := os.ReadFile(envFile)
		require.NoError(t, err)
		expectedDbName := "myapp_" + suffix
		assert.Contains(t, string(content), "DB_DATABASE="+expectedDbName)

		writeDomainStep := steps.Create("env.write", config.StepConfig{Key: "APP_DOMAIN", Value: "app.{{ .Path }}.test"})
		require.NotNil(t, writeDomainStep)
		err = writeDomainStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		content, err = os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "APP_DOMAIN=app.feature-auth.test")

		destroyStep := steps.Create("db.destroy", config.StepConfig{})
		require.NotNil(t, destroyStep)
		err = destroyStep.Run(ctx, types.StepOptions{Verbose: false})
		require.NoError(t, err)

		destroyedSuffix := ctx.GetDbSuffix()
		assert.NotEmpty(t, destroyedSuffix, "DbSuffix should still be set after destroy")
	})
}
