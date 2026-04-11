package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// EnsureLibs copies bundled JS libraries to the runtime libs directory if not present.
// The embeddedLibs should contain the library files at its root (no subdirectory prefix).
func EnsureLibs(rootDir string, embeddedLibs fs.FS) error {
	libsDir := GetLibsDir(rootDir)
	if err := os.MkdirAll(libsDir, 0755); err != nil {
		return fmt.Errorf("failed to create libs directory: %w", err)
	}

	entries, err := fs.ReadDir(embeddedLibs, ".")
	if err != nil {
		return fmt.Errorf("failed to read embedded libs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		destPath := filepath.Join(libsDir, entry.Name())

		// Skip if file already exists with same size
		info, _ := entry.Info()
		if stat, err := os.Stat(destPath); err == nil && stat.Size() == info.Size() {
			continue
		}

		data, err := fs.ReadFile(embeddedLibs, entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded lib %s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write lib %s: %w", entry.Name(), err)
		}
	}

	return nil
}
