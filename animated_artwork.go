package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func saveAnimatedArtwork(folderPath string, squareVideo string, tallVideo string) {
	if !Config.SaveAnimatedArtwork || squareVideo == "" {
		return
	}
	fmt.Println("Found Animation Artwork.")
	saveAnimatedArtworkVariant(folderPath, "square", squareVideo)

	if Config.EmbyAnimatedArtwork {
		outPath := filepath.Join(folderPath, "folder.jpg")
		if err := runCommandWithAtomicOutput(outPath, func(tempPath string) *exec.Cmd {
			return exec.Command(Config.FFmpegPath, "-i", filepath.Join(folderPath, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", tempPath)
		}); err != nil {
			fmt.Printf("animated artwork square to gif err: %v\n", err)
		}
	}

	if tallVideo != "" {
		saveAnimatedArtworkVariant(folderPath, "tall", tallVideo)
	}
}

func saveAnimatedArtworkVariant(folderPath string, label string, video string) {
	motionVideoURL, err := extractVideo(video)
	if err != nil {
		fmt.Printf("no motion video %s.\n %v\n", label, err)
		return
	}

	outPath := filepath.Join(folderPath, fmt.Sprintf("%s_animated_artwork.mp4", label))
	exists, err := fileExists(outPath)
	if err != nil {
		fmt.Printf("Failed to check if animated artwork %s exists.\n", label)
	}
	if exists {
		fmt.Printf("Animated artwork %s already exists locally.\n", label)
		return
	}

	displayLabel := animatedArtworkDisplayLabel(label)
	fmt.Printf("Animation Artwork %s Downloading...\n", displayLabel)
	if err := runCommandWithAtomicOutput(outPath, func(tempPath string) *exec.Cmd {
		return exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionVideoURL, "-c", "copy", tempPath)
	}); err != nil {
		fmt.Printf("animated artwork %s dl err: %v\n", label, err)
		return
	}
	fmt.Printf("Animation Artwork %s Downloaded\n", displayLabel)
}

func animatedArtworkDisplayLabel(label string) string {
	switch label {
	case "square":
		return "Square"
	case "tall":
		return "Tall"
	default:
		return label
	}
}
