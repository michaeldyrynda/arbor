package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// DetectCowSupport probes whether the filesystem at dir supports copy-on-write
// (reflink) copies.
//
//   - macOS / APFS: attempts `cp -c` (clonefile syscall).
//   - Linux / Btrfs|XFS: attempts `cp --reflink=always`.
//   - All other platforms: returns false, nil.
//
// A returned error means the probe itself failed for an unexpected reason; it
// does NOT mean CoW is unsupported (that is indicated by supported=false).
func DetectCowSupport(dir string) (supported bool, err error) {
	// Write a tiny temp file to probe with
	srcFile, err := os.CreateTemp(dir, "arbor-cow-probe-src-*")
	if err != nil {
		return false, fmt.Errorf("creating probe source: %w", err)
	}
	srcPath := srcFile.Name()
	_ = srcFile.Close()
	defer os.Remove(srcPath) //nolint:errcheck

	dstPath := srcPath + ".dst"
	defer os.Remove(dstPath) //nolint:errcheck

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("cp", "-c", srcPath, dstPath)
		if runErr := cmd.Run(); runErr != nil {
			return false, nil // clonefile not available (non-APFS volume)
		}
		return true, nil
	case "linux":
		cmd := exec.Command("cp", "--reflink=always", srcPath, dstPath)
		if runErr := cmd.Run(); runErr != nil {
			return false, nil // reflink not available
		}
		return true, nil
	default:
		return false, nil
	}
}

// CowSupportWarning returns a human-readable warning message explaining that
// CoW is not supported on the current filesystem and that a regular copy will
// be used instead.
func CowSupportWarning() string {
	switch runtime.GOOS {
	case "darwin":
		return "Copy-on-write is not supported on this volume (requires APFS). " +
			"Workspaces will be created using a regular copy instead. " +
			"Consider using an APFS-formatted volume for full CoW benefits."
	case "linux":
		return "Copy-on-write is not supported on this filesystem (requires Btrfs or XFS with reflink). " +
			"Workspaces will be created using a regular copy instead."
	default:
		return "Copy-on-write is not supported on this platform. " +
			"Workspaces will be created using a regular copy instead."
	}
}

// CopyCoW copies the directory at src to dst using copy-on-write semantics
// when the filesystem supports it, falling back to a regular recursive copy
// otherwise.
//
// dst must not already exist. src must be an existing directory.
func CopyCoW(src, dst string) error {
	// Validate source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source directory %q: %w", src, err)
	}

	// Validate destination does not exist
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination directory %q already exists", dst)
	}

	// Ensure parent of dst exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating parent of destination: %w", err)
	}

	supported, err := DetectCowSupport(filepath.Dir(dst))
	if err != nil {
		// Detection failed — fall back to regular copy
		supported = false
	}

	return copyDir(src, dst, supported)
}

// copyDir copies src directory to dst.
// When cowSupported is true and the platform supports it, CoW clone flags are
// used; otherwise a plain recursive copy is performed.
func copyDir(src, dst string, cowSupported bool) error {
	switch runtime.GOOS {
	case "darwin":
		// -R  recursive
		// -c  clone (CoW) when cowSupported, otherwise ignored (falls back gracefully)
		args := []string{"-R"}
		if cowSupported {
			args = append(args, "-c")
		}
		// Ensure src doesn't have trailing slash so cp places it AT dst, not inside
		args = append(args, src, dst)
		cmd := exec.Command("cp", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cp failed: %w\n%s", err, string(output))
		}
		return nil
	case "linux":
		reflinkFlag := "--reflink=auto"
		cmd := exec.Command("cp", "-R", reflinkFlag, src, dst)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cp failed: %w\n%s", err, string(output))
		}
		return nil
	default:
		// Fallback: use os package recursive copy
		return copyDirFallback(src, dst)
	}
}

// copyDirFallback is a pure-Go recursive directory copy used on platforms where
// neither APFS nor reflink is available.
func copyDirFallback(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}
