package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PratikDev/transcoder/services"
	"github.com/PratikDev/transcoder/services/utils"
	"github.com/PratikDev/transcoder/types"
	"github.com/google/uuid"
)

const (
	serverPort        = ":3000" // Port for the API server
	maxUploadSize     = 30      // Maximum upload size in MB
	fileFormFieldName = "video"
)

var (
	statusManager *services.StatusManager
)

func init() {
	// Initialize the global status manager when the program starts
	statusManager = services.NewStatusManager()
}

func main() {
	// Create upload and output directories if they don't exist
	if err := os.MkdirAll(utils.UPLOAD_DIR, 0755); err != nil {
		log.Fatalf("Failed to create upload directory %s: %v", utils.UPLOAD_DIR, err)
	}
	if err := os.MkdirAll(utils.OUTPUT_DIR, 0755); err != nil {
		log.Fatalf("Failed to create output directory %s: %v", utils.OUTPUT_DIR, err)
	}

	http.HandleFunc("/transcode", handleTranscode)                     // Main transcoding endpoint
	http.HandleFunc("/transcode/status/", handleTranscodeStatusStream) // SSE endpoint
	http.HandleFunc("/transcode/jobs/", handleCancelTranscode)         // Endpoint to cancel a transcoding job
	http.HandleFunc("/status", handleServerStatus)                     // For checking server health

	log.Printf("Server starting on port %s", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, nil))
}

