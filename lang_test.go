package main

import "testing"

func TestParseLang(t *testing.T) {
	tests := []struct {
		in         string
		iso1, iso3 string
	}{
		{"eng", "en", "eng"},
		{"ENG", "en", "eng"},
		{" eng ", "en", "eng"},
		{"en", "en", "eng"},
		{"spa", "es", "spa"},
		{"cat", "ca", "cat"},
		{"ca", "ca", "cat"},
		{"ger", "de", "ger"}, // ISO 639-2/B
		{"deu", "de", "ger"}, // ISO 639-2/T alias
		{"fre", "fr", "fre"},
		{"fra", "fr", "fre"},
		{"gre", "el", "ell"}, // OpenSubtitles serves Greek as "ell"
		{"srp", "sr", "scc"}, // and Serbian as "scc"
		{"pob", "pt-BR", "pob"},
		{"pt-br", "pt-BR", "pob"},
		{"PT-BR", "pt-BR", "pob"},
		{"zh", "zh", "chi"},
		{"chi", "zh", "chi"}, // must not resolve to the zh-TW variant
	}

	for _, tc := range tests {
		got, err := ParseLang(tc.in)
		if err != nil {
			t.Errorf("ParseLang(%q): %v", tc.in, err)
			continue
		}
		if got.ISO1 != tc.iso1 || got.ISO3 != tc.iso3 {
			t.Errorf("ParseLang(%q) = {%s %s}, want {%s %s}",
				tc.in, got.ISO1, got.ISO3, tc.iso1, tc.iso3)
		}
	}

	for _, in := range []string{"", "   ", "zzz", "english", "e"} {
		if got, err := ParseLang(in); err == nil {
			t.Errorf("ParseLang(%q) = %v, want error", in, got)
		}
	}
}
