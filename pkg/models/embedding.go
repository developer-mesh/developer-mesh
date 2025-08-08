package models

import (
	"time"
)

// Embedding represents a vector embedding with additional metadata
type Embedding struct {
	ID           string    `json:"id"`
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	Embedding    []float32 `json:"embedding"`
	ModelID      string    `json:"model_id"`
}

// EmbeddingResponse represents the response from an embedding generation request
type EmbeddingResponse struct {
	EmbeddingID string                 `json:"embedding_id"`
	Vector      []float64              `json:"vector,omitempty"`
	Model       string                 `json:"model"`
	Provider    string                 `json:"provider"`
	Dimensions  int                    `json:"dimensions"`
	TaskType    string                 `json:"task_type"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
}
