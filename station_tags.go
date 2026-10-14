package main

import (
	mp4tag "github.com/itouakirai/go-mp4tag"
	"os"
)

func writeStationTags(path, name, coverPath string) error {
	tags := &mp4tag.MP4Tags{Title: name, Artist: "Apple Music Station", Album: name, AlbumArtist: "Apple Music Station", TrackNumber: 1, TrackTotal: 1, DiscNumber: 1, DiscTotal: 1, Custom: map[string]string{"PERFORMER": "Apple Music Station"}}
	if Config.EmbedCover && coverPath != "" {
		cover, err := os.ReadFile(coverPath)
		if err != nil {
			return err
		}
		tags.Pictures = []*mp4tag.MP4Picture{{Format: mp4tag.ImageTypeAuto, Data: cover}}
	}
	file, err := mp4tag.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Write(tags, []string{})
}
