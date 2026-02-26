package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html_image_creator/pkg/config"
	"html_image_creator/pkg/post"
	"html_image_creator/pkg/storage"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Handler implements the MCP protocol for HTML Image Creator
type Handler struct {
	config        *config.Config
	postSvc       *post.Service
	screenshotSvc ScreenshotService
}

// ScreenshotService defines the interface for screenshot functionality
type ScreenshotService interface {
	TakeScreenshot(postDir string, width, height int, outputPath string) error
}

// NewHandler creates a new handler instance
func NewHandler(cfg *config.Config, screenshotSvc ScreenshotService) *Handler {
	store := storage.NewStorage(cfg.RootDir)
	postSvc := post.NewService(store)

	return &Handler{
		config:        cfg,
		postSvc:       postSvc,
		screenshotSvc: screenshotSvc,
	}
}

// ListTools returns the list of available tools
func (h *Handler) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	tools := h.GetTools()
	return &protocol.ListToolsResponse{
		Tools: tools,
	}, nil
}

// CallTool handles tool invocations
func (h *Handler) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	switch req.Name {
	case "create_image_post":
		return h.handleCreateImagePost(ctx, req.Arguments)
	case "update_image_post":
		return h.handleUpdateImagePost(ctx, req.Arguments)
	case "get_image_post":
		return h.handleGetImagePost(ctx, req.Arguments)
	case "list_image_posts":
		return h.handleListImagePosts(ctx, req.Arguments)
	case "export_image":
		return h.handleExportImage(ctx, req.Arguments)
	case "add_media":
		return h.handleAddMedia(ctx, req.Arguments)
	default:
		return nil, fmt.Errorf("unknown tool: %s", req.Name)
	}
}

func (h *Handler) handleCreateImagePost(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("name is required and must be a string")
	}

	htmlContent, ok := args["html_content"].(string)
	if !ok || htmlContent == "" {
		return nil, fmt.Errorf("html_content is required and must be a string")
	}

	widthFloat, ok := args["width"].(float64)
	if !ok {
		return nil, fmt.Errorf("width is required and must be an integer")
	}
	width := int(widthFloat)

	heightFloat, ok := args["height"].(float64)
	if !ok {
		return nil, fmt.Errorf("height is required and must be an integer")
	}
	height := int(heightFloat)

	p, err := h.postSvc.CreatePost(name, htmlContent, width, height)
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to create image post: %v", err)), nil
	}

	result := map[string]interface{}{
		"status":     "succeeded",
		"post_id":    p.ID,
		"name":       p.Name,
		"width":      p.Width,
		"height":     p.Height,
		"file_path":  h.postSvc.GetHTMLPath(p.ID),
		"created_at": p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at": p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return h.successResponse(result), nil
}

func (h *Handler) handleUpdateImagePost(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	postID, ok := args["post_id"].(string)
	if !ok || postID == "" {
		return nil, fmt.Errorf("post_id is required and must be a string")
	}

	htmlContent, ok := args["html_content"].(string)
	if !ok || htmlContent == "" {
		return nil, fmt.Errorf("html_content is required and must be a string")
	}

	p, err := h.postSvc.UpdatePost(postID, htmlContent)
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to update image post: %v", err)), nil
	}

	result := map[string]interface{}{
		"status":     "succeeded",
		"post_id":    p.ID,
		"name":       p.Name,
		"width":      p.Width,
		"height":     p.Height,
		"file_path":  h.postSvc.GetHTMLPath(p.ID),
		"updated_at": p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return h.successResponse(result), nil
}

func (h *Handler) handleGetImagePost(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	postID, ok := args["post_id"].(string)
	if !ok || postID == "" {
		return nil, fmt.Errorf("post_id is required and must be a string")
	}

	p, err := h.postSvc.GetPost(postID)
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to get image post: %v", err)), nil
	}

	result := map[string]interface{}{
		"status":       "succeeded",
		"post_id":      p.ID,
		"name":         p.Name,
		"html_content": p.HTMLContent,
		"width":        p.Width,
		"height":       p.Height,
		"file_path":    h.postSvc.GetHTMLPath(p.ID),
		"created_at":   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":   p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return h.successResponse(result), nil
}

func (h *Handler) handleListImagePosts(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	posts, err := h.postSvc.ListPosts()
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to list image posts: %v", err)), nil
	}

	postsList := make([]map[string]interface{}, len(posts))
	for i, p := range posts {
		postsList[i] = map[string]interface{}{
			"post_id":    p.ID,
			"name":       p.Name,
			"width":      p.Width,
			"height":     p.Height,
			"file_path":  h.postSvc.GetHTMLPath(p.ID),
			"created_at": p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at": p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	result := map[string]interface{}{
		"status": "succeeded",
		"count":  len(postsList),
		"posts":  postsList,
	}

	return h.successResponse(result), nil
}

func (h *Handler) handleExportImage(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	postID, ok := args["post_id"].(string)
	if !ok || postID == "" {
		return nil, fmt.Errorf("post_id is required and must be a string")
	}

	outputPath, ok := args["output_path"].(string)
	if !ok || outputPath == "" {
		return nil, fmt.Errorf("output_path is required and must be a string")
	}

	// Get post to read dimensions
	p, err := h.postSvc.GetPost(postID)
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to get post: %v", err)), nil
	}

	// Take screenshot
	postDir := h.postSvc.GetPostPath(postID)
	if err := h.screenshotSvc.TakeScreenshot(postDir, p.Width, p.Height, outputPath); err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to export image: %v", err)), nil
	}

	result := map[string]interface{}{
		"status":      "succeeded",
		"post_id":     postID,
		"output_path": outputPath,
	}

	return h.successResponse(result), nil
}

func (h *Handler) handleAddMedia(ctx context.Context, args map[string]interface{}) (*protocol.CallToolResponse, error) {
	postID, ok := args["post_id"].(string)
	if !ok || postID == "" {
		return nil, fmt.Errorf("post_id is required and must be a string")
	}

	sourcePath, ok := args["source_path"].(string)
	if !ok || sourcePath == "" {
		return nil, fmt.Errorf("source_path is required and must be a string")
	}

	relativePath, err := h.postSvc.AddMedia(postID, sourcePath)
	if err != nil {
		return h.errorResponse(fmt.Sprintf("Failed to add media: %v", err)), nil
	}

	result := map[string]interface{}{
		"status":        "succeeded",
		"post_id":       postID,
		"relative_path": relativePath,
	}

	return h.successResponse(result), nil
}

// Helper methods

func (h *Handler) successResponse(data map[string]interface{}) *protocol.CallToolResponse {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}
}

func (h *Handler) errorResponse(errorMsg string) *protocol.CallToolResponse {
	data := map[string]interface{}{
		"status": "failed",
		"error":  errorMsg,
	}
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}
}