func handleTranscode(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all requests to this endpoint
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight (OPTIONS) request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	// Wrap the request body with MaxBytesReader to enforce the upload size limit
	// This limit applies to the entire request body.
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadSize<<20)) // maxUploadSize in MB converted to bytes

	// Parse multipart form data.
	// The maxMemory argument for ParseMultipartForm now dictates how much of the form data
	// (within the MaxBytesReader limit) is stored in memory before spooling to disk.
	// It can be the same as maxUploadSize or smaller if you want to control in-memory usage more granularly.
	err := r.ParseMultipartForm(int64(maxUploadSize << 20)) // Using maxUploadSize for in-memory buffer as well
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			// This error comes from http.MaxBytesReader
			log.Printf("Upload failed: File exceeds maximum allowed size of %d MB. Actual size: %d bytes", maxUploadSize, maxBytesErr.Limit)
			http.Error(w, fmt.Sprintf("Upload failed: File exceeds maximum allowed size of %d MB", maxUploadSize), http.StatusRequestEntityTooLarge)
			return
		}
		// Handle other parsing errors
		log.Printf("Failed to parse form: %v", err)
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile(fileFormFieldName)
	if err != nil {
		log.Printf("Failed to get video file from form: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get video file from form: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	taskID := uuid.New().String()

	// Extract file info
	fileName := header.Filename
	extName := strings.ToLower(filepath.Ext(fileName))
	uniqueFileName := fmt.Sprintf("%s%s", taskID, extName)
	tempFilePath := filepath.Join(utils.UPLOAD_DIR, uniqueFileName)

	// Save the uploaded file temporarily
	dst, err := os.Create(tempFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close() // Close the file after writing
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare TranscoderSource
	source := types.TranscoderSource{
		File:     tempFilePath,
		Filename: fileName,
		Extname:  extName,
	}

	// Create a new context that can be cancelled.
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Store the cancel function in the status manager, keyed by taskID.
	statusManager.StoreCancelFunc(taskID, cancelFunc)

	log.Printf("Received file: %s, saved to %s. Assigned Task ID: %s", fileName, tempFilePath, taskID)

	// Initiate transcoding in a goroutine (non-blocking)
	go func(ctx context.Context, currentTaskID string, currentTempFilePath string, currentFileName string) {
		// This defer ensures the temp file is removed after the goroutine finishes,
		// regardless of whether transcoding succeeded or failed.
		defer func() {
			cancelFunc() // Ensure context resources are freed
			if err := os.Remove(tempFilePath); err != nil {
				log.Printf("[%s] Error removing temporary file %s: %v", taskID, tempFilePath, err)
			} else {
				log.Printf("[%s] Successfully removed temporary file: %s", taskID, tempFilePath)
			}

			// Remove the task from StatusManager when it's completely done
			statusManager.RemoveTask(taskID)
			log.Printf("[%s] Task removed from status manager.", taskID)
		}()

		log.Printf("[%s] Starting transcoding for %s in background...", taskID, fileName)
		startTime := time.Now()

		transcoder := services.NewTranscoder(source, utils.OUTPUT_DIR, statusManager, taskID)
		if transcoder == nil {
			// If transcoder is nil, it means initialization failed for some reason.
			// We need to send a failure status and ensure the task is cleaned up.
			errMsg := fmt.Sprintf("Failed to initialize transcoder for %s", fileName)
			log.Printf("[%s] %s", taskID, errMsg)
			statusManager.SendUpdate(taskID, types.StatusUpdate{
				Type:    "failed",
				Message: errMsg,
			})
		}
		transcoder.Process(ctx)

		elapsedTime := time.Since(startTime)
		log.Printf("[%s] Transcoding for %s completed. Total time: %s", taskID, fileName, elapsedTime)
	}(ctx, taskID, tempFilePath, fileName)

	response := map[string]any{
		"message":         fmt.Sprintf("Transcoding of %s started successfully.", fileName),
		"taskId":          taskID,
		"statusStreamUrl": fmt.Sprintf("/transcode/status/%s", taskID),
	}
	w.WriteHeader(http.StatusAccepted) // 202 Accepted means processing has started
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleTranscodeStatusStream(w http.ResponseWriter, r *http.Request) {
	// Extract taskID from the URL path
	taskID := strings.TrimPrefix(r.URL.Path, "/transcode/status/")
	if taskID == "" {
		log.Println("Task ID is required for status stream")
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: Set CORS policy

	// Register the client with the StatusManager to receive updates
	clientChan, err := statusManager.RegisterSubscriber(taskID)
	if err != nil {
		// Error occurred during registration, likely task not found or not active.
		log.Printf("Error registering subscriber for task %s: %v", taskID, err)
		// Respond with HTTP 404 Not Found if the task is not found or not active.
		http.Error(w, fmt.Sprintf("Cannot subscribe to task status: %s. Task not found, not active, or already completed.", taskID), http.StatusNotFound)
		return
	}

	// Log the successful subscription
	log.Printf("Client connected to status stream for Task ID: %s", taskID)

	// Deregister the client when this handler function returns
	defer statusManager.DeregisterSubscriber(taskID, clientChan)

	// Keep the connection open and send updates
	for {
		select {
		case update, ok := <-clientChan:
			if !ok {
				// Channel has been closed by StatusManager.RemoveTask, meaning the task is done.
				log.Printf("[%s] Status channel closed by manager (task completed or removed). Client handler exiting for channel %p.", taskID, clientChan)
				return // Exit loop, defer will call DeregisterSubscriber
			}

			// Marshal the update struct to JSON
			jsonData, err := json.Marshal(update)
			if err != nil {
				log.Printf("[%s] Error marshalling status update: %v", taskID, err)
				continue // Skip this update, but keep connection alive
			}

			// Send as an SSE event
			_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
			if err != nil {
				// Client disconnected or network error
				log.Printf("[%s] Client disconnected or write error: %v", taskID, err)
				return // Exit the loop and close handler
			}

			// Flush the response writer to send the data immediately
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

		case <-r.Context().Done():
			// Client disconnected
			log.Printf("[%s] Client connection closed.", taskID)
			return // Exit the loop and close handler
		}
	}
}

func handleCancelTranscode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Only DELETE requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/transcode/jobs/")
	if taskID == "" {
		log.Println("Task ID is required for cancellation")
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Received cancellation request for Task ID: %s", taskID)

	err := statusManager.CancelTask(taskID)
	if err != nil {
		log.Printf("Failed to cancel task %s: %v", taskID, err)
		// We send a 404 Not Found if the task doesn't exist to be cancelled.
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Task %s cancelled successfully.\n", taskID)
}

func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET requests are allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Transcoder API is running!")
}
