package services

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PratikDev/transcoder/services/utils"
	"github.com/PratikDev/transcoder/types"
)

// Transcoder handles the video transcoding process.
type Transcoder struct {
	source      types.TranscoderSource
	resolutions []types.Resolutions
	output      string
}

// NewTranscoder creates a new Transcoder instance.
func NewTranscoder(source types.TranscoderSource, output string) *Transcoder {
	vidResolution, err := utils.DetectVideoResolution(source.File)
	if err != nil {
		log.Printf("[error]: failed to detect video resolution for %s: %v", source.File, err)
		return nil
	}

	resolutions := utils.GetAvailableResolutions(vidResolution)
	if len(resolutions) == 0 {
		log.Printf("[error]: no valid resolutions found for %s", source.File)
		return nil
	}

	return &Transcoder{
		source:      source,
		resolutions: resolutions,
		output:      output,
	}
}

// Process starts the transcoding process for the source video.
func (t *Transcoder) Process() {
	item := t.source

	// Get the output folder name from the file and output dir name
	outputFolder := utils.GetOutputFolderName(t.output, item.Filename)

	// Make the output folder
	if err := os.MkdirAll(outputFolder, 0755); err != nil {
		log.Printf("[error]: failed to create output folder %s: %v", outputFolder, err)
		return
	}

	success := t.transcodeResolutions(outputFolder)
	if !success {
		log.Printf("[failed]: transcoding for %s failed", item.Filename)
		return
	}

	log.Printf("[finished]: %s file successfully processed", item.Filename)
}

// transcodeResolutions transcodes the source video into multiple resolutions.
func (t *Transcoder) transcodeResolutions(outputFolder string) bool {
	var wg sync.WaitGroup
	playlistChan := make(chan types.TranscoderPlaylist, len(t.resolutions))
	errorOccurred := false // Flag to track if any transcoding failed

	for _, resolution := range t.resolutions {
		wg.Add(1)
		go func(res types.Resolutions) {
			defer wg.Done()

			playlist, err := t.transcode(res, outputFolder)
			if err != nil {
				log.Printf("[skipping]: %s for %s; %v", res.String(), t.source.Filename, err)
				errorOccurred = true // Mark that an error occurred
				return
			}
			if playlist != nil {
				playlistChan <- *playlist
			}
		}(resolution)
	}

	wg.Wait()
	close(playlistChan)

	if errorOccurred {
		return false // If any transcoding failed, consider the whole process failed
	}

	resolutionPlaylists := []types.TranscoderPlaylist{}
	for playlist := range playlistChan {
		resolutionPlaylists = append(resolutionPlaylists, playlist)
	}

	return t.buildMainPlaylist(resolutionPlaylists, outputFolder)
}

// transcode transcodes the video to a specific resolution and generates an HLS playlist.
func (t *Transcoder) transcode(
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

	cmd := exec.Command("ffmpeg", args...)

	log.Printf("[started]: transcoding %s for %s", resolution.String(), t.source.Filename)

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
				parts := strings.Fields(line)
				frame, timemark, speed := utils.ParseFFmpegProgress(parts)
				if timemark != "" {
					log.Printf("[progress]: @ frame %s; timemark %s; speed %s", frame, timemark, speed)
				}
			}
		}
	}()

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg command: %w", err)
	}

	wgOutput.Wait() // Wait for stdout and stderr scanners to finish reading
	err = cmd.Wait()
	if err != nil {
		// Now you can safely use totalStderr.String() to get all captured stderr
		return nil, fmt.Errorf("[ffmpeg error]: transcoding failed for %s (%s): %w, stderr: %s",
			t.source.Filename, resolution.String(), err, totalStderr.String())
	}

	log.Printf("[completed]: transcoding %s for %s; output %s", resolution.String(), t.source.Filename, outputPlaylist)

	detectedRes, err := utils.DetectPlaylistResolution(outputPlaylist)
	if err != nil {
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
		return false
	}

	mainPlaylistPath := filepath.Join(outputFolder, "main.m3u8")
	log.Printf("[started]: generating main playlist %s", mainPlaylistPath)

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
		return false
	}

	log.Printf("[completed]: generating main playlist %s", mainPlaylistPath)
	return true
}
