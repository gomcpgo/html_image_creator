package config

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestEnsureLibs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake embedded FS with test library files
	fakeLibs := fstest.MapFS{
		"chart.min.js": &fstest.MapFile{Data: []byte("// chart.js content")},
		"d3.min.js":    &fstest.MapFile{Data: []byte("// d3.js content")},
	}

	err := EnsureLibs(tmpDir, fakeLibs)
	if err != nil {
		t.Fatalf("EnsureLibs failed: %v", err)
	}

	// Verify files were written
	libsDir := GetLibsDir(tmpDir)
	for _, name := range []string{"chart.min.js", "d3.min.js"} {
		path := filepath.Join(libsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
}

func TestEnsureLibs_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()

	fakeLibs := fstest.MapFS{
		"chart.min.js": &fstest.MapFile{Data: []byte("// chart.js content")},
	}

	// Run twice
	if err := EnsureLibs(tmpDir, fakeLibs); err != nil {
		t.Fatalf("first EnsureLibs failed: %v", err)
	}
	if err := EnsureLibs(tmpDir, fakeLibs); err != nil {
		t.Fatalf("second EnsureLibs failed: %v", err)
	}

	// File should still exist
	path := filepath.Join(GetLibsDir(tmpDir), "chart.min.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "// chart.js content" {
		t.Errorf("unexpected content: %s", content)
	}
}
