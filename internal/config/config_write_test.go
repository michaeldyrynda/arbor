package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveProject(t *testing.T) {
	t.Run("creates new project config", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &Config{
			SiteName:      "MyProject",
			Preset:        "laravel",
			DefaultBranch: "main",
		}

		err := SaveProject(tmpDir, cfg)
		if err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		// Verify file was created
		configPath := filepath.Join(tmpDir, "arbor.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("config file was not created")
		}

		// Load it back and verify
		loaded, err := LoadProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to load project: %v", err)
		}

		if loaded.SiteName != "MyProject" {
			t.Errorf("expected SiteName 'MyProject', got '%s'", loaded.SiteName)
		}
		if loaded.Preset != "laravel" {
			t.Errorf("expected Preset 'laravel', got '%s'", loaded.Preset)
		}
		if loaded.DefaultBranch != "main" {
			t.Errorf("expected DefaultBranch 'main', got '%s'", loaded.DefaultBranch)
		}
	})

	t.Run("preserves existing config data", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "arbor.yaml")

		// Create initial config with extra fields
		initialContent := `site_name: OldSite
preset: old_preset
default_branch: old_branch
custom_field: custom_value
`
		if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
			t.Fatalf("failed to create initial config: %v", err)
		}

		// Save with only some fields updated
		cfg := &Config{
			SiteName: "NewSite",
			// Preset and DefaultBranch left empty - should preserve existing
		}

		err := SaveProject(tmpDir, cfg)
		if err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		// Read back the raw content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "site_name: NewSite") {
			t.Errorf("expected updated site_name not found in:\n%s", contentStr)
		}
		if !strings.Contains(contentStr, "preset: old_preset") {
			t.Errorf("expected preserved preset not found in:\n%s", contentStr)
		}
		if !strings.Contains(contentStr, "default_branch: old_branch") {
			t.Errorf("expected preserved default_branch not found in:\n%s", contentStr)
		}
	})

	t.Run("round-trip preserves data", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Save initial config
		cfg := &Config{
			SiteName:      "TestProject",
			Preset:        "php",
			DefaultBranch: "develop",
		}

		err := SaveProject(tmpDir, cfg)
		if err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		// Load it back
		loaded, err := LoadProject(tmpDir)
		if err != nil {
			t.Fatalf("LoadProject failed: %v", err)
		}

		// Verify all fields
		if loaded.SiteName != "TestProject" {
			t.Errorf("SiteName mismatch: expected 'TestProject', got '%s'", loaded.SiteName)
		}
		if loaded.Preset != "php" {
			t.Errorf("Preset mismatch: expected 'php', got '%s'", loaded.Preset)
		}
		if loaded.DefaultBranch != "develop" {
			t.Errorf("DefaultBranch mismatch: expected 'develop', got '%s'", loaded.DefaultBranch)
		}
	})
}

func TestSaveProject_WorkspaceMode(t *testing.T) {
	t.Run("saves workspace_mode cow", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &Config{
			SiteName:      "TestProject",
			DefaultBranch: "main",
			WorkspaceMode: "cow",
		}

		if err := SaveProject(tmpDir, cfg); err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		loaded, err := LoadProject(tmpDir)
		if err != nil {
			t.Fatalf("LoadProject failed: %v", err)
		}

		if loaded.WorkspaceMode != "cow" {
			t.Errorf("WorkspaceMode mismatch: expected 'cow', got '%s'", loaded.WorkspaceMode)
		}
	})

	t.Run("saves workspace_mode worktree", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &Config{
			SiteName:      "TestProject",
			DefaultBranch: "main",
			WorkspaceMode: "worktree",
		}

		if err := SaveProject(tmpDir, cfg); err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		loaded, err := LoadProject(tmpDir)
		if err != nil {
			t.Fatalf("LoadProject failed: %v", err)
		}

		if loaded.WorkspaceMode != "worktree" {
			t.Errorf("WorkspaceMode mismatch: expected 'worktree', got '%s'", loaded.WorkspaceMode)
		}
	})

	t.Run("preserves existing workspace_mode when saving other fields", func(t *testing.T) {
		tmpDir := t.TempDir()

		initial := `site_name: old
default_branch: main
workspace_mode: cow
`
		configPath := filepath.Join(tmpDir, "arbor.yaml")
		if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
			t.Fatalf("failed to write initial config: %v", err)
		}

		// Save only site_name update – workspace_mode should be preserved
		cfg := &Config{SiteName: "new"}
		if err := SaveProject(tmpDir, cfg); err != nil {
			t.Fatalf("SaveProject failed: %v", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}
		if !strings.Contains(string(content), "workspace_mode: cow") {
			t.Errorf("workspace_mode should be preserved, got:\n%s", content)
		}
	})
}
