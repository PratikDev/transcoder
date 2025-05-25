package main

import (
	"log"
	"time"

	"github.com/PratikDev/transcoder/services"
	"github.com/PratikDev/transcoder/types"
)

func main() {
	source := types.TranscoderSource{
		File:     "./assets/video.mp4",
		Filename: "video.mp4",
		Extname:  "mp4",
	}
	resolutions := []types.Resolutions{720, 480, 360}
	outputDir := "./output"

	transcoder := services.NewTranscoder(source, resolutions, outputDir)

	// Record the start time
	startTime := time.Now()

	transcoder.Process()

	// Calculate the elapsed time
	elapsedTime := time.Since(startTime)

	log.Println("Transcoding process completed.")
	log.Printf("Total transcoding time: %s", elapsedTime) // Log the duration
}
