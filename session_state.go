package main

import (
	"strings"
	"sync"

	"amdl/utils/structs"
)

type sessionState struct {
	Counter             structs.Counter
	OKDict              map[string][]int
	AddedTracks         []AddedTrack
	SelectedAudioFormat *AudioFormat
	SourceURL           string
	SongMode            bool
	Log                 strings.Builder
	LogMu               sync.Mutex
}

func newSessionState(sourceURL string) *sessionState {
	return &sessionState{
		OKDict:    make(map[string][]int),
		SourceURL: sourceURL,
		SongMode:  dl_song,
	}
}

func (s *sessionState) merge(other *sessionState) {
	if s == nil || other == nil {
		return
	}
	s.Counter.Total += other.Counter.Total
	s.Counter.Success += other.Counter.Success
	s.Counter.Unavailable += other.Counter.Unavailable
	s.Counter.NotSong += other.Counter.NotSong
	s.Counter.Error += other.Counter.Error
	s.AddedTracks = append(s.AddedTracks, other.AddedTracks...)
	for key, values := range other.OKDict {
		s.OKDict[key] = append(s.OKDict[key], values...)
	}
}

func (s *sessionState) isAtmosMode() bool {
	if s != nil && s.SelectedAudioFormat != nil {
		return s.SelectedAudioFormat.Codecs == "ec-3" || s.SelectedAudioFormat.Codecs == "ac-3"
	}
	return dl_atmos
}

func (s *sessionState) isAACMode() bool {
	if s != nil && s.SelectedAudioFormat != nil {
		return s.SelectedAudioFormat.Codecs == "mp4a.40.2"
	}
	return dl_aac
}

func (s *sessionState) aacType() string {
	if s != nil && s.SelectedAudioFormat != nil && s.SelectedAudioFormat.Codecs == "mp4a.40.2" {
		return audioFormatAACType(*s.SelectedAudioFormat)
	}
	return Config.AacType
}

func (s *sessionState) alacMax() int {
	if s != nil && s.SelectedAudioFormat != nil && s.SelectedAudioFormat.Codecs == "alac" {
		if sampleRate := audioFormatSampleRate(*s.SelectedAudioFormat); sampleRate > 0 {
			return sampleRate
		}
	}
	if Config.AlacMax == 0 {
		return 192000
	}
	return Config.AlacMax
}

func (s *sessionState) atmosMax() int {
	if s != nil && s.SelectedAudioFormat != nil && (s.SelectedAudioFormat.Codecs == "ec-3" || s.SelectedAudioFormat.Codecs == "ac-3") {
		if bitrate := audioFormatBitrate(*s.SelectedAudioFormat); bitrate > 0 {
			return bitrate
		}
	}
	return Config.AtmosMax
}

func (s *sessionState) currentCodec() string {
	if s.isAtmosMode() {
		return "ATMOS"
	}
	if s.isAACMode() {
		return "AAC"
	}
	return "ALAC"
}
