package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

const (
	maxBody = 16 << 20

	// The .org XML-RPC API keys its rate limits off the User-Agent and rejects
	// unregistered ones, so we send the same string vlsub does.
	orgUserAgent = "VLSub 0.10.2"
)

// version identifies the build. Release binaries stamp it at link time with
//
//	-ldflags "-X main.version=v1.2.3"
//
// and anything installed with "go install" falls back to the module version
// the toolchain records in the binary.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			version = v
		}
	}
}

// comUserAgent is a function rather than a constant so that it observes the
// stamped version.
func comUserAgent() string { return "vlsub-go v" + version }

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "vlsub-go: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("vlsub-go", flag.ContinueOnError)
	fs.SetOutput(stdout)

	var (
		file     = fs.String("file", "", "path to the movie or episode file (required)")
		lang     = fs.String("lang", "eng", "subtitle language: eng, spa, cat, en, pt-BR …")
		out      = fs.String("out", "", "output path (default: alongside the movie as NAME.LANG.EXT)")
		backend  = fs.String("backend", "auto", "which API to use: auto, org (keyless XML-RPC) or com (REST, needs a key)")
		apiKey   = fs.String("api-key", "", "opensubtitles.com API key (or set OPENSUBTITLES_API_KEY); only used by the com backend")
		user     = fs.String("username", "", "opensubtitles.com username (or set OPENSUBTITLES_USERNAME)")
		pass     = fs.String("password", "", "opensubtitles.com password (or set OPENSUBTITLES_PASSWORD)")
		list     = fs.Bool("list", false, "list matching subtitles and exit without downloading")
		format   = fs.String("format", "", "only accept this subtitle format, e.g. srt (default: any)")
		sdh      = fs.Bool("sdh", false, "prefer hearing-impaired (SDH) subtitles")
		force    = fs.Bool("force", false, "overwrite the output file if it already exists")
		noRepair = fs.Bool("no-repair", false, "skip the split-cue repair pass")
		timeout  = fs.Duration("timeout", 30*time.Second, "network timeout per request")
		showVer  = fs.Bool("version", false, "print the version and exit")
	)

	fs.Usage = func() {
		fmt.Fprintf(stdout, "vlsub-go %s — download subtitles from OpenSubtitles for a local video file.\n\n", version)
		fmt.Fprintf(stdout, "Usage:\n  vlsub-go -file MOVIE [-lang eng]\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprint(stdout, `
No account or API key is needed. By default vlsub-go uses the keyless XML-RPC API
on opensubtitles.org, the same one the vlsub VLC extension uses.

The file is identified by its OpenSubtitles hash, so a match is timed against
your exact release. If the hash finds nothing, vlsub-go falls back to searching by
the title parsed from the filename.

To use the newer REST API on opensubtitles.com instead, supply a key:
  export OPENSUBTITLES_API_KEY=...        # from opensubtitles.com -> Consumers
  export OPENSUBTITLES_USERNAME=...       # optional, raises the quota
  export OPENSUBTITLES_PASSWORD=...
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVer {
		fmt.Fprintf(stdout, "vlsub-go %s\n", version)
		return nil
	}
	if *file == "" {
		fs.Usage()
		return errors.New("-file is required")
	}

	language, err := ParseLang(*lang)
	if err != nil {
		return err
	}
	if _, err := os.Stat(*file); err != nil {
		return fmt.Errorf("cannot read -file: %w", err)
	}

	hc := &http.Client{Timeout: *timeout}
	key := firstNonEmpty(*apiKey, os.Getenv("OPENSUBTITLES_API_KEY"))

	provider, err := pickProvider(*backend, key, hc)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "using %s\n", provider.Name())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Signing in is only meaningful for the REST backend.
	if com, ok := provider.(*ComProvider); ok {
		if u := firstNonEmpty(*user, os.Getenv("OPENSUBTITLES_USERNAME")); u != "" {
			p := firstNonEmpty(*pass, os.Getenv("OPENSUBTITLES_PASSWORD"))
			if p == "" {
				return errors.New("username given without a password: pass -password or set OPENSUBTITLES_PASSWORD")
			}
			acct, err := com.Login(ctx, u, p)
			if err != nil {
				return fmt.Errorf("login: %w", err)
			}
			fmt.Fprintf(stdout, "signed in as %s (%s, %d downloads/day)\n", u, acct.Level, acct.AllowedDownloads)
		}
	}

	results, err := search(ctx, provider, stdout, *file, language)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no %s subtitles found for %s", language.ISO3, filepath.Base(*file))
	}

	ranked := rank(results, *sdh)
	if want := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(*format), ".")); want != "" {
		kept := make([]Result, 0, len(ranked))
		for _, r := range ranked {
			if r.Ext() == want {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			return fmt.Errorf("none of the %d result(s) are in %s format", len(ranked), want)
		}
		fmt.Fprintf(stdout, "%d of %d result(s) are %s\n", len(kept), len(ranked), want)
		ranked = kept
	}
	if *list {
		fmt.Fprintf(stdout, "\n%d result(s), best first:\n", len(ranked))
		for i, r := range ranked {
			fmt.Fprintf(stdout, "%3d. %s\n", i+1, r.Label())
		}
		return nil
	}

	best := ranked[0]
	dest := *out
	if dest == "" {
		dest = defaultOutPath(*file, *lang, best.Ext())
	}
	if !*force {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("%s already exists (use -force to overwrite)", dest)
		}
	}
	fmt.Fprintf(stdout, "selected: %s\n", best.Label())

	raw, remoteName, note, err := provider.Fetch(ctx, best)
	if err != nil {
		return err
	}

	body, converted := EnsureUTF8(raw)
	if converted {
		fmt.Fprintln(stdout, "converted from Windows-1252 to UTF-8")
	}
	if !*noRepair {
		repaired, merged := MergeSplitCues(body)
		if merged > 0 {
			body = repaired
			fmt.Fprintf(stdout, "repaired %d split cue(s) that would have rendered bottom-to-top\n", merged)
		}
	}

	if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	fmt.Fprintf(stdout, "wrote %s (%d bytes", dest, len(body))
	if remoteName != "" {
		fmt.Fprintf(stdout, ", from %s", remoteName)
	}
	fmt.Fprintln(stdout, ")")
	if note != "" {
		fmt.Fprintln(stdout, note)
	}
	return nil
}

