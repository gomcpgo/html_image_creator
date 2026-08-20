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
			Description: "Create a new HTML image post with fixed canvas dimensions. The LLM generates HTML/CSS content that will be rendered at the specified width and height. Common sizes: 1080x1080 (Instagram square), 1080x1920 (Instagram story), 1200x628 (Facebook/LinkedIn), 1280x720 (YouTube thumbnail). The HTML should use Google Fonts via <link> tags for consistent rendering. To include local or generated images (e.g., from AI image generation tools), pass their absolute file paths in media_files. Reference them in the HTML as media/filename.ext (e.g., <img src=\"media/photo.png\"> or background-image: url('media/landscape.jpg')).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the image post"
					},
					"html_content": {
						"type": "string",
						"description": "Full HTML/CSS content for the image. Use inline styles or <style> tags. The content will be rendered at the exact specified dimensions. Reference media files as media/filename.ext."
					},
					"width": {
						"type": "integer",
						"description": "Canvas width in pixels (e.g., 1080)"
					},
					"height": {
						"type": "integer",
						"description": "Canvas height in pixels (e.g., 1080)"
					},
					"media_files": {
						"type": "array",
						"items": { "type": "string" },
						"description": "Optional list of absolute file paths to copy into the post's media folder. Each file becomes available as media/filename.ext in the HTML."
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
			Name:        "create_chart",
			Description: "Create a data visualisation chart. The LLM generates HTML/JS using bundled chart libraries. Use <script src=\"libs/chart.min.js\"> for Chart.js or <script src=\"libs/d3.min.js\"> for D3. Load data via fetch('data.json') or fetch('data.csv'). Signal render completion by setting document.title = 'ready' after the chart finishes drawing. Common dimensions: 800x600 (standard), 1200x800 (presentation), 1080x1080 (square).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of the chart"
					},
					"html_content": {
						"type": "string",
						"description": "Full HTML/JS content for the chart. Use bundled libraries via libs/chart.min.js or libs/d3.min.js. Load data with fetch('data.json') or fetch('data.csv'). Set document.title = 'ready' after chart renders."
					},
					"width": {
						"type": "integer",
						"description": "Canvas width in pixels (e.g., 800)"
					},
					"height": {
						"type": "integer",
						"description": "Canvas height in pixels (e.g., 600)"
					},
					"data": {
						"type": "string",
						"description": "Chart data as JSON array/object or CSV string"
					},
					"data_format": {
						"type": "string",
						"enum": ["json", "csv"],
						"description": "Format of the data: json or csv"
					},
					"media_files": {
						"type": "array",
						"items": { "type": "string" },
						"description": "Optional list of absolute file paths to copy into the post's media folder."
					}
				},
				"required": ["name", "html_content", "width", "height", "data", "data_format"]
			}`),
		},
		{
			Name:        "set_chart_data",
			Description: "Update the data for an existing chart post without changing the HTML template. After updating, call export_image to re-render with the new data.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"post_id": {
						"type": "string",
						"description": "The unique post ID of the chart"
					},
					"data": {
						"type": "string",
						"description": "New chart data as JSON array/object or CSV string"
					},
					"data_format": {
						"type": "string",
						"enum": ["json", "csv"],
						"description": "Format of the data: json or csv"
					}
				},
				"required": ["post_id", "data", "data_format"]
			}`),
		},
		{
			Name:        "render_frames",
			Description: "Render a flipbook of frames from a spec file: per scene, a base HTML plus cumulative deltas (set_class/set_html/append_html/remove ops), rendered in one headless-Chrome page so untouched elements stay pixel-identical across frames. Runs declarative layout validations (in_canvas, no_overlap, keep_out, max_lines, plus an implicit untouched-elements-must-not-move check) against real getBoundingClientRect geometry and reports violations attributed to the frame that introduced them. Spec format: docs/render-frames-spec.md. Use validate_only for a fast layout audit without writing PNGs.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"spec_file": {
						"type": "string",
						"description": "Absolute path to the spec JSON file (scenes, deltas, validations, output_dir, optional asset_dir served as the web root for images/CSS/fonts)"
					},
					"validate_only": {
						"type": "boolean",
						"description": "If true, apply deltas and run validations but write no PNGs (default false)"
					}
				},
				"required": ["spec_file"]
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
