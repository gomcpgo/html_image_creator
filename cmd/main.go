package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"html_image_creator/pkg/config"
	mcpHandler "html_image_creator/pkg/handler"
	"html_image_creator/pkg/screenshot"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/protocol"
	"github.com/gomcpgo/mcp/pkg/server"
)

func main() {
	// Define terminal mode flags
	var (
		createPost   string
		updatePost   string
		htmlContent  string
		width        int
		height       int
		listPosts    bool
		getPost      string
		exportPost   string
		exportOutput string
		addMedia     string
		mediaPath    string
	)

	flag.StringVar(&createPost, "create", "", "Create a new image post with the specified name")
	flag.StringVar(&updatePost, "update", "", "Update post with the specified ID")
	flag.StringVar(&htmlContent, "html", "", "HTML content for create/update operations")
	flag.IntVar(&width, "width", 0, "Canvas width in pixels (required for create)")
	flag.IntVar(&height, "height", 0, "Canvas height in pixels (required for create)")
	flag.BoolVar(&listPosts, "list", false, "List all image posts")
	flag.StringVar(&getPost, "get", "", "Get image post by ID")
	flag.StringVar(&exportPost, "export", "", "Export image post by ID")
	flag.StringVar(&exportOutput, "output", "", "Output path for export")
	flag.StringVar(&addMedia, "add-media", "", "Add media to post (specify post ID)")
	flag.StringVar(&mediaPath, "media-path", "", "Path to media file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create handler
	screenshotSvc := screenshot.NewScreenshotter()
	h := mcpHandler.NewHandler(cfg, screenshotSvc)
	ctx := context.Background()

	// Terminal mode operations
	if createPost != "" {
		if htmlContent == "" {
			log.Fatal("--html is required when creating a post")
		}
		if width <= 0 || height <= 0 {
			log.Fatal("--width and --height are required when creating a post")
		}
		runTerminalCommand(ctx, h, "create_image_post", map[string]interface{}{
			"name":         createPost,
			"html_content": htmlContent,
			"width":        float64(width),
			"height":       float64(height),
		})
		return
	}

	if updatePost != "" {
		if htmlContent == "" {
			log.Fatal("--html is required when updating a post")
		}
		runTerminalCommand(ctx, h, "update_image_post", map[string]interface{}{
			"post_id":      updatePost,
			"html_content": htmlContent,
		})
		return
	}

	if listPosts {
		runTerminalCommand(ctx, h, "list_image_posts", map[string]interface{}{})
		return
	}

	if getPost != "" {
		runTerminalCommand(ctx, h, "get_image_post", map[string]interface{}{
			"post_id": getPost,
		})
		return
	}

	if exportPost != "" {
		if exportOutput == "" {
			log.Fatal("--output is required when exporting")
		}
		runTerminalCommand(ctx, h, "export_image", map[string]interface{}{
			"post_id":     exportPost,
			"output_path": exportOutput,
		})
		return
	}

	if addMedia != "" {
		if mediaPath == "" {
			log.Fatal("--media-path is required when adding media")
		}
		runTerminalCommand(ctx, h, "add_media", map[string]interface{}{
			"post_id":     addMedia,
			"source_path": mediaPath,
		})
		return
	}

	// MCP Server mode (default)
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(h)

	srv := server.New(server.Options{
		Name:     "html-image-creator",
		Version:  "1.0.0",
		Registry: registry,
	})

	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// runTerminalCommand executes a tool command in terminal mode
func runTerminalCommand(ctx context.Context, h *mcpHandler.Handler, toolName string, args map[string]interface{}) {
	req := &protocol.CallToolRequest{
		Name:      toolName,
		Arguments: args,
	}

	resp, err := h.CallTool(ctx, req)
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	for _, content := range resp.Content {
		if content.Type == "text" {
			var data interface{}
			if err := json.Unmarshal([]byte(content.Text), &data); err == nil {
				pretty, _ := json.MarshalIndent(data, "", "  ")
				fmt.Println(string(pretty))
			} else {
				fmt.Println(content.Text)
			}
		}
	}
}
