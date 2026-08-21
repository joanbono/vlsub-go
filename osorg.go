package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// OrgProvider talks to the legacy XML-RPC API on opensubtitles.org. It needs no
// API key: LogIn is called with empty credentials and only a registered
// User-Agent, which is how the vlsub VLC extension works.
type OrgProvider struct {
	Endpoint  string
	UserAgent string
	HTTP      *http.Client

	token string
}

func NewOrgProvider(hc *http.Client) *OrgProvider {
	return &OrgProvider{
		Endpoint:  "https://api.opensubtitles.org/xml-rpc",
		UserAgent: orgUserAgent,
		HTTP:      hc,
	}
}

func (p *OrgProvider) Name() string { return "opensubtitles.org (XML-RPC, no key required)" }

func (p *OrgProvider) call(ctx context.Context, method string, params ...any) (map[string]any, error) {
	body, err := encodeRequest(method, params...)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("User-Agent", p.UserAgent)

	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s: HTTP %d", method, resp.StatusCode)
	}

	v, err := decodeResponse(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected a struct, got %T", method, v)
	}
	if st := mapStr(m, "status"); st != "" && !strings.HasPrefix(st, "200") {
		return nil, fmt.Errorf("%s: %s", method, st)
	}
	return m, nil
}

// login obtains an anonymous session token.
func (p *OrgProvider) login(ctx context.Context) error {
	if p.token != "" {
		return nil
	}
	m, err := p.call(ctx, "LogIn", "", "", "en", p.UserAgent)
	if err != nil {
		return err
	}
	p.token = mapStr(m, "token")
	if p.token == "" {
		return fmt.Errorf("LogIn: server returned no token")
	}
	return nil
}

func (p *OrgProvider) Search(ctx context.Context, q Query) ([]Result, error) {
	if err := p.login(ctx); err != nil {
		return nil, err
	}

	// One call can carry several alternative criteria; the server reports which
	// one matched in MatchedBy, so we ask by hash and by title together.
	var criteria []any
	if q.Hash != "" && q.Size > 0 {
		criteria = append(criteria, map[string]any{
			"sublanguageid": q.Lang.ISO3,
			"moviehash":     q.Hash,
			"moviebytesize": strconv.FormatInt(q.Size, 10),
		})
	}
	if q.Text != "" {
		c := map[string]any{"sublanguageid": q.Lang.ISO3, "query": q.Text}
		if q.Season > 0 {
			c["season"] = strconv.Itoa(q.Season)
		}
		if q.Episode > 0 {
			c["episode"] = strconv.Itoa(q.Episode)
		}
		criteria = append(criteria, c)
	}
	if len(criteria) == 0 {
		return nil, fmt.Errorf("search needs a hash or a title")
	}

	m, err := p.call(ctx, "SearchSubtitles", p.token, criteria)
	if err != nil {
		return nil, err
	}

	// With no hits the server sends data as boolean false rather than an array.
	rows, ok := m["data"].([]any)
	if !ok {
		return nil, nil
	}

	out := make([]Result, 0, len(rows))
	for _, row := range rows {
		r, ok := row.(map[string]any)
		if !ok {
			continue
		}
		name := mapStr(r, "MovieReleaseName")
		if strings.TrimSpace(name) == "" {
			name = mapStr(r, "SubFileName")
		}
		rank := strings.ToLower(mapStr(r, "UserRank"))
		out = append(out, Result{
			Name:            strings.TrimSpace(name),
			Lang:            mapStr(r, "SubLanguageID"),
			Format:          mapStr(r, "SubFormat"),
			Downloads:       mapInt(r, "SubDownloadsCnt"),
			FPS:             mapFloat(r, "MovieFPS"),
			HearingImpaired: mapStr(r, "SubHearingImpaired") == "1",
			HashMatch:       mapStr(r, "MatchedBy") == "moviehash",
			Trusted:         rank == "trusted" || rank == "administrator" || rank == "platinum member",
			link:            mapStr(r, "SubDownloadLink"),
		})
	}
	return out, nil
}

func (p *OrgProvider) Fetch(ctx context.Context, r Result) ([]byte, string, string, error) {
	if r.link == "" {
		return nil, "", "", fmt.Errorf("result has no download link")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.link, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", p.UserAgent)

	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", "", fmt.Errorf("fetching subtitle: HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, "", "", err
	}

	data, err := gunzipIfNeeded(raw)
	if err != nil {
		return nil, "", "", err
	}
	return data, r.Name, "", nil
}

// gunzipIfNeeded decompresses the response when it carries a gzip magic
// number. The .org download endpoint always gzips, but tolerate plain bodies.
func gunzipIfNeeded(b []byte) ([]byte, error) {
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		return b, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("decompressing subtitle: %w", err)
	}
	defer zr.Close()

	out, err := io.ReadAll(io.LimitReader(zr, maxBody))
	if err != nil {
		return nil, fmt.Errorf("decompressing subtitle: %w", err)
	}
	return out, nil
}
