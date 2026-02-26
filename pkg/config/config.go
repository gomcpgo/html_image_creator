package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the configuration for the HTML Image Creator
type Config struct {
	RootDir string // Root directory for storing image posts
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	rootDir := os.Getenv("HTML_IMAGE_CREATOR_ROOT_DIR")
	if rootDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		rootDir = filepath.Join(homeDir, ".html_image_posts")
	}

	// Ensure root directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory %s: %w", rootDir, err)
	}

	return &Config{
		RootDir: rootDir,
	}, nil
}
