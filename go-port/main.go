package main

import (
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
)

const (
	uploadDir         = "./uploads" // Directory to temporarily store uploaded videos
	outputDir         = "./output"  // Directory for transcoded output
	serverPort        = ":3000"     // Port for the API server
	maxUploadSize     = 20          // Maximum upload size in MB
	fileFormFieldName = "video"
)

func main() {
	// Create upload and output directories if they don't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory %s: %v", uploadDir, err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory %s: %v", outputDir, err)
	}

	http.HandleFunc("/transcode", handleTranscode)

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
	defer dst.Close()             // Close the file after writing
	defer os.Remove(tempFilePath) // Clean up temp file after processing
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Received file: %s, saved to %s", fileName, tempFilePath)

	// Prepare TranscoderSource
	source := types.TranscoderSource{
		File:     tempFilePath,
		Filename: fileName,
		Extname:  extName,
	}

	log.Printf("Starting transcoding for %s...", fileName)
	startTime := time.Now()

	// Initiate Transcoding
	transcoder := services.NewTranscoder(source, outputDir)
	transcoder.Process()

	elapsedTime := time.Since(startTime)
	log.Printf("Transcoding for %s completed. Total time: %s", fileName, elapsedTime)

	w.WriteHeader(http.StatusOK)
}
