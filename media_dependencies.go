package main

import (
	ampapi "amdl/internal/amp-api"
	playreadyrip "amdl/internal/playready-rip"
	widevinerip "amdl/internal/widevine-rip"
	"amdl/internal/widevine-rip/runv5"
	"amdl/internal/wrapper"
	"context"
)

var (
	getSongResp       = ampapi.GetSongResp
	getMusicVideoResp = ampapi.GetMusicVideoResp
	getMVWebplayback  = wrapper.GetWebplayback
	extractMVData     = widevinerip.DownloadAndDecryptStream
	extractMVVideoURL = extractVideoVariantWithSelection
	extractMVAudioURL = extractMvAudio
	fetchMVStream     = func(ctx context.Context, adamID, url, server string, playReady bool) (widevinerip.EncryptedStream, error) {
		if playReady {
			return playreadyrip.FetchStream(ctx, adamID, url, server)
		}
		return runv5.FetchStream(ctx, adamID, url, server)
	}
)
