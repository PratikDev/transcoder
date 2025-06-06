package services

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/PratikDev/transcoder/services/utils"
	"github.com/PratikDev/transcoder/types"
)

// Transcoder handles the video transcoding process.
type Transcoder struct {
	source        types.TranscoderSource
	resolutions   []types.Resolutions
	output        string
	statusMgr     *StatusManager // Reference to the StatusManager
	taskID        string         // Unique ID for this transcoding task
	inputDuration float64        // Store input video duration for progress calculation
}

// NewTranscoder creates a new Transcoder instance.
func NewTranscoder(source types.TranscoderSource, outputDir string, statusMgr *StatusManager, taskID string) *Transcoder {
	// Get video resolution
	vidResolution, err := utils.DetectVideoResolution(source.File)
	if err != nil {
		log.Printf("[error]: failed to detect video resolution for %s: %v", source.File, err)
		return nil
	}

	// Get target targetResolutions based on the detected video resolution
	targetResolutions := utils.GetTargetResolutions(vidResolution)
	if len(targetResolutions) == 0 {
		log.Printf("[error]: no valid resolutions found for %s", source.File)
		return nil
	}

	// Get the input video duration
	inputDuration, err := utils.DetectInputDuration(source.File)
	if err != nil {
		log.Printf("[error]: failed to detect input duration for %s: %v", source.File, err)
		return nil
	}
	if inputDuration <= 0 {
		log.Printf("[error]: invalid input duration for %s: %f", source.File, inputDuration)
		return nil
	}

	return &Transcoder{
		source:        source,
		resolutions:   targetResolutions,
		output:        outputDir,
		statusMgr:     statusMgr,
		taskID:        taskID,
		inputDuration: inputDuration,
	}
}

// Process starts the transcoding process for the source video.
func (t *Transcoder) Process(ctx context.Context) {
	item := t.source
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "started", Message: fmt.Sprintf("Transcoding started for %s", item.Filename)})

	// Create output directory for this task
	outputFolder, err := utils.CreateOutputDirectory(t.taskID)
	if err != nil {
		log.Printf("[failed]: %v", err)
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Failed to create output directory for %s", item.Filename)})
		return
	}

	success := t.transcodeResolutions(ctx, outputFolder)
	if !success {
		// Check if the context was cancelled.
		if ctx.Err() == context.Canceled {
			log.Printf("[cancelled]: Transcoding for %s was cancelled by user.", item.Filename)
			t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "cancelled", Message: fmt.Sprintf("Transcoding cancelled for %s", item.Filename)})
		} else {
			log.Printf("[failed]: Transcoding for %s failed.", item.Filename)
			t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Transcoding failed for %s", item.Filename)})
		}
		return
	}

	log.Printf("[finished]: %s file successfully processed", item.Filename)

	// Define the path for the output zip file.
	zipFilePath := outputFolder + ".zip"
	log.Printf("[%s] Zipping output folder %s to %s", t.taskID, outputFolder, zipFilePath)
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{
		Type:    "progress",
		Message: "Archiving transcoded files...",
	})

	err = utils.ZipOutputFolder(outputFolder, zipFilePath)
	if err != nil {
		log.Printf("[%s] Failed to zip output folder: %v", t.taskID, err)
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{
			Type:    "failed",
			Message: fmt.Sprintf("Failed to archive files: %v", err),
		})
		return
	}

	log.Printf("[%s] Successfully created zip archive: %s", t.taskID, zipFilePath)

	// Output folder cleanup
	if err := os.RemoveAll(outputFolder); err != nil {
		log.Printf("[%s] Warning: Failed to clean up output folder %s: %v", t.taskID, outputFolder, err)
	}

	// Send a final "completed" status update.
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{
		Type:    "completed",
		Message: "Transcoding and archiving complete. Your download is ready.",
	})

}

