package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/PratikDev/transcoder/services/utils"
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

	// Check if the task is active and known.
	// A task is considered active if it exists in the sm.tasks map.
	// SendUpdate adds tasks to sm.tasks, and RemoveTask deletes them.
	currentStatus, taskExists := sm.tasks[taskID]
	if !taskExists {
		// If task is not in sm.tasks, it means it hasn't received its first update,
		// has already completed and been removed, or never existed.
		log.Printf("Attempt to subscribe to non-existent or inactive task: %s", taskID)
		return nil, fmt.Errorf("task '%s' not found or not active", taskID)
	}

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
	// We already fetched currentStatus and know taskExists is true.
	select {
	case clientChan <- currentStatus.LastUpdate:
		// Sent successfully
	default:
		log.Printf("Failed to send initial status to a new subscriber for task: %s (channel: %p). Channel might be full or closed.", taskID, clientChan)
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
	// Only update the LastUpdate field, preserving other fields like Cancel
	task := sm.tasks[taskID]
	task.LastUpdate = update
	sm.tasks[taskID] = task

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

	// Clean up the cancel function if it exists to prevent memory leaks
	if task, ok := sm.tasks[taskID]; ok {
		if task.Cancel != nil {
			// This is good practice although the context is likely already done.
			task.Cancel()
		}
	}

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

// CancelTask finds the cancel function for a task and executes it.
func (sm *StatusManager) CancelTask(taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, ok := sm.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Check if the cancel function is not registered
	// This is a safeguard; ideally, the cancel function should always be set
	// when the task is created
	if task.Cancel == nil {
		return fmt.Errorf("no cancel function registered for task %s", taskID)
	}

	task.Cancel() // Execute the context cancel function
	log.Printf("Cancellation signal sent for task: %s", taskID)

	// remove the output directory for this task
	if err := utils.RemoveOutputDirectory(taskID); err != nil {
		errMsg := fmt.Sprintf("failed to remove output directory for task %s: %v", taskID, err)
		log.Println(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// StoreCancelFunc stores the cancel function for a given taskID.
func (sm *StatusManager) StoreCancelFunc(taskID string, cancel context.CancelFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// It's possible the first status update hasn't happened yet,
	// so we ensure the task entry exists.
	if task, ok := sm.tasks[taskID]; ok {
		log.Printf("Storing cancel function for task: %s", taskID)

		task.Cancel = cancel
		sm.tasks[taskID] = task
	} else {
		log.Printf("Creating new task entry for cancel function: %s", taskID)

		sm.tasks[taskID] = types.TaskStatus{Cancel: cancel}
	}
}
