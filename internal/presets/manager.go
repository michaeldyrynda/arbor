package presets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Manager struct {
	presets map[string]Preset
}

func NewManager() *Manager {
	m := &Manager{
		presets: make(map[string]Preset),
	}
	m.Register(NewLaravel())
	m.Register(NewPHP())
	return m
}

func (m *Manager) Register(preset Preset) {
	m.presets[preset.Name()] = preset
}

func (m *Manager) Get(name string) (Preset, bool) {
	preset, ok := m.presets[name]
	return preset, ok
}

func (m *Manager) Detect(path string) string {
	for _, preset := range m.presets {
		if preset.Detect(path) {
			return preset.Name()
		}
	}
	return ""
}

func (m *Manager) Suggest(path string) string {
	detected := m.Detect(path)
	if detected != "" {
		return detected
	}
	return "php"
}

func (m *Manager) Available() []string {
	names := make([]string, 0, len(m.presets))
	for name := range m.presets {
		names = append(names, name)
	}
	return names
}

func PromptForPreset(m *Manager, suggested string) (string, error) {
	available := m.Available()

	fmt.Printf("Detected preset: %s\n", suggested)
	fmt.Print("Select preset (or press Enter to accept): ")

	var choice string
	_, err := fmt.Scanln(&choice)
	if err != nil && !strings.Contains(err.Error(), "unexpected newline") {
		return "", err
	}

	choice = strings.TrimSpace(choice)
	if choice == "" {
		return suggested, nil
	}

	for _, name := range available {
		if name == choice {
			return choice, nil
		}
	}

	fmt.Printf("Unknown preset: %s. Using suggested: %s\n", choice, suggested)
	return suggested, nil
}

func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func EnsureDirectory(path string) error {
	if !DirectoryExists(path) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

func JoinPath(base string, parts ...string) string {
	result := base
	for _, part := range parts {
		result = filepath.Join(result, part)
	}
	return result
}
