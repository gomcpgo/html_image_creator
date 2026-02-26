package handler

import (
	"encoding/json"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// GetTools returns the list of available MCP tools
func (h *Handler) GetTools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "create_image_post",
			Description: "Create a new HTML image post with fixed canvas dimensions. The LLM generates HTML/CSS content that will be rendered at the specified width and height. Common sizes: 1080x1080 (Instagram square), 1080x1920 (Instagram story), 1200x628 (Facebook/LinkedIn), 1280x720 (YouTube thumbnail). The HTML should use Google Fonts via <link> tags for consistent rendering.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the image post"
					},
					"html_content": {
						"type": "string",
						"description": "Full HTML/CSS content for the image. Use inline styles or <style> tags. The content will be rendered at the exact specified dimensions."
					},
					"width": {
						"type": "integer",
						"description": "Canvas width in pixels (e.g., 1080)"
					},
					"height": {
						"type": "integer",
						"description": "Canvas height in pixels (e.g., 1080)"
					}
				},
				"required": ["name", "html_content", "width", "height"]
			}`),
		},
		{
			Name:        "update_image_post",
			Description: "Update the HTML content of an existing image post. Canvas dimensions cannot be changed after creation.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"post_id": {
						"type": "string",
						"description": "The unique post ID"
					},
					"html_content": {
						"type": "string",
						"description": "The new HTML/CSS content"
					}
				},
				"required": ["post_id", "html_content"]
			}`),
		},
		{
			Name:        "get_image_post",
			Description: "Retrieve an image post's content and metadata by ID.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"post_id": {
						"type": "string",
						"description": "The unique post ID"
					}
				},
				"required": ["post_id"]
			}`),
		},
		{
			Name:        "list_image_posts",
			Description: "List all image posts with their metadata.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "export_image",
			Description: "Export an image post as a PNG file. Renders the HTML at exact canvas dimensions using headless Chrome and saves as a pixel-accurate screenshot.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"post_id": {
						"type": "string",
						"description": "The unique post ID"
					},
					"output_path": {
						"type": "string",
						"description": "Absolute path for the output PNG file"
					}
				},
				"required": ["post_id", "output_path"]
			}`),
		},
		{
			Name:        "add_media",
			Description: "Add an image file to a post's media folder. Copies the file and returns the relative path to use in HTML (e.g., in <img src=\"media/photo.jpg\">).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"post_id": {
						"type": "string",
						"description": "The unique post ID"
					},
					"source_path": {
						"type": "string",
						"description": "The absolute path to the source media file"
					}
				},
				"required": ["post_id", "source_path"]
			}`),
		},
	}
}
