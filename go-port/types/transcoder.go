package types

import (
	"fmt"
)

// source file information.
type TranscoderSource struct {
	File     string
	Filename string
	Extname  string
}

// information about a generated HLS playlist for a specific resolution.
type TranscoderPlaylist struct {
	Resolution           ResolutionPreset
	PlaylistFilename     string
	PlaylistPathFromMain string
	PlaylistPath         string
}

// video width, height and bitrate.
type ResolutionPreset struct {
	Height  int
	Width   int
	Bitrate int
}

// Resolutions enum type
type Resolutions int

// Enum values for Resolutions
const (
	P2160 Resolutions = 2160
	P1440 Resolutions = 1440
	P1080 Resolutions = 1080
	P720  Resolutions = 720
	P480  Resolutions = 480
	P360  Resolutions = 360
)

// map from Resolutions enum to ResolutionPreset struct
var RESOLUTIONS = map[Resolutions]ResolutionPreset{
	P2160: {Height: 2160, Width: 3840, Bitrate: 14000}, // 4K resolution
	P1440: {Height: 1440, Width: 2560, Bitrate: 9000},  // 2K resolution
	P1080: {Height: 1080, Width: 1920, Bitrate: 6500},  // 1080p resolution
	P720:  {Height: 720, Width: 1280, Bitrate: 4000},   // 720p resolution
	P480:  {Height: 480, Width: 854, Bitrate: 2000},    // 480p resolution
	P360:  {Height: 360, Width: 640, Bitrate: 1000},    // 360p resolution
}

// returns the string representation of Resolutions.
func (r Resolutions) String() string {
	return fmt.Sprintf("%dP", int(r))
}

// FFProbeStream represents a single stream in the FFProbe output.
type FFProbeStream struct {
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// FFProbeFormat represents the format information in the FFProbe output.
type FFProbeFormat struct {
	Filename   string `json:"filename"`
	NbStreams  int    `json:"nb_streams"`
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
}

// FFProbeOutput represents the JSON output structure from ffprobe.
type FFProbeOutput struct {
	Streams []FFProbeStream `json:"streams"`
	Format  FFProbeFormat   `json:"format"`
}
