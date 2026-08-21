# vlsub-go

A Go port of the [vlsub](https://github.com/exebetche/vlsub) VLC extension, as a standalone CLI.

Point it at a video file, get a subtitle file next to it.

```sh
vlsub-go -file "Amelie.mkv"              # -lang defaults to eng
vlsub-go -file "Amelie.mkv" -lang cat
```

**No account or API key required.** Like vlsub, it calls `LogIn` on the XML-RPC API at
`api.opensubtitles.org` with empty credentials and a registered User-Agent.

Also like vlsub, it identifies the file by its **OpenSubtitles hash** — a 64-bit checksum
of the file size plus the first and last 64 KiB — so a match is timed against your exact
release rather than merely the right title. If the hash finds nothing, it falls back to
searching by the title parsed from the filename.

## Install

Download a binary for your platform from the
[latest release](https://github.com/joanbono/vlsub-go/releases/latest) — Linux, macOS and
Windows on amd64 and arm64. Each release ships a `checksums.txt`:

```sh
sha256sum -c checksums.txt --ignore-missing
```

With a Go toolchain:

```sh
go install github.com/joanbono/vlsub-go@latest
```

Or from a clone:

```sh
git clone https://github.com/joanbono/vlsub-go && cd vlsub-go
go build -o vlsub-go .
```

Requires Go 1.26 or newer. No dependencies outside the standard library.

## Releasing

Releases are driven by the [`VERSION`](VERSION) file. Bump it and push to `main`:

```sh
echo 0.3.0 > VERSION
git commit -am "Release 0.3.0" && git push
```

[`.github/workflows/release.yml`](.github/workflows/release.yml) then tags `v0.3.0`, builds
all six targets, packages each with the README and LICENSE, writes `checksums.txt`, and
publishes the GitHub release. No manual tagging, and no third-party actions.

The version is stamped into the binary with `-ldflags "-X main.version=..."`, so
`vlsub-go -version` reports it.

Details worth knowing:

- **Pushes where `VERSION` has not changed do nothing.** The workflow compares it against
  the existing tags and stops if that version is already released, so ordinary commits to
  `main` are free.
- **`VERSION` must be plain semver** — `1.2.3`, or `1.2.3-rc1` for a prerelease. No leading
  `v`. Anything else fails the run rather than publishing a misnamed release.
- **Pushing a tag by hand still works** and takes precedence over the file.
- **A manual run** from the Actions tab builds every target and uploads the archives as
  workflow artifacts without publishing a release — useful for checking a build before
  committing to a version.

Tagging happens inside the release workflow rather than in a separate one by necessity: a
tag pushed using the default `GITHUB_TOKEN` does not trigger further workflow runs, so a
split auto-tag design would tag and then build nothing.

## Usage

Output lands beside the video as `NAME.LANG.EXT` — the convention Jellyfin and Plex expect,
using the language code exactly as you typed it, so `-lang eng` yields `Amelie.eng.srt`.

Inspect the candidates before spending a download:

```console
$ vlsub-go -file "The Big Bang Theory 01x03.mkv" -list
using opensubtitles.org (XML-RPC, no key required)
hash 6b431861ec0d068f (176.1 MiB)
searching "The Big Bang Theory" S01E03 in eng
16 result(s), 1 matched by hash

  1. Big Bang Theory S01E03 The Fuzzy Boots Corollary.DVD.NonHI [eng] hash-match, trusted, 29.97fps, ssa, 6497 dl
  2. the.big.bang.theory.s01e03.hdtv.xvid-xor                   [eng] trusted, 23.98fps, srt, 210654 dl
  3. The.Big.Bang.Theory.S01.DVDRip.XviD-FoV                    [eng] trusted, 23.976fps, srt, 183922 dl
  ...
```

The top hit there is an `.ssa`. A hash match outranks everything else because it is the
only signal that the subtitle was timed against your specific file — pass `-format srt` if
you would rather have SubRip and accept a looser match.

### Flags

| flag | default | meaning |
|---|---|---|
| `-file` | *(required)* | video file to match |
| `-lang` | `eng` | `eng`, `en`, `spa`, `cat`, `pt-BR`; ISO 639-1, 639-2/B or 639-2/T |
| `-out` | *(auto)* | explicit output path |
| `-list` | `false` | print ranked matches and exit without downloading |
| `-format` | *(any)* | only accept this format, e.g. `srt` |
| `-sdh` | `false` | prefer hearing-impaired (SDH) subtitles |
| `-force` | `false` | overwrite an existing output file |
| `-no-repair` | `false` | skip the split-cue repair pass |
| `-backend` | `auto` | `org` (keyless), `com` (REST, needs a key), or `auto` |
| `-api-key` | env | `com` backend only; or `OPENSUBTITLES_API_KEY` |
| `-username` | env | `com` backend only; or `OPENSUBTITLES_USERNAME` |
| `-password` | env | `com` backend only; or `OPENSUBTITLES_PASSWORD` |
| `-timeout` | `30s` | per-request network timeout |
| `-version` | | print the version and exit |

### Whole directories

One file per invocation, by design. For a library, drive it from the shell:

```sh
find . -name '*.mkv' -print0 | xargs -0 -I{} vlsub-go -file {} -lang eng -format srt
```

An existing output file is skipped unless you pass `-force`, so re-running is safe and
only fills the gaps.

## Backends

|  | `org` (default) | `com` |
|---|---|---|
| endpoint | `api.opensubtitles.org/xml-rpc` | `api.opensubtitles.com/api/v1` |
| API key | none | required |
| status | works, but officially deprecated | actively maintained |
| language codes | ISO 639-2/B (`eng`) | ISO 639-1 (`en`) |

`-backend auto` uses `com` when an API key is present and `org` otherwise, so the tool
works with zero setup but upgrades itself the moment you export a key:

```sh
export OPENSUBTITLES_API_KEY=...      # opensubtitles.com -> Consumers
export OPENSUBTITLES_USERNAME=...     # optional, raises the download quota
export OPENSUBTITLES_PASSWORD=...
```

The language mapping is internal, so `-lang eng` is correct for either backend.

> The `org` XML-RPC API has been slated for retirement for years. It is live as of this
> writing and is what vlsub itself uses. If it does go away, export a free key as above and
> nothing else changes.

### Result ranking

Best first: hash match, then SRT over other formats, then your `-sdh` preference, then
trusted uploader, then download count.

## Split-cue repair

Some OpenSubtitles uploads — SDH ones especially — store a two-line cue as *two
consecutive cues sharing one timing*:

```srt
2
00:00:14,710 --> 00:00:17,940
Here we are, gentlemen,

3
00:00:14,710 --> 00:00:17,940
the Gates of Elzebub.
```

Players anchor subtitles to the bottom of the frame and stack simultaneous cues upward, so
the second line renders *above* the first and the sentence reads backwards. On dialogue
cues (`- No.` / `- Here, let me try.`) it also misattributes lines to the wrong speaker.

`vlsub-go` detects consecutive cues with identical timings, merges them, and renumbers.
It runs by default, reports how many cues it merged, and leaves files that have no
duplicate timings byte-identical. Disable with `-no-repair`.

Subtitles that are not valid UTF-8 are transcoded from Windows-1252, which is what the
`org` backend commonly serves.

## How the hash works

```
hash = filesize
     + every little-endian uint64 in the first 64 KiB
     + every little-endian uint64 in the last  64 KiB
```

All arithmetic wraps at 64 bits, and the result is rendered as 16 hex digits. Files under
128 KiB cannot be hashed, since the two chunks would overlap; those fall back to a title
search.

## Layout

| file | role |
|---|---|
| `main.go` | flags, backend selection, orchestration |
| `provider.go` | `Provider` interface, normalised `Query`/`Result`, ranking |
| `osorg.go` | opensubtitles.org backend (keyless XML-RPC) |
| `oscom.go` | opensubtitles.com backend (REST v1) |
| `xmlrpc.go` | minimal XML-RPC codec — Go has none in the standard library |
| `hash.go` | OpenSubtitles hash |
| `lang.go` | language codes, mapped for both backends |
| `naming.go` | title, season and episode parsed from a filename |
| `srt.go` | split-cue repair |
| `encoding.go` | Windows-1252 to UTF-8 |

## Tests

```sh
go test ./...
```

The hash tests derive their expected values from the specification by hand rather than
from a reference implementation, so they check the algorithm rather than pinning current
behaviour: an all-zero file hashes to its own size, an all-ones file exercises 64-bit
wraparound, and a file with a dirty middle section proves only the end chunks are read.
The XML-RPC tests decode a real captured `LogIn` response, an array-of-structs search
result, a fault, and the no-results case where the server sends `data` as boolean `false`
instead of an empty array.

## Status

The `org` backend is verified end-to-end against the live API: hashing, search, ranking,
gzip download and write. The `com` backend is written to the documented contract but has
not been exercised against a real API key.

Language coverage is the set OpenSubtitles actually serves, not all of ISO 639.

## Credit

The approach, the hash and the keyless anonymous login are all from
[exebetche/vlsub](https://github.com/exebetche/vlsub) by Antoine Bécot.

## License

[Apache-2.0](LICENSE)
