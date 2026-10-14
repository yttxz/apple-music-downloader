package app

import (
	ampapi "amdl/internal/amp-api"
	"amdl/internal/model"
)

// WriteMP4Tags exposes shared tag processing to the fork's workflow runner.
func (r *Runner) WriteMP4Tags(track *model.Track, lyrics string) error {
	return r.writeMP4Tags(track, lyrics)
}
func (r *Runner) WriteMVTags(path string, info *ampapi.MusicVideoResp, track *model.Track, cover string) error {
	return r.writeMVMP4Tags(path, info, track, cover)
}
func (r *Runner) ExtractMVAudio(url string) (string, error) { return r.extractMvAudio(url) }

// StreamAllowedCPC reports the content protection advertised for a video variant.
func StreamAllowedCPC(master, variant string) string { return streamAllowedCPC(master, variant) }
