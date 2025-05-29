package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/PratikDev/transcoder/types"
)

// StatusManager handles tracking and broadcasting transcoding progress.
type StatusManager struct {
	tasks       map[string]types.TaskStatus                     // Store last known status for each task
	subscribers map[string]map[chan types.StatusUpdate]struct{} // Map of taskID to a map of subscriber channels
	mu          sync.RWMutex                                    // Mutex for concurrent access to maps
}

// NewStatusManager creates and returns a new StatusManager instance.
func NewStatusManager() *StatusManager {
	return &StatusManager{
		tasks:       make(map[string]types.TaskStatus),
		subscribers: make(map[string]map[chan types.StatusUpdate]struct{}),
	}
}

// RegisterSubscriber registers a new client subscriber for a given taskID.
// It returns a read-only channel where updates will be sent.
func (sm *StatusManager) RegisterSubscriber(taskID string) (chan types.StatusUpdate, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Initialize subscriber map for this taskID if it doesn't exist
	if _, ok := sm.subscribers[taskID]; !ok {
		sm.subscribers[taskID] = make(map[chan types.StatusUpdate]struct{})
	}

	// Create a buffered channel to prevent blocking the sender if the receiver is slow
	// Buffer size can be tuned. A small buffer prevents excessive buffering.
	clientChan := make(chan types.StatusUpdate, 5) // Buffer 5 updates

	sm.subscribers[taskID][clientChan] = struct{}{}
	log.Printf("New subscriber registered for task: %s", taskID)

	// Send the last known status immediately to the new subscriber
	if status, ok := sm.tasks[taskID]; ok {
		select {
		case clientChan <- status.LastUpdate:
			// Sent successfully
		default:
			log.Printf("Failed to send initial status to a slow subscriber for task: %s", taskID)
		}
	}

	return clientChan, nil
}

// DeregisterSubscriber removes a client subscriber for a given taskID.
func (sm *StatusManager) DeregisterSubscriber(taskID string, clientChan chan types.StatusUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if chans, ok := sm.subscribers[taskID]; ok {
		delete(chans, clientChan) // Remove the subscriber channel
		close(clientChan)         // Close the channel to signal done to client
		if len(chans) == 0 {
			delete(sm.subscribers, taskID) // Clean up if no more subscribers for this task
			log.Printf("All subscribers deregistered for task: %s", taskID)
		}
	}
	log.Printf("Subscriber deregistered for task: %s", taskID)
}

// SendUpdate broadcasts a status update for a specific taskID to all its subscribers.
func (sm *StatusManager) SendUpdate(taskID string, update types.StatusUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	update.Timestamp = time.Now().UnixMilli() // Set timestamp for the update

	// Update the last known status for this task
	sm.tasks[taskID] = types.TaskStatus{LastUpdate: update}

	// Iterate over all subscribers for this task and send the update
	if chans, ok := sm.subscribers[taskID]; ok {
		for clientChan := range chans {
			select {
			case clientChan <- update:
				// Sent successfully
			default:
				// If the client's channel is full, skip sending to avoid blocking
				log.Printf("Skipping update for a slow subscriber for task %s, channel full.", taskID)
			}
		}
	} else {
		// If no subscribers, just log the update (useful for tasks that might run unattended)
		jsonUpdate, _ := json.Marshal(update)
		log.Printf("No subscribers for task %s, last update: %s", taskID, jsonUpdate)
	}
}

// RemoveTask clears a task's status and subscribers when it's fully done.
func (sm *StatusManager) RemoveTask(taskID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.tasks, taskID)
	// Subscribers should ideally be handled by DeregisterSubscriber, but this ensures cleanup
	if chans, ok := sm.subscribers[taskID]; ok {
		for clientChan := range chans {
			close(clientChan) // Close each subscriber channel
		}
		delete(sm.subscribers, taskID)
	}
	log.Printf("Task %s and its status/subscribers removed.", taskID)
}
