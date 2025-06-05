package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/PratikDev/transcoder/types"
)

// Pre-compile regex patterns for efficiency.
// These will be used by ParseFFmpegProgress.
var (
	// frameRegex matches "frame= N" (or "frame=N") and captures N (the frame number).
	frameRegex = regexp.MustCompile(`frame=\s*(\d+)`)

	// timeRegex matches "time=HH:MM:SS.ms" and captures the timestamp string.
	timeRegex = regexp.MustCompile(`time=\s*([\d:.]+)`)

	// speedRegex matches "speed= N.Mx" and captures N.M (the speed multiplier).
	speedRegex = regexp.MustCompile(`speed=\s*([\d.]+)x`)
)

const (
	UPLOAD_DIR = "./uploads" // Directory to temporarily store uploaded videos
	OUTPUT_DIR = "./output"  // Directory for transcoded output
)

// GetFilenameLessExt returns the filename without its extension.
func GetFilenameLessExt(fileName string) string {
	return strings.TrimSuffix(fileName, strings.ToLower(filepath.Ext(fileName)))
}

// ParseFFmpegProgress parses a single progress line string from FFmpeg's stderr
// using regular expressions to extract frame, timemark, and speed.
func ParseFFmpegProgress(line string) (frame, timemark, speed string) {
	frameMatch := frameRegex.FindStringSubmatch(line)
	if len(frameMatch) > 1 {
		frame = frameMatch[1]
	}

	timeMatch := timeRegex.FindStringSubmatch(line)
	if len(timeMatch) > 1 {
		timemark = timeMatch[1]
	}

	speedMatch := speedRegex.FindStringSubmatch(line)
	if len(speedMatch) > 1 {
		speed = speedMatch[1]
	}

	return
}

// DetectResolution uses ffprobe to detect the resolution of a playlist file.
func DetectPlaylistResolution(playlistPath string) (types.ResolutionPreset, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,codec_type",
		"-of", "json",
		playlistPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return types.ResolutionPreset{}, fmt.Errorf("ffprobe command failed on playlist %s: %w, stderr: %s", playlistPath, err, stderr.String())
	}

	var result types.FFProbeOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return types.ResolutionPreset{}, fmt.Errorf("failed to parse ffprobe output for playlist %s: %w", playlistPath, err)
	}

	var width, height int
	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			width = stream.Width
			height = stream.Height
			break
		}
	}

	if width == 0 || height == 0 {
		return types.ResolutionPreset{}, fmt.Errorf("could not detect playlist resolution for %s", playlistPath)
	}

	return types.ResolutionPreset{Width: width, Height: height}, nil
}

// DetectVideoResolution uses ffprobe to detect the resolution of a video file.
func DetectVideoResolution(path string) (types.Resolutions, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,codec_type",
		"-of", "json",
		path,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ffprobe command failed: %w, stderr: %s", err, stderr.String())
	}

	var result types.FFProbeOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var width, height int
	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			width = stream.Width
			height = stream.Height
			break
		}
	}

	if width == 0 || height == 0 {
		return 0, fmt.Errorf("could not detect video resolution for %s", path)
	}

	// Find the closest matching resolution in our predefined map
	for resEnum, preset := range types.RESOLUTIONS {
		if preset.Width == width && preset.Height == height {
			return resEnum, nil
		}
	}

	// Default to P720 if no exact match is found
	log.Printf("No exact resolution match found for %dx%d. Defaulting to P720.", width, height)
	return types.P720, nil
}

// DetectInputDuration uses ffprobe to get the duration of the input video.
func DetectInputDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to detect input duration: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse input duration '%s': %w", durationStr, err)
	}
	return duration, nil
}

// GetTargetResolutions returns a list of available resolutions that are less than or equal to the provided resolution.
// It filters out resolutions that have a width or height of 0.
func GetTargetResolutions(resolution types.Resolutions) []types.Resolutions {
	availableResolutions := []types.Resolutions{}
	for res, preset := range types.RESOLUTIONS {
		if res <= resolution && preset.Width > 0 && preset.Height > 0 {
			availableResolutions = append(availableResolutions, res)
		}
	}
	return availableResolutions
}

// RemoveOutputDirectory removes the output directory for a given task ID.
func RemoveOutputDirectory(taskID string) error {
	outputDir := filepath.Join(OUTPUT_DIR, taskID)
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to remove output directory %s: %w", outputDir, err)
	}

	return nil
}
