package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Query describes one subtitle search.
type Query struct {
	Lang    Language
	Hash    string
	Size    int64
	Text    string
	Season  int
	Episode int
}

// Result is one search hit, normalised across backends.
type Result struct {
	Name            string
	Lang            string
	Format          string // srt, ass, sub …
	Downloads       int
	FPS             float64
	HearingImpaired bool
	HashMatch       bool // timed against this exact release
	Trusted         bool

	// Backend handle for fetching this result. Exactly one is set.
	link   string // opensubtitles.org: direct (gzipped) URL
	fileID int    // opensubtitles.com: file id for the download endpoint
}

// Provider is a subtitle source.
type Provider interface {
	Name() string
	Search(ctx context.Context, q Query) ([]Result, error)
	Fetch(ctx context.Context, r Result) (data []byte, filename string, note string, err error)
}

// Ext returns the file extension to save this result under.
func (r Result) Ext() string {
	f := strings.ToLower(strings.TrimSpace(r.Format))
	if f == "" {
		return "srt"
	}
	return f
}

// Label renders a result as one human-readable line.
func (r Result) Label() string {
	var tags []string
	if r.HashMatch {
		tags = append(tags, "hash-match")
	}
	if r.Trusted {
		tags = append(tags, "trusted")
	}
	if r.HearingImpaired {
		tags = append(tags, "sdh")
	}
	if r.FPS > 0 {
		tags = append(tags, strconv.FormatFloat(r.FPS, 'f', -1, 64)+"fps")
	}
	tags = append(tags, r.Ext(), strconv.Itoa(r.Downloads)+" dl")

	name := r.Name
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf("%-58.58s [%s] %s", name, r.Lang, strings.Join(tags, ", "))
}

// rank orders results best-first. A hash match wins outright: it is the only
// signal that the subtitle was timed against this exact release. After that we
// prefer SRT over other formats, honour the caller's SDH preference, then
// trusted uploaders, then popularity.
func rank(in []Result, preferSDH bool) []Result {
	out := make([]Result, len(in))
	copy(out, in)

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.HashMatch != b.HashMatch {
			return a.HashMatch
		}
		if aSRT, bSRT := a.Ext() == "srt", b.Ext() == "srt"; aSRT != bSRT {
			return aSRT
		}
		if a.HearingImpaired != b.HearingImpaired {
			return a.HearingImpaired == preferSDH
		}
		if a.Trusted != b.Trusted {
			return a.Trusted
		}
		return a.Downloads > b.Downloads
	})
	return out
}
