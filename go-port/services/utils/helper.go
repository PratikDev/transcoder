package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PratikDev/transcoder/types"
)

// GetFilenameLessExt returns the filename without its extension.
func GetFilenameLessExt(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

// GetOutputFolderName constructs the output folder path based on the output directory and the filename without extension.
func GetOutputFolderName(output string, fileName string) string {
	return filepath.Join(output, GetFilenameLessExt(fileName))
}

// ParseFFmpegProgress parses the progress output from FFmpeg and returns the frame, time mark, and speed.
func ParseFFmpegProgress(parts []string) (frame, timemark, speed string) {
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "frame="):
			frame = strings.TrimPrefix(part, "frame=")
		case strings.HasPrefix(part, "time="):
			timemark = strings.TrimPrefix(part, "time=")
		case strings.HasPrefix(part, "speed="):
			speed = strings.TrimPrefix(part, "speed=")
		}
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
