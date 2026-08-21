package main

import (
	"regexp"
	"strconv"
	"strings"
)

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

type cue struct {
	timing string
	lines  []string
}

// MergeSplitCues repairs a defect common in OpenSubtitles uploads: a cue that
// should hold two lines is stored as two consecutive cues sharing one timing.
// Players anchor subtitles to the bottom and stack simultaneous cues upward,
// so the second line renders *above* the first and the text reads backwards.
//
// It returns the repaired SRT and the number of merges performed. Input that
// has no duplicate timings is returned unchanged.
func MergeSplitCues(srt string) (string, int) {
	text := strings.ReplaceAll(srt, "\r\n", "\n")
	text = strings.TrimPrefix(text, "\ufeff")

	var cues []cue
	for _, block := range blankLine.Split(strings.TrimSpace(text), -1) {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		// A well-formed block is: index, timing, then one or more text lines.
		if len(lines) < 3 || !strings.Contains(lines[1], "-->") {
			continue
		}
		var body []string
		for _, l := range lines[2:] {
			if strings.TrimSpace(l) != "" {
				body = append(body, l)
			}
		}
		cues = append(cues, cue{timing: strings.TrimSpace(lines[1]), lines: body})
	}

	merged := make([]cue, 0, len(cues))
	for _, c := range cues {
		if n := len(merged); n > 0 && merged[n-1].timing == c.timing {
			merged[n-1].lines = append(merged[n-1].lines, c.lines...)
			continue
		}
		merged = append(merged, c)
	}

	count := len(cues) - len(merged)
	if count == 0 {
		return srt, 0
	}

	var b strings.Builder
	for i, c := range merged {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteByte('\n')
		b.WriteString(c.timing)
		b.WriteByte('\n')
		b.WriteString(strings.Join(c.lines, "\n"))
		b.WriteString("\n\n")
	}
	return b.String(), count
}
