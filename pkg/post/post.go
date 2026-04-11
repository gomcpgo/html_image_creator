package post

import (
	"fmt"
	"time"
)

// Service provides image post operations
type Service struct {
	storage StorageInterface
}

// StorageInterface defines the storage operations needed by the service
type StorageInterface interface {
	PostExists(postID string) bool
	CreatePost(post *ImagePost) error
	UpdatePost(post *ImagePost) error
	GetPost(postID string) (*ImagePost, error)
	ListPosts() ([]*PostInfo, error)
	CopyMediaFile(postID, sourcePath string) (string, error)
	DeletePost(postID string) error
	GetPostPath(postID string) string
	GetHTMLPath(postID string) string
	WriteChartData(postID string, data string, format string) error
	ReadChartData(postID string) (string, error)
	UpdateMetadata(postID string, meta *Metadata) error
}

// NewService creates a new post service
func NewService(storage StorageInterface) *Service {
	return &Service{
		storage: storage,
	}
}

// CreatePost creates a new image post with fixed canvas dimensions
func (s *Service) CreatePost(name, htmlContent string, width, height int) (*ImagePost, error) {
	if name == "" {
		return nil, fmt.Errorf("post name cannot be empty")
	}
	if htmlContent == "" {
		return nil, fmt.Errorf("HTML content cannot be empty")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("width and height must be positive integers")
	}

	postID := GeneratePostID(name, s.storage.PostExists)

	now := time.Now()
	p := &ImagePost{
		ID:          postID,
		Name:        name,
		HTMLContent: htmlContent,
		Width:       width,
		Height:      height,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.storage.CreatePost(p); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return p, nil
}

// UpdatePost updates an existing post's HTML content (dimensions are immutable)
func (s *Service) UpdatePost(postID, htmlContent string) (*ImagePost, error) {
	if !ValidatePostID(postID) {
		return nil, fmt.Errorf("invalid post ID: %s", postID)
	}
	if htmlContent == "" {
		return nil, fmt.Errorf("HTML content cannot be empty")
	}

	// Get existing post to preserve metadata including dimensions
	p, err := s.storage.GetPost(postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	p.HTMLContent = htmlContent
	p.UpdatedAt = time.Now()

	if err := s.storage.UpdatePost(p); err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	return p, nil
}

// GetPost retrieves a post by ID
func (s *Service) GetPost(postID string) (*ImagePost, error) {
	if !ValidatePostID(postID) {
		return nil, fmt.Errorf("invalid post ID: %s", postID)
	}

	p, err := s.storage.GetPost(postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return p, nil
}

// ListPosts returns all posts
func (s *Service) ListPosts() ([]*PostInfo, error) {
	posts, err := s.storage.ListPosts()
	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}
	return posts, nil
}

// AddMedia adds a media file to a post and returns the relative path
func (s *Service) AddMedia(postID, sourcePath string) (string, error) {
	if !ValidatePostID(postID) {
		return "", fmt.Errorf("invalid post ID: %s", postID)
	}
	if sourcePath == "" {
		return "", fmt.Errorf("source path cannot be empty")
	}

	relativePath, err := s.storage.CopyMediaFile(postID, sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to add media: %w", err)
	}

	return relativePath, nil
}

// DeletePost deletes a post
func (s *Service) DeletePost(postID string) error {
	if !ValidatePostID(postID) {
		return fmt.Errorf("invalid post ID: %s", postID)
	}

	if err := s.storage.DeletePost(postID); err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	return nil
}

// CreateChart creates a new chart post with separate data storage
func (s *Service) CreateChart(name, htmlContent string, width, height int, data, dataFormat string) (*ImagePost, error) {
	if data == "" {
		return nil, fmt.Errorf("chart data cannot be empty")
	}
	if dataFormat != "json" && dataFormat != "csv" {
		return nil, fmt.Errorf("invalid data format %q: must be json or csv", dataFormat)
	}

	p, err := s.CreatePost(name, htmlContent, width, height)
	if err != nil {
		return nil, err
	}

	if err := s.storage.WriteChartData(p.ID, data, dataFormat); err != nil {
		return nil, fmt.Errorf("failed to write chart data: %w", err)
	}

	// Update metadata with chart flags
	meta := &Metadata{
		Name:         p.Name,
		Width:        p.Width,
		Height:       p.Height,
		HasChartData: true,
		DataFormat:   dataFormat,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
	if err := s.storage.UpdateMetadata(p.ID, meta); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	return p, nil
}

// SetChartData updates the data for an existing chart post
func (s *Service) SetChartData(postID, data, dataFormat string) error {
	if !ValidatePostID(postID) {
		return fmt.Errorf("invalid post ID: %s", postID)
	}
	if data == "" {
		return fmt.Errorf("chart data cannot be empty")
	}
	if dataFormat != "json" && dataFormat != "csv" {
		return fmt.Errorf("invalid data format %q: must be json or csv", dataFormat)
	}

	p, err := s.storage.GetPost(postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	// Read existing metadata to check if this is a chart post
	// We check by trying to read existing chart data
	if _, err := s.storage.ReadChartData(postID); err != nil {
		return fmt.Errorf("post %s is not a chart post: no existing chart data", postID)
	}

	if err := s.storage.WriteChartData(postID, data, dataFormat); err != nil {
		return fmt.Errorf("failed to write chart data: %w", err)
	}

	now := time.Now()
	meta := &Metadata{
		Name:         p.Name,
		Width:        p.Width,
		Height:       p.Height,
		HasChartData: true,
		DataFormat:   dataFormat,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    now,
	}
	if err := s.storage.UpdateMetadata(postID, meta); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	return nil
}

// GetPostPath returns the absolute path to the post directory
func (s *Service) GetPostPath(postID string) string {
	return s.storage.GetPostPath(postID)
}

// GetHTMLPath returns the absolute path to the HTML file
func (s *Service) GetHTMLPath(postID string) string {
	return s.storage.GetHTMLPath(postID)
}
