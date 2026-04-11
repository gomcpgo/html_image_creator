package post

import (
	"fmt"
	"testing"
	"time"
)

// mockStorage implements StorageInterface for testing
type mockStorage struct {
	posts      map[string]*ImagePost
	chartData  map[string]string
	metadata   map[string]*Metadata
	dataFormat map[string]string
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		posts:      make(map[string]*ImagePost),
		chartData:  make(map[string]string),
		metadata:   make(map[string]*Metadata),
		dataFormat: make(map[string]string),
	}
}

func (m *mockStorage) PostExists(postID string) bool {
	_, ok := m.posts[postID]
	return ok
}

func (m *mockStorage) CreatePost(p *ImagePost) error {
	m.posts[p.ID] = p
	m.metadata[p.ID] = &Metadata{
		Name:      p.Name,
		Width:     p.Width,
		Height:    p.Height,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	return nil
}

func (m *mockStorage) UpdatePost(p *ImagePost) error {
	if _, ok := m.posts[p.ID]; !ok {
		return fmt.Errorf("post %s does not exist", p.ID)
	}
	m.posts[p.ID] = p
	return nil
}

func (m *mockStorage) GetPost(postID string) (*ImagePost, error) {
	p, ok := m.posts[postID]
	if !ok {
		return nil, fmt.Errorf("post %s does not exist", postID)
	}
	return p, nil
}

func (m *mockStorage) ListPosts() ([]*PostInfo, error) {
	return nil, nil
}

func (m *mockStorage) CopyMediaFile(postID, sourcePath string) (string, error) {
	return "media/" + sourcePath, nil
}

func (m *mockStorage) DeletePost(postID string) error {
	delete(m.posts, postID)
	return nil
}

func (m *mockStorage) GetPostPath(postID string) string {
	return "/tmp/" + postID
}

func (m *mockStorage) GetHTMLPath(postID string) string {
	return "/tmp/" + postID + "/index.html"
}

func (m *mockStorage) WriteChartData(postID string, data string, format string) error {
	if _, ok := m.posts[postID]; !ok {
		return fmt.Errorf("post %s does not exist", postID)
	}
	m.chartData[postID] = data
	m.dataFormat[postID] = format
	return nil
}

func (m *mockStorage) ReadChartData(postID string) (string, error) {
	data, ok := m.chartData[postID]
	if !ok {
		return "", fmt.Errorf("no chart data for post %s", postID)
	}
	return data, nil
}

func (m *mockStorage) UpdateMetadata(postID string, meta *Metadata) error {
	if _, ok := m.posts[postID]; !ok {
		return fmt.Errorf("post %s does not exist", postID)
	}
	m.metadata[postID] = meta
	return nil
}

func TestCreateChart(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	chartHTML := `<html><body><canvas id="chart"></canvas><script src="libs/chart.min.js"></script></body></html>`
	data := `[{"month":"Jan","sales":100}]`

	p, err := svc.CreateChart("Sales Chart", chartHTML, 800, 600, data, "json")
	if err != nil {
		t.Fatalf("CreateChart failed: %v", err)
	}

	if p.Name != "Sales Chart" {
		t.Errorf("expected name 'Sales Chart', got %q", p.Name)
	}
	if p.Width != 800 || p.Height != 600 {
		t.Errorf("expected 800x600, got %dx%d", p.Width, p.Height)
	}

	// Verify chart data was written
	storedData, ok := store.chartData[p.ID]
	if !ok {
		t.Fatal("chart data was not written to storage")
	}
	if storedData != data {
		t.Errorf("stored data mismatch: got %q, want %q", storedData, data)
	}

	// Verify metadata has chart flag
	meta := store.metadata[p.ID]
	if !meta.HasChartData {
		t.Error("metadata.HasChartData should be true")
	}
	if meta.DataFormat != "json" {
		t.Errorf("metadata.DataFormat should be json, got %q", meta.DataFormat)
	}
}

func TestCreateChart_CSV(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	data := "month,sales\nJan,100\n"
	p, err := svc.CreateChart("CSV Chart", "<html></html>", 800, 600, data, "csv")
	if err != nil {
		t.Fatalf("CreateChart failed: %v", err)
	}

	if store.dataFormat[p.ID] != "csv" {
		t.Errorf("expected csv format, got %q", store.dataFormat[p.ID])
	}
}

func TestCreateChart_InvalidFormat(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	_, err := svc.CreateChart("Bad Chart", "<html></html>", 800, 600, "data", "xml")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestCreateChart_EmptyData(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	_, err := svc.CreateChart("Empty Chart", "<html></html>", 800, 600, "", "json")
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestSetChartData(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	// Create a chart first
	p, err := svc.CreateChart("Update Test", "<html></html>", 800, 600, `{"old":true}`, "json")
	if err != nil {
		t.Fatalf("CreateChart failed: %v", err)
	}

	originalUpdatedAt := p.UpdatedAt

	// Allow time difference
	time.Sleep(1 * time.Millisecond)

	// Update chart data
	newData := `{"new":true}`
	err = svc.SetChartData(p.ID, newData, "json")
	if err != nil {
		t.Fatalf("SetChartData failed: %v", err)
	}

	// Verify data was updated
	if store.chartData[p.ID] != newData {
		t.Errorf("data not updated: got %q, want %q", store.chartData[p.ID], newData)
	}

	// Verify timestamp was updated
	meta := store.metadata[p.ID]
	if !meta.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should have been updated")
	}
}

func TestSetChartData_NonexistentPost(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	err := svc.SetChartData("nonexistent-a1b2", "data", "json")
	if err == nil {
		t.Fatal("expected error for nonexistent post")
	}
}

func TestSetChartData_NonChartPost(t *testing.T) {
	store := newMockStorage()
	svc := NewService(store)

	// Create a regular image post (not a chart)
	_, err := svc.CreatePost("Regular Post", "<html></html>", 800, 600)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	// Find the post ID (we don't know it exactly due to random suffix)
	var postID string
	for id := range store.posts {
		postID = id
		break
	}

	err = svc.SetChartData(postID, "data", "json")
	if err == nil {
		t.Fatal("expected error when setting chart data on non-chart post")
	}
}
