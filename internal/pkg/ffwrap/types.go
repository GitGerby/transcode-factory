package ffwrap

import libCodec "github.com/gitgerby/transcode-factory/internal/pkg/ffwrap/codec"

type MediaMetadata struct {
	Duration string
	Codec    string
	Width    int
	Height   int
}

type FfprobeOutput struct {
	Streams []FfprobeStreams
	Format  FfprobeFormat
}

type FfprobeStreams struct {
	Codec      string `json:"codec_name"`
	Codec_type string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type FfprobeFormat struct {
	Duration string `json:"duration"`
}

type TranscodeRequest struct {
	Source         string   `json:"source"`
	Destination    string   `json:"destination"`
	Srt_files      []string `json:"srt_files"`
	Crf            int      `json:"crf"`
	Autocrop       bool     `json:"autocrop"`
	Video_filters  string   `json:"video_filters"`
	Audio_filters  string   `json:"audio_filters"`
	Codec          string   `json:"codec"`
	LogDestination string
}

type ColorInfoWrapper struct {
	Frames []libCodec.ColorInfo `json:"frames"`
}