// transcodeResolutions transcodes the source video into multiple resolutions.
func (t *Transcoder) transcodeResolutions(ctx context.Context, outputFolder string) bool {
	var wg sync.WaitGroup
	var mu sync.Mutex
	playlistChan := make(chan types.TranscoderPlaylist, len(t.resolutions))
	errorOccurred := false // Flag to track if any transcoding failed

	for _, resolution := range t.resolutions {
		wg.Add(1)

		go func(res types.Resolutions) {
			defer wg.Done()

			playlist, err := t.transcode(ctx, res, outputFolder)
			if err != nil {
				// Check if the error was due to the context being canceled.
				if errors.Is(err, context.Canceled) {
					log.Printf("[cancelled]: Transcoding %s was cancelled.", res.String())
					// Don't treat cancellation as a regular error that sets the errorOccurred flag.
					return
				}

				log.Printf("[skipping]: %s for %s; %v", res.String(), t.source.Filename, err)
				mu.Lock()
				errorOccurred = true
				mu.Unlock()
				t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Skipping %s: %v", res.String(), err)})
				return
			}

			if playlist != nil {
				playlistChan <- *playlist
			}
		}(resolution)
	}

	wg.Wait()
	close(playlistChan)

	// After waiting, check if the context was cancelled. If so, the entire operation
	// is considered unsuccessful, and we should not proceed.
	if ctx.Err() == context.Canceled {
		return false
	}

	if errorOccurred {
		return false // If any transcoding failed, consider the whole process failed
	}

	resolutionPlaylists := []types.TranscoderPlaylist{}
	for playlist := range playlistChan {
		resolutionPlaylists = append(resolutionPlaylists, playlist)
	}

	// If no playlists were generated,
	// don't build the main playlist.
	if len(resolutionPlaylists) == 0 {
		return false
	}

	return t.buildMainPlaylist(resolutionPlaylists, outputFolder)
}

// transcode transcodes the video to a specific resolution and generates an HLS playlist.
func (t *Transcoder) transcode(
	ctx context.Context,
	resolution types.Resolutions,
	outputFolder string,
) (*types.TranscoderPlaylist, error) {
	preset, ok := types.RESOLUTIONS[resolution]
	if !ok {
		return nil, fmt.Errorf("[argument error]: Invalid resolution provided: %s", resolution.String())
	}

	filenameLessExt := utils.GetFilenameLessExt(t.source.Filename)
	resolutionOutput := filepath.Join(outputFolder, resolution.String())
	outputFilenameLessExt := fmt.Sprintf("%s_%s", filenameLessExt, resolution.String())
	outputPlaylist := filepath.Join(resolutionOutput, fmt.Sprintf("%sp.m3u8", outputFilenameLessExt))
	outputSegment := filepath.Join(resolutionOutput, fmt.Sprintf("%s_%%03d.ts", outputFilenameLessExt))
	outputPlaylistFromMain := filepath.Join(resolution.String(), fmt.Sprintf("%sp.m3u8", outputFilenameLessExt))

	if err := os.MkdirAll(resolutionOutput, 0755); err != nil {
		return nil, fmt.Errorf("failed to create resolution output folder %s: %w", resolutionOutput, err)
	}

	args := []string{
		"-i", t.source.File,
		"-preset", "fast",
		"-crf", "28",
		"-sc_threshold", "0",
		"-g", "48",
		"-keyint_min", "48",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", outputSegment,
		"-vf", fmt.Sprintf("scale=-2:%d", preset.Height),
		"-b:v", fmt.Sprintf("%dk", preset.Bitrate),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:a", "128k",
		outputPlaylist,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	log.Printf("[started]: transcoding %s for %s", resolution.String(), t.source.Filename)
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "started", Message: fmt.Sprintf("Started %s transcoding", resolution.String()), Data: types.TaskData{
		Resolution: resolution.String(),
		Timestamp:  0,
		Frame:      "",
		Progress:   0.0,
	}})

	// Capture stderr to a pipe for progress logging
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	scannerStderr := bufio.NewScanner(stderrPipe)

	// Use a buffer to capture all stderr for logging in case of command failure
	var totalStderr bytes.Buffer

	var wgOutput sync.WaitGroup
	wgOutput.Add(1)

	go func() {
		defer wgOutput.Done()
		for scannerStderr.Scan() {
			line := scannerStderr.Text()
			fmt.Fprintf(&totalStderr, "%s\n", line) // Capture all stderr

			// Basic progress parsing (can be more robust if needed)
			if strings.Contains(line, "frame=") && strings.Contains(line, "time=") {
				frame, timemark, speed := utils.ParseFFmpegProgress(line)
				if timemark != "" {
					timemarkParts := strings.Split(timemark, ":")
					if len(timemarkParts) < 3 {
						log.Printf("[error]: unexpected timemark format: %s", timemark)
						continue
					}

					hours, _ := strconv.ParseFloat(timemarkParts[0], 64)
					minutes, _ := strconv.ParseFloat(timemarkParts[1], 64)
					seconds, _ := strconv.ParseFloat(timemarkParts[2], 64)
					currentSeconds := hours*3600 + minutes*60 + seconds

					progressPercent := 0.0
					progressPercent = min((currentSeconds/t.inputDuration)*100, 100)

					msg := fmt.Sprintf("Transcoding %s: frame %s, time %s, speed %sx",
						resolution.String(), frame, timemark, speed)

					log.Printf("[progress]: %s (%.2f%%)", msg, progressPercent)
					t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{
						Type:    "progress",
						Message: msg,
						Data: types.TaskData{
							Resolution: resolution.String(),
							Frame:      frame,
							Timestamp:  int64(currentSeconds),
							Progress:   progressPercent,
						},
					})
				}
			}
		}
	}()

	err = cmd.Start()
	if err != nil {
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Failed to start %s command: %v", resolution.String(), err)})
		return nil, fmt.Errorf("failed to start ffmpeg command: %w", err)
	}

	wgOutput.Wait() // Wait for stdout and stderr scanners to finish reading
	err = cmd.Wait()
	if err != nil {
		// Check if the error is because the context was cancelled.
		if ctx.Err() == context.Canceled {
			errMsg := fmt.Sprintf("transcoding %s cancelled for %s", resolution.String(), t.source.Filename)
			log.Println(errMsg)
			// Return a specific error or nil, signaling cancellation.
			return nil, ctx.Err()
		}

		// Now you can safely use totalStderr.String() to get all captured stderr
		errMsg := fmt.Sprintf("[ffmpeg error]: transcoding %s failed for %s: %v, stderr: %s",
			resolution.String(), t.source.Filename, err, totalStderr.String())
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: errMsg})
		return nil, fmt.Errorf("%s", errMsg)
	}

	log.Printf("[completed]: transcoding %s for %s; output %s", resolution.String(), t.source.Filename, outputPlaylist)
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "completed", Message: fmt.Sprintf("Completed %s output generation.", resolution.String()), Data: types.TaskData{
		Resolution: resolution.String(),
		Timestamp:  0,
		Frame:      "",
		Progress:   100.0, // Mark as complete
	}})

	detectedRes, err := utils.DetectPlaylistResolution(outputPlaylist)
	if err != nil {
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Failed to detect playlist resolution for %s: %v", resolution.String(), err)})
		return nil, fmt.Errorf("failed to detect playlist resolution for %s: %w", outputPlaylist, err)
	}

	return &types.TranscoderPlaylist{
		Resolution:           detectedRes,
		PlaylistFilename:     filepath.Base(outputPlaylist),
		PlaylistPathFromMain: outputPlaylistFromMain,
		PlaylistPath:         outputPlaylist,
	}, nil
}

