package post

import "time"

// ImagePost represents an HTML image post with fixed canvas dimensions
type ImagePost struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	HTMLContent string    `json:"html_content"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Metadata represents post metadata stored in metadata.json
type Metadata struct {
	Name      string    `json:"name"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PostInfo is a lightweight post summary for listing
type PostInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	FilePath  string    `json:"file_path"`
}
