package main

import "strings"

func filterArtistItems(items []ArtistSelectItem) []ArtistSelectItem {
	if !Config.ArtistFilterSinglesEPs &&
		!Config.ArtistFilterLive &&
		!Config.ArtistFilterCompilations &&
		!Config.ArtistFilterDuplicates {
		return items
	}
	out := make([]ArtistSelectItem, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		name := strings.ToLower(item.Name)
		if Config.ArtistFilterSinglesEPs && item.Type == "ALBUM" && (strings.Contains(name, " - single") || strings.Contains(name, " - ep") || strings.HasSuffix(name, " single") || strings.HasSuffix(name, " ep")) {
			continue
		}
		if Config.ArtistFilterLive && strings.Contains(name, "live") {
			continue
		}
		if Config.ArtistFilterCompilations && strings.Contains(name, "compilation") {
			continue
		}
		if Config.ArtistFilterDuplicates {
			key := normalizedArtistItemName(item.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, item)
	}
	return out
}

func normalizedArtistItemName(name string) string {
	name = strings.ToLower(name)
	for _, marker := range []string{" (deluxe", " (remaster", " (expanded", " (explicit", " (clean"} {
		if idx := strings.Index(name, marker); idx >= 0 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}
