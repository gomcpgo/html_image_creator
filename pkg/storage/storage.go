package storage

import (
	"encoding/json"
	"fmt"
	"html_image_creator/pkg/post"
	"io"
	"os"
	"path/filepath"
)

// Storage handles file operations for image posts
type Storage struct {
	rootDir string
}

// NewStorage creates a new Storage instance
func NewStorage(rootDir string) *Storage {
	return &Storage{
		rootDir: rootDir,
	}
}

// GetPostPath returns the directory path for a post
func (s *Storage) GetPostPath(postID string) string {
	return filepath.Join(s.rootDir, postID)
}

// GetHTMLPath returns the path to the index.html file
func (s *Storage) GetHTMLPath(postID string) string {
	return filepath.Join(s.GetPostPath(postID), "index.html")
}

// GetMetadataPath returns the path to the metadata.json file
func (s *Storage) GetMetadataPath(postID string) string {
	return filepath.Join(s.GetPostPath(postID), "metadata.json")
}

// GetMediaDir returns the path to the media directory
func (s *Storage) GetMediaDir(postID string) string {
	return filepath.Join(s.GetPostPath(postID), "media")
}

// PostExists checks if a post exists
func (s *Storage) PostExists(postID string) bool {
	htmlPath := s.GetHTMLPath(postID)
	_, err := os.Stat(htmlPath)
	return err == nil
}

// CreatePost creates a new post on disk
func (s *Storage) CreatePost(p *post.ImagePost) error {
	postPath := s.GetPostPath(p.ID)
	if err := os.MkdirAll(postPath, 0755); err != nil {
		return fmt.Errorf("failed to create post directory: %w", err)
	}

	mediaDir := s.GetMediaDir(p.ID)
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return fmt.Errorf("failed to create media directory: %w", err)
	}

	htmlPath := s.GetHTMLPath(p.ID)
	if err := os.WriteFile(htmlPath, []byte(p.HTMLContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	metadata := post.Metadata{
		Name:      p.Name,
		Width:     p.Width,
		Height:    p.Height,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if err := s.writeMetadata(p.ID, &metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// UpdatePost updates an existing post's HTML content and metadata
func (s *Storage) UpdatePost(p *post.ImagePost) error {
	if !s.PostExists(p.ID) {
		return fmt.Errorf("post %s does not exist", p.ID)
	}

	htmlPath := s.GetHTMLPath(p.ID)
	if err := os.WriteFile(htmlPath, []byte(p.HTMLContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	metadata := post.Metadata{
		Name:      p.Name,
		Width:     p.Width,
		Height:    p.Height,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if err := s.writeMetadata(p.ID, &metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// GetPost retrieves a post from disk
func (s *Storage) GetPost(postID string) (*post.ImagePost, error) {
	if !s.PostExists(postID) {
		return nil, fmt.Errorf("post %s does not exist", postID)
	}

	htmlPath := s.GetHTMLPath(postID)
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTML file: %w", err)
	}

	metadata, err := s.readMetadata(postID)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return &post.ImagePost{
		ID:          postID,
		Name:        metadata.Name,
		HTMLContent: string(htmlBytes),
		Width:       metadata.Width,
		Height:      metadata.Height,
		CreatedAt:   metadata.CreatedAt,
		UpdatedAt:   metadata.UpdatedAt,
	}, nil
}

// ListPosts returns all posts
func (s *Storage) ListPosts() ([]*post.PostInfo, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read root directory: %w", err)
	}

	var posts []*post.PostInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		postID := entry.Name()
		if !s.PostExists(postID) {
			continue
		}

		metadata, err := s.readMetadata(postID)
		if err != nil {
			continue // Skip posts with invalid metadata
		}

		posts = append(posts, &post.PostInfo{
			ID:        postID,
			Name:      metadata.Name,
			Width:     metadata.Width,
			Height:    metadata.Height,
			CreatedAt: metadata.CreatedAt,
			UpdatedAt: metadata.UpdatedAt,
			FilePath:  filepath.Join(postID, "index.html"),
		})
	}

	return posts, nil
}

// CopyMediaFile copies a media file to the post's media directory
func (s *Storage) CopyMediaFile(postID, sourcePath string) (string, error) {
	if !s.PostExists(postID) {
		return "", fmt.Errorf("post %s does not exist", postID)
	}

	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	filename := filepath.Base(sourcePath)
	mediaDir := s.GetMediaDir(postID)
	destPath := filepath.Join(mediaDir, filename)

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	relativePath := filepath.Join("media", filename)
	return relativePath, nil
}

// DeletePost deletes a post and all its files
func (s *Storage) DeletePost(postID string) error {
	if !s.PostExists(postID) {
		return fmt.Errorf("post %s does not exist", postID)
	}

	postPath := s.GetPostPath(postID)
	if err := os.RemoveAll(postPath); err != nil {
		return fmt.Errorf("failed to delete post directory: %w", err)
	}

	return nil
}

func (s *Storage) writeMetadata(postID string, metadata *post.Metadata) error {
	metadataPath := s.GetMetadataPath(postID)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// WriteChartData writes chart data to a file in the post directory
func (s *Storage) WriteChartData(postID string, data string, format string) error {
	if !s.PostExists(postID) {
		return fmt.Errorf("post %s does not exist", postID)
	}
	if format != "json" && format != "csv" {
		return fmt.Errorf("invalid data format %q: must be json or csv", format)
	}

	filename := "data." + format
	dataPath := filepath.Join(s.GetPostPath(postID), filename)
	if err := os.WriteFile(dataPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write chart data: %w", err)
	}
	return nil
}

// ReadChartData reads chart data from the post directory (tries data.json then data.csv)
func (s *Storage) ReadChartData(postID string) (string, error) {
	if !s.PostExists(postID) {
		return "", fmt.Errorf("post %s does not exist", postID)
	}

	postPath := s.GetPostPath(postID)
	for _, ext := range []string{"json", "csv"} {
		dataPath := filepath.Join(postPath, "data."+ext)
		content, err := os.ReadFile(dataPath)
		if err == nil {
			return string(content), nil
		}
	}
	return "", fmt.Errorf("no chart data found for post %s", postID)
}

// UpdateMetadata writes updated metadata for a post
func (s *Storage) UpdateMetadata(postID string, meta *post.Metadata) error {
	if !s.PostExists(postID) {
		return fmt.Errorf("post %s does not exist", postID)
	}
	return s.writeMetadata(postID, meta)
}

func (s *Storage) readMetadata(postID string) (*post.Metadata, error) {
	metadataPath := s.GetMetadataPath(postID)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata post.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}
