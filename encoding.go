package main

import (
	"strings"
	"unicode/utf8"
)

// cp1252High maps the bytes 0x80–0x9F, where Windows-1252 differs from
// ISO-8859-1, onto their Unicode code points. 0x81, 0x8D, 0x8F, 0x90 and 0x9D
// are unassigned and fall back to the raw byte value.
var cp1252High = [32]rune{
	'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡',
	'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—',
	'˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
}

// EnsureUTF8 returns the text as valid UTF-8. Subtitles from opensubtitles.org
// are frequently Windows-1252 rather than UTF-8; if the input is not valid
// UTF-8 it is transcoded on that assumption, which is right far more often than
// leaving mojibake in place. The bool reports whether a conversion happened.
func EnsureUTF8(b []byte) (string, bool) {
	if utf8.Valid(b) {
		return strings.TrimPrefix(string(b), "\ufeff"), false
	}

	var sb strings.Builder
	sb.Grow(len(b) + len(b)/4)
	for _, c := range b {
		switch {
		case c < 0x80:
			sb.WriteByte(c)
		case c < 0xA0:
			sb.WriteRune(cp1252High[c-0x80])
		default:
			sb.WriteRune(rune(c))
		}
	}
	return sb.String(), true
}
