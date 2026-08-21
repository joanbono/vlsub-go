package main

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Season/episode markers, most explicit form first so that "S01E03" wins
	// over a bare "01x03" elsewhere in the name.
	episodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bs(\d{1,2})[ ._-]*e(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`),
	}

	// Everything from the first release tag onward is noise for a title search.
	releaseJunk = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|576p|480p|x264|x265|h ?264|h ?265|hevc|xvid|divx|bluray|blu-ray|bdrip|brrip|dvdrip|dvdscr|webrip|web-dl|webdl|hdtv|aac\d?|ac3|eac3|dts(-hd)?|truehd|atmos|10bit|8bit|hdr\d*|dolby|remux|proper|repack|internal|extended|unrated|limited|multi|dual)\b.*$`)

	separators = regexp.MustCompile(`[._]+`)
	whitespace = regexp.MustCompile(`\s+`)
)

// ParseName derives a title query and, for episodes, season and episode
// numbers from a media filename. Season and episode are zero when the name
// does not look like an episode.
func ParseName(path string) (query string, season, episode int) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	s := separators.ReplaceAllString(base, " ")

	for _, re := range episodePatterns {
		if m := re.FindStringSubmatchIndex(s); m != nil {
			season, _ = strconv.Atoi(s[m[2]:m[3]])
			episode, _ = strconv.Atoi(s[m[4]:m[5]])
			// The title is whatever precedes the marker.
			s = s[:m[0]]
			break
		}
	}

	s = releaseJunk.ReplaceAllString(s, "")
	s = whitespace.ReplaceAllString(s, " ")
	return strings.Trim(s, " -_"), season, episode
}
