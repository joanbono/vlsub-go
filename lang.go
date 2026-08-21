package main

import (
	"fmt"
	"sort"
	"strings"
)

// Language pairs the ISO 639-1 code the REST API expects with the ISO 639-2/B
// code the XML-RPC API expects. The two backends disagree, so both are kept.
type Language struct {
	ISO1 string // e.g. "en", "pt-BR"
	ISO3 string // e.g. "eng", "pob"
}

func (l Language) String() string { return l.ISO3 }

// The codes OpenSubtitles actually serves. Where ISO 639-2 offers a
// bibliographic and a terminological spelling, the table holds the one
// OpenSubtitles uses and aliases below map the other onto it.
var languages = []Language{
	{"af", "afr"}, {"ar", "ara"}, {"az", "aze"}, {"be", "bel"}, {"bg", "bul"},
	{"bn", "ben"}, {"bs", "bos"}, {"br", "bre"}, {"ca", "cat"}, {"cs", "cze"},
	{"cy", "wel"}, {"da", "dan"}, {"de", "ger"}, {"el", "ell"}, {"en", "eng"},
	{"eo", "epo"}, {"es", "spa"}, {"et", "est"}, {"eu", "baq"}, {"fa", "per"},
	{"fi", "fin"}, {"fr", "fre"}, {"ga", "gle"}, {"gd", "gla"}, {"gl", "glg"},
	{"he", "heb"}, {"hi", "hin"}, {"hr", "hrv"}, {"hu", "hun"}, {"hy", "arm"},
	{"id", "ind"}, {"is", "ice"}, {"it", "ita"}, {"ja", "jpn"}, {"ka", "geo"},
	{"kk", "kaz"}, {"km", "khm"}, {"ko", "kor"}, {"ku", "kur"}, {"la", "lat"},
	{"lb", "ltz"}, {"lt", "lit"}, {"lv", "lav"}, {"mk", "mac"}, {"ml", "mal"},
	{"mn", "mon"}, {"ms", "may"}, {"ne", "nep"}, {"nl", "dut"}, {"no", "nor"},
	{"oc", "oci"}, {"pl", "pol"}, {"pt", "por"}, {"ro", "rum"}, {"ru", "rus"},
	{"si", "sin"}, {"sk", "slo"}, {"sl", "slv"}, {"sq", "alb"}, {"sr", "scc"},
	{"sv", "swe"}, {"sw", "swa"}, {"ta", "tam"}, {"te", "tel"}, {"tl", "tgl"},
	{"th", "tha"}, {"tr", "tur"}, {"uk", "ukr"}, {"ur", "urd"}, {"vi", "vie"},
	{"zh", "chi"},

	// Regional variants that exist under their own code.
	{"pt-BR", "pob"}, {"pt-PT", "pom"}, {"zh-TW", "zht"},
}

// aliases map alternative spellings onto a canonical ISO3 code in the table.
var aliases = map[string]string{
	"deu": "ger", "fra": "fre", "ces": "cze", "gre": "ell", "slk": "slo",
	"ron": "rum", "nld": "dut", "isl": "ice", "hye": "arm", "kat": "geo",
	"fas": "per", "eus": "baq", "cym": "wel", "mkd": "mac", "sqi": "alb",
	"srp": "scc", "msa": "may", "zho": "chi", "hrv": "hrv", "pob": "pob",
}

var byCode = map[string]Language{}

func init() {
	for _, l := range languages {
		byCode[strings.ToLower(l.ISO1)] = l
		// First entry wins, so plain "zh" keeps "chi" rather than a variant.
		if _, taken := byCode[l.ISO3]; !taken {
			byCode[l.ISO3] = l
		}
	}
	for alt, canonical := range aliases {
		if l, ok := byCode[canonical]; ok {
			byCode[alt] = l
		}
	}
}

// ParseLang resolves a user-supplied code. It accepts ISO 639-1 ("en"),
// ISO 639-2 in either spelling ("eng", "ger", "deu") and regional forms
// ("pt-BR", "pob").
func ParseLang(in string) (Language, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return Language{}, fmt.Errorf("empty language code")
	}
	if l, ok := byCode[s]; ok {
		return l, nil
	}
	// Normalise a regional form such as "pt-br" to "pt-BR" and retry.
	if base, region, found := strings.Cut(s, "-"); found {
		if l, ok := byCode[base+"-"+strings.ToUpper(region)]; ok {
			return l, nil
		}
		if l, ok := byCode[base]; ok {
			return l, nil
		}
	}
	return Language{}, fmt.Errorf("unknown language %q; expected a code like %s",
		in, strings.Join(sampleCodes(8), ", "))
}

func sampleCodes(n int) []string {
	common := []string{"eng", "spa", "cat", "fre", "ger", "ita", "por", "pt-BR"}
	if len(common) > n {
		common = common[:n]
	}
	sort.Strings(common)
	return common
}
