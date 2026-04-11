package handler

import (
	"context"
	"encoding/json"
	"html_image_creator/pkg/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

type mockScreenshot struct{}

func (m *mockScreenshot) TakeScreenshot(postDir string, width, height int, outputPath string) error {
	return nil
}

func setupTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{RootDir: tmpDir}
	h := NewHandler(cfg, &mockScreenshot{})
	return h, tmpDir
}

func callTool(t *testing.T, h *Handler, name string, args map[string]interface{}) map[string]interface{} {
	t.Helper()
	resp, err := h.CallTool(context.Background(), &protocol.CallToolRequest{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) failed: %v", name, err)
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(resp.Content[0].Text), &result)
	return result
}

func TestHandleCreateChart(t *testing.T) {
	h, tmpDir := setupTestHandler(t)

	result := callTool(t, h, "create_chart", map[string]interface{}{
		"name":         "Sales Chart",
		"html_content": `<html><body><canvas id="c"></canvas></body></html>`,
		"width":        float64(800),
		"height":       float64(600),
		"data":         `[{"month":"Jan","sales":100}]`,
		"data_format":  "json",
	})

	if result["status"] != "succeeded" {
		t.Fatalf("expected succeeded, got %v: %v", result["status"], result["error"])
	}

	postID := result["post_id"].(string)

	// Verify data.json on disk
	dataPath := filepath.Join(tmpDir, postID, "data.json")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("data.json not found: %v", err)
	}
	if string(content) != `[{"month":"Jan","sales":100}]` {
		t.Errorf("data.json content mismatch")
	}

	if result["has_chart_data"] != true {
		t.Error("expected has_chart_data=true in result")
	}
}

func TestHandleCreateChart_CSV(t *testing.T) {
	h, _ := setupTestHandler(t)

	result := callTool(t, h, "create_chart", map[string]interface{}{
		"name":         "CSV Chart",
		"html_content": "<html></html>",
		"width":        float64(800),
		"height":       float64(600),
		"data":         "month,sales\nJan,100\n",
		"data_format":  "csv",
	})

	if result["status"] != "succeeded" {
		t.Fatalf("expected succeeded, got %v", result["status"])
	}
	if result["data_format"] != "csv" {
		t.Errorf("expected data_format=csv, got %v", result["data_format"])
	}
}

func TestHandleCreateChart_MissingData(t *testing.T) {
	h, _ := setupTestHandler(t)

	_, err := h.CallTool(context.Background(), &protocol.CallToolRequest{
		Name: "create_chart",
		Arguments: map[string]interface{}{
			"name":         "No Data",
			"html_content": "<html></html>",
			"width":        float64(800),
			"height":       float64(600),
			"data_format":  "json",
		},
	})
	if err == nil {
		t.Fatal("expected error for missing data")
	}
}

func TestHandleSetChartData(t *testing.T) {
	h, _ := setupTestHandler(t)

	// Create chart first
	createResult := callTool(t, h, "create_chart", map[string]interface{}{
		"name":         "Update Test",
		"html_content": "<html></html>",
		"width":        float64(800),
		"height":       float64(600),
		"data":         `{"old":true}`,
		"data_format":  "json",
	})
	postID := createResult["post_id"].(string)

	// Update data
	setResult := callTool(t, h, "set_chart_data", map[string]interface{}{
		"post_id":     postID,
		"data":        `{"new":true}`,
		"data_format": "json",
	})

	if setResult["status"] != "succeeded" {
		t.Fatalf("expected succeeded, got %v: %v", setResult["status"], setResult["error"])
	}
}

func TestHandleSetChartData_NonChartPost(t *testing.T) {
	h, _ := setupTestHandler(t)

	// Create regular image post
	createResult := callTool(t, h, "create_image_post", map[string]interface{}{
		"name":         "Regular Post",
		"html_content": "<html></html>",
		"width":        float64(800),
		"height":       float64(600),
	})
	postID := createResult["post_id"].(string)

	// Try to set chart data — should fail
	setResult := callTool(t, h, "set_chart_data", map[string]interface{}{
		"post_id":     postID,
		"data":        `{"test":true}`,
		"data_format": "json",
	})

	if setResult["status"] != "failed" {
		t.Fatal("expected failure for setting chart data on non-chart post")
	}
}
