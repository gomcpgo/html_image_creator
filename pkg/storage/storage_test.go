package storage

import (
	"encoding/json"
	"html_image_creator/pkg/post"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStorage(t *testing.T) (*Storage, string) {
	t.Helper()
	tmpDir := t.TempDir()
	s := NewStorage(tmpDir)
	return s, tmpDir
}

func createTestPost(t *testing.T, s *Storage, id string) {
	t.Helper()
	p := &post.ImagePost{
		ID:          id,
		Name:        "Test Post",
		HTMLContent: "<html><body>test</body></html>",
		Width:       800,
		Height:      600,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.CreatePost(p); err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}
}

func TestWriteChartData_JSON(t *testing.T) {
	s, tmpDir := setupTestStorage(t)
	createTestPost(t, s, "test-chart-a1b2")

	data := `[{"month":"Jan","sales":100},{"month":"Feb","sales":200}]`
	err := s.WriteChartData("test-chart-a1b2", data, "json")
	if err != nil {
		t.Fatalf("WriteChartData failed: %v", err)
	}

	// Verify file was written
	dataPath := filepath.Join(tmpDir, "test-chart-a1b2", "data.json")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("data.json not found: %v", err)
	}
	if string(content) != data {
		t.Errorf("data mismatch: got %q, want %q", string(content), data)
	}
}

func TestWriteChartData_CSV(t *testing.T) {
	s, tmpDir := setupTestStorage(t)
	createTestPost(t, s, "test-chart-c3d4")

	data := "month,sales\nJan,100\nFeb,200\n"
	err := s.WriteChartData("test-chart-c3d4", data, "csv")
	if err != nil {
		t.Fatalf("WriteChartData failed: %v", err)
	}

	dataPath := filepath.Join(tmpDir, "test-chart-c3d4", "data.csv")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("data.csv not found: %v", err)
	}
	if string(content) != data {
		t.Errorf("data mismatch: got %q, want %q", string(content), data)
	}
}

func TestWriteChartData_NonexistentPost(t *testing.T) {
	s, _ := setupTestStorage(t)

	err := s.WriteChartData("nonexistent-a1b2", "data", "json")
	if err == nil {
		t.Fatal("expected error for nonexistent post")
	}
}

func TestWriteChartData_InvalidFormat(t *testing.T) {
	s, _ := setupTestStorage(t)
	createTestPost(t, s, "test-chart-e5f6")

	err := s.WriteChartData("test-chart-e5f6", "data", "xml")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestReadChartData(t *testing.T) {
	s, _ := setupTestStorage(t)
	createTestPost(t, s, "test-chart-g7h8")

	data := `{"labels":["A","B"],"values":[10,20]}`
	if err := s.WriteChartData("test-chart-g7h8", data, "json"); err != nil {
		t.Fatalf("WriteChartData failed: %v", err)
	}

	got, err := s.ReadChartData("test-chart-g7h8")
	if err != nil {
		t.Fatalf("ReadChartData failed: %v", err)
	}
	if got != data {
		t.Errorf("data mismatch: got %q, want %q", got, data)
	}
}

func TestReadChartData_NoData(t *testing.T) {
	s, _ := setupTestStorage(t)
	createTestPost(t, s, "test-plain-i9j0")

	_, err := s.ReadChartData("test-plain-i9j0")
	if err == nil {
		t.Fatal("expected error reading chart data from non-chart post")
	}
}

func TestUpdateMetadata(t *testing.T) {
	s, tmpDir := setupTestStorage(t)
	createTestPost(t, s, "test-meta-k1l2")

	meta := &post.Metadata{
		Name:         "Test Post",
		Width:        800,
		Height:       600,
		HasChartData: true,
		DataFormat:   "json",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := s.UpdateMetadata("test-meta-k1l2", meta)
	if err != nil {
		t.Fatalf("UpdateMetadata failed: %v", err)
	}

	// Read back and verify
	metaPath := filepath.Join(tmpDir, "test-meta-k1l2", "metadata.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("metadata.json not found: %v", err)
	}

	var readMeta post.Metadata
	if err := json.Unmarshal(content, &readMeta); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}
	if !readMeta.HasChartData {
		t.Error("HasChartData should be true")
	}
	if readMeta.DataFormat != "json" {
		t.Errorf("DataFormat should be json, got %s", readMeta.DataFormat)
	}
}
