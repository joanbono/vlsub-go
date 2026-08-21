package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ComProvider talks to the REST API v1 on opensubtitles.com. It requires an API
// key, but is the endpoint OpenSubtitles actively maintains.
type ComProvider struct {
	APIKey string
	Host   string
	Token  string // set by Login; switches to the account download quota
	HTTP   *http.Client

	lastQuota Quota
}

func NewComProvider(apiKey string, hc *http.Client) *ComProvider {
	return &ComProvider{APIKey: apiKey, Host: "api.opensubtitles.com", HTTP: hc}
}

func (c *ComProvider) Name() string { return "opensubtitles.com (REST v1, API key)" }

// Quota reports the download allowance after a download request.
type Quota struct {
	Used      int
	Remaining int
	ResetTime string
}

// APIError carries the status so the common failures get an explanation rather
// than a bare code.
type APIError struct {
	Status   int
	Endpoint string
	Body     string
}

func (e *APIError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("%s: rejected (HTTP %d) — check the API key and, if set, the username and password", e.Endpoint, e.Status)
	case http.StatusNotAcceptable:
		return fmt.Sprintf("%s: download quota exhausted (HTTP 406) — the allowance resets every 24h", e.Endpoint)
	case http.StatusTooManyRequests:
		return fmt.Sprintf("%s: rate limited (HTTP 429) — wait a few seconds and retry", e.Endpoint)
	default:
		body := e.Body
		if len(body) > 300 {
			body = body[:300] + "…"
		}
		return fmt.Sprintf("%s: HTTP %d: %s", e.Endpoint, e.Status, body)
	}
}

func (c *ComProvider) do(ctx context.Context, method, endpoint string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://"+c.Host+"/api/v1"+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", c.APIKey)
	req.Header.Set("User-Agent", comUserAgent())
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, Endpoint: endpoint, Body: strings.TrimSpace(string(data))}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: decoding response: %w", endpoint, err)
	}
	return nil
}

// Account describes the signed-in user's allowance.
type Account struct {
	AllowedDownloads int
	Level            string
	VIP              bool
}

// Login exchanges credentials for a bearer token. Optional: search works with
// the key alone, and downloads fall back to the anonymous quota.
func (c *ComProvider) Login(ctx context.Context, user, pass string) (Account, error) {
	var resp struct {
		Token   string `json:"token"`
		BaseURL string `json:"base_url"`
		User    struct {
			AllowedDownloads int    `json:"allowed_downloads"`
			Level            string `json:"level"`
			VIP              bool   `json:"vip"`
		} `json:"user"`
	}
	if err := c.do(ctx, http.MethodPost, "/login",
		map[string]string{"username": user, "password": pass}, &resp); err != nil {
		return Account{}, err
	}
	c.Token = resp.Token
	// VIP accounts are served from another host; honour the redirect.
	if h := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(resp.BaseURL, "https://"), "http://"), "/"); h != "" {
		c.Host = h
	}
	return Account{
		AllowedDownloads: resp.User.AllowedDownloads,
		Level:            resp.User.Level,
		VIP:              resp.User.VIP,
	}, nil
}

func (c *ComProvider) Search(ctx context.Context, q Query) ([]Result, error) {
	v := url.Values{}
	v.Set("languages", q.Lang.ISO1)
	if q.Hash != "" {
		v.Set("moviehash", q.Hash)
	}
	if q.Text != "" {
		v.Set("query", q.Text)
	}
	if q.Season > 0 {
		v.Set("season_number", strconv.Itoa(q.Season))
	}
	if q.Episode > 0 {
		v.Set("episode_number", strconv.Itoa(q.Episode))
	}

	var resp struct {
		Data []struct {
			Attributes struct {
				Language        string  `json:"language"`
				DownloadCount   int     `json:"download_count"`
				HearingImpaired bool    `json:"hearing_impaired"`
				MovieHashMatch  bool    `json:"moviehash_match"`
				FromTrusted     bool    `json:"from_trusted"`
				Release         string  `json:"release"`
				Format          string  `json:"format"`
				FPS             float64 `json:"fps"`
				Files           []struct {
					FileID   int    `json:"file_id"`
					FileName string `json:"file_name"`
				} `json:"files"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/subtitles?"+v.Encode(), nil, &resp); err != nil {
		return nil, err
	}

	out := make([]Result, 0, len(resp.Data))
	for _, d := range resp.Data {
		a := d.Attributes
		if len(a.Files) == 0 {
			continue // nothing downloadable attached
		}
		name := a.Release
		if strings.TrimSpace(name) == "" {
			name = a.Files[0].FileName
		}
		out = append(out, Result{
			Name:            strings.TrimSpace(name),
			Lang:            a.Language,
			Format:          a.Format,
			Downloads:       a.DownloadCount,
			FPS:             a.FPS,
			HearingImpaired: a.HearingImpaired,
			HashMatch:       a.MovieHashMatch,
			Trusted:         a.FromTrusted,
			fileID:          a.Files[0].FileID,
		})
	}
	return out, nil
}

func (c *ComProvider) Fetch(ctx context.Context, r Result) ([]byte, string, string, error) {
	var resp struct {
		Link      string `json:"link"`
		FileName  string `json:"file_name"`
		Requests  int    `json:"requests"`
		Remaining int    `json:"remaining"`
		ResetTime string `json:"reset_time"`
	}
	if err := c.do(ctx, http.MethodPost, "/download",
		map[string]int{"file_id": r.fileID}, &resp); err != nil {
		return nil, "", "", err
	}
	c.lastQuota = Quota{Used: resp.Requests, Remaining: resp.Remaining, ResetTime: resp.ResetTime}
	note := fmt.Sprintf("quota: %d used, %d remaining (resets in %s)",
		resp.Requests, resp.Remaining, resp.ResetTime)

	if resp.Link == "" {
		return nil, "", note, fmt.Errorf("download: API returned no link for file %d", r.fileID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.Link, nil)
	if err != nil {
		return nil, "", note, err
	}
	req.Header.Set("User-Agent", comUserAgent())

	dl, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", note, err
	}
	defer dl.Body.Close()

	if dl.StatusCode < 200 || dl.StatusCode > 299 {
		return nil, "", note, fmt.Errorf("fetching subtitle: HTTP %d", dl.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(dl.Body, maxBody))
	if err != nil {
		return nil, "", note, err
	}
	return data, resp.FileName, note, nil
}
