package types

// TaskStatus represents the current state of a transcoding task.
type TaskStatus struct {
	LastUpdate StatusUpdate
	// You could add more fields here, e.g.,
	// IsFinished bool
	// Error      error
}

// StatusUpdate represents a single progress update to be sent to the client via SSE.
type StatusUpdate struct {
	Type      string  `json:"type"`               // e.g., "started", "progress", "completed", "failed"
	Message   string  `json:"message"`            // Detailed message
	Progress  float64 `json:"progress,omitempty"` // Percentage (0-100)
	Timestamp int64   `json:"timestamp"`          // Unix timestamp for when the update occurred
}