// pickProvider resolves the -backend choice. "auto" prefers the keyless .org
// API and only uses .com when a key is available.
func pickProvider(backend, key string, hc *http.Client) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "org":
		return NewOrgProvider(hc), nil
	case "com":
		if key == "" {
			return nil, errors.New("the com backend needs an API key: pass -api-key or set OPENSUBTITLES_API_KEY " +
				"(create one free under Consumers at https://www.opensubtitles.com/en/consumers), " +
				"or use -backend org which needs no key")
		}
		return NewComProvider(key, hc), nil
	case "auto", "":
		if key != "" {
			return NewComProvider(key, hc), nil
		}
		return NewOrgProvider(hc), nil
	default:
		return nil, fmt.Errorf("unknown -backend %q: want auto, org or com", backend)
	}
}

// defaultOutPath builds MOVIE.LANG.EXT next to the video, the layout Jellyfin
// and Plex expect. The language is spelled as the user typed it so an existing
// library stays internally consistent.
func defaultOutPath(file, lang, ext string) string {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	return fmt.Sprintf("%s.%s.%s", base, strings.ToLower(strings.TrimSpace(lang)), ext)
}

// search tries the hash first, then falls back to a title search.
func search(ctx context.Context, p Provider, stdout io.Writer, path string, lang Language) ([]Result, error) {
	q := Query{Lang: lang}

	hash, size, err := HashFile(path)
	switch {
	case errors.Is(err, ErrFileTooSmall):
		fmt.Fprintln(stdout, "file is under 128 KiB, skipping the hash lookup")
	case err != nil:
		return nil, fmt.Errorf("hashing %s: %w", path, err)
	default:
		fmt.Fprintf(stdout, "hash %s (%.1f MiB)\n", hash, float64(size)/(1<<20))
		q.Hash, q.Size = hash, size
	}

	q.Text, q.Season, q.Episode = ParseName(path)
	if q.Hash == "" && q.Text == "" {
		return nil, errors.New("no hash and no title could be derived from the filename")
	}
	if q.Text != "" {
		if q.Season > 0 {
			fmt.Fprintf(stdout, "searching %q S%02dE%02d in %s\n", q.Text, q.Season, q.Episode, lang.ISO3)
		} else {
			fmt.Fprintf(stdout, "searching %q in %s\n", q.Text, lang.ISO3)
		}
	}

	results, err := p.Search(ctx, q)
	if err != nil {
		return nil, err
	}

	hashHits := 0
	for _, r := range results {
		if r.HashMatch {
			hashHits++
		}
	}
	fmt.Fprintf(stdout, "%d result(s), %d matched by hash\n", len(results), hashHits)
	return results, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
