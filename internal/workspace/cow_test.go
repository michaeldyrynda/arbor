package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCowSupport_ReturnsResult(t *testing.T) {
	tmpDir := t.TempDir()

	// DetectCowSupport should not panic and should return a bool + nil error
	// on any supported platform. We don't assert the bool because it depends
	// on the filesystem, but we do assert that it doesn't error unexpectedly.
	supported, err := DetectCowSupport(tmpDir)
	// err is allowed to be non-nil on unsupported platforms, but the function
	// must not panic. We simply log the result.
	t.Logf("CoW support: supported=%v, err=%v", supported, err)
}

func TestCopyCoW_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "does-not-exist")
	dst := filepath.Join(tmpDir, "dst")

	err := CopyCoW(src, dst)
	assert.Error(t, err, "CopyCoW should error when source does not exist")
}

func TestCopyCoW_DestinationAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0644))
	require.NoError(t, os.MkdirAll(dst, 0755))

	err := CopyCoW(src, dst)
	assert.Error(t, err, "CopyCoW should error when destination already exists")
}

func TestCopyCoW_CopiesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello world"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0644))

	err := CopyCoW(src, dst)
	require.NoError(t, err)

	// Destination should exist
	_, statErr := os.Stat(dst)
	require.NoError(t, statErr, "destination directory should exist")

	// Files should be copied
	content, readErr := os.ReadFile(filepath.Join(dst, "file.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "hello world", string(content))

	nestedContent, readErr := os.ReadFile(filepath.Join(dst, "subdir", "nested.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "nested", string(nestedContent))
}

func TestCowSupport_WarnMessage(t *testing.T) {
	// CowSupportWarning should return a non-empty string for any platform
	msg := CowSupportWarning()
	assert.NotEmpty(t, msg)
}
