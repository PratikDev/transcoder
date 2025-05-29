package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PratikDev/transcoder/services"
	"github.com/PratikDev/transcoder/types"
	"github.com/google/uuid"
)

const (
	uploadDir         = "./uploads" // Directory to temporarily store uploaded videos
	outputDir         = "./output"  // Directory for transcoded output
	serverPort        = ":3000"     // Port for the API server
	maxUploadSize     = 20          // Maximum upload size in MB
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
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory %s: %v", uploadDir, err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory %s: %v", outputDir, err)
	}

	http.HandleFunc("/transcode", handleTranscode)
	http.HandleFunc("/transcode/status/", handleTranscodeStatusStream) // SSE endpoint
	http.HandleFunc("/status", handleServerStatus)                     // Optional: for checking server health

	log.Printf("Server starting on port %s", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, nil))
}

func handleTranscode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form data (for file upload)
	err := r.ParseMultipartForm(maxUploadSize << 20) // Convert MB to bytes
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile(fileFormFieldName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get video file from form: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Extract file info
	fileName := header.Filename
	extName := strings.ToLower(filepath.Ext(fileName))
	baseName := strings.TrimSuffix(fileName, extName)
	epochMillis := time.Now().UnixNano()
	uniqueFileName := fmt.Sprintf("%s_%d%s", baseName, epochMillis, extName)
	tempFilePath := filepath.Join(uploadDir, uniqueFileName)

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

	taskID := uuid.New().String()

	log.Printf("Received file: %s, saved to %s. Assigned Task ID: %s", fileName, tempFilePath, taskID)

	// Prepare TranscoderSource
	source := types.TranscoderSource{
		File:     tempFilePath,
		Filename: fileName,
		Extname:  extName,
	}

	// Initiate transcoding in a goroutine (non-blocking)
	go func() {
		// This defer ensures the temp file is removed after the goroutine finishes,
		// regardless of whether transcoding succeeded or failed.
		defer func() {
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

		transcoder := services.NewTranscoder(source, outputDir, statusManager, taskID)
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
		transcoder.Process()

		elapsedTime := time.Since(startTime)
		log.Printf("[%s] Transcoding for %s completed. Total time: %s", taskID, fileName, elapsedTime)
	}()

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

func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET requests are allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Transcoder API is running!")
}
