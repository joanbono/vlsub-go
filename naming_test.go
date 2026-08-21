package main

import "testing"

func TestParseName(t *testing.T) {
	tests := []struct {
		path            string
		query           string
		season, episode int
	}{
		{"The Big Bang Theory 01x03.mkv", "The Big Bang Theory", 1, 3},
		{"/media/Series/The Big Bang Theory/S01/The Big Bang Theory 01x17.mkv", "The Big Bang Theory", 1, 17},
		{"Breaking.Bad.S05E14.1080p.BluRay.x264.mkv", "Breaking Bad", 5, 14},
		{"Some.Show.s2e7.HDTV.mp4", "Some Show", 2, 7},
		{"Inception.2010.1080p.BluRay.x264.mkv", "Inception 2010", 0, 0},
		{"Amelie.mkv", "Amelie", 0, 0},
		// A resolution must not be mistaken for an episode marker.
		{"Movie.Name.1920x1080.WEBRip.mkv", "Movie Name 1920x1080", 0, 0},
	}

	for _, tc := range tests {
		q, s, e := ParseName(tc.path)
		if q != tc.query || s != tc.season || e != tc.episode {
			t.Errorf("ParseName(%q) = (%q, %d, %d), want (%q, %d, %d)",
				tc.path, q, s, e, tc.query, tc.season, tc.episode)
		}
	}
}