// buildMainPlaylist creates the master M3U8 playlist.
func (t *Transcoder) buildMainPlaylist(playlists []types.TranscoderPlaylist, outputFolder string) bool {
	if len(playlists) == 0 {
		log.Printf("[skipping]: main playlist for %s; no resolution playlists found", outputFolder)
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: "Skipping main playlist: no resolutions transcoded."})
		return false
	}

	mainPlaylistPath := filepath.Join(outputFolder, "main.m3u8")
	log.Printf("[started]: generating main playlist %s", mainPlaylistPath)
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "progress", Message: "Generating master playlist..."})

	mainContent := []string{"#EXTM3U", "#EXT-X-VERSION:3"}

	for _, playlist := range playlists {
		log.Printf("[playlist]: %dp for %s", playlist.Resolution.Height, playlist.PlaylistPathFromMain)
		mainContent = append(mainContent,
			fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d",
				playlist.Resolution.Bitrate*1000, playlist.Resolution.Width, playlist.Resolution.Height))
		mainContent = append(mainContent, playlist.PlaylistPathFromMain)
	}

	finalContent := strings.Join(mainContent, "\n")

	if err := os.WriteFile(mainPlaylistPath, []byte(finalContent), 0644); err != nil {
		log.Printf("[error]: failed to write main playlist %s: %v", mainPlaylistPath, err)
		t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "failed", Message: fmt.Sprintf("Failed to write main playlist: %v", err)})
		return false
	}

	log.Printf("[completed]: generating main playlist %s", mainPlaylistPath)
	t.statusMgr.SendUpdate(t.taskID, types.StatusUpdate{Type: "completed", Message: "Master playlist generated."})
	return true
}
