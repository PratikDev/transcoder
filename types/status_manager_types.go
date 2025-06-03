package types

// TaskStatus represents the current state of a transcoding task.
type TaskStatus struct {
	LastUpdate StatusUpdate
	// You could add more fields here, e.g.,
	// IsFinished bool
	// Error      error
}

type TaskData struct {
	Resolution string  `json:"resolution"` // Target resolution for the transcoding task
	Frame      string  `json:"frame"`      // Ongoing frame for the transcoding task
	Timestamp  int64   `json:"timestamp"`  // Unix timestamp of the video that is being transcoded
	Progress   float64 `json:"progress"`   // Progress of completion for the task (0-100)
}

// StatusUpdate represents a single progress update to be sent to the client via SSE.
type StatusUpdate struct {
	Type      string   `json:"type"`      // e.g., "started", "progress", "completed", "failed"
	Message   string   `json:"message"`   // Detailed message
	Data      TaskData `json:"data"`      // Additional data related to the task
	Timestamp int64    `json:"timestamp"` // Unix timestamp for when the update occurred
}
