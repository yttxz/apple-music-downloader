package main

import (
	"apple-music-downloader/utils/ampapi"
	"apple-music-downloader/utils/runv3"
)

var (
	getSongResp       = ampapi.GetSongResp
	getMusicVideoResp = ampapi.GetMusicVideoResp
	getMVWebplayback  = runv3.GetWebplayback
	runMVDownload     = runv3.Run
	extractMVData     = runv3.ExtMvData
	extractMVVideoURL = extractVideoWithSelection
	extractMVAudioURL = extractMvAudio
)
