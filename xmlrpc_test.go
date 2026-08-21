package main

import (
	"strings"
	"testing"
)

func TestEncodeRequest(t *testing.T) {
	got, err := encodeRequest("LogIn", "", "", "en", "vlsub-go v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0"?><methodCall><methodName>LogIn</methodName><params>` +
		`<param><value><string></string></value></param>` +
		`<param><value><string></string></value></param>` +
		`<param><value><string>en</string></value></param>` +
		`<param><value><string>vlsub-go v0.2.0</string></value></param>` +
		`</params></methodCall>`
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEncodeRequestNestedAndEscaped(t *testing.T) {
	got, err := encodeRequest("SearchSubtitles", "tok", []any{
		map[string]any{"sublanguageid": "eng", "query": "a & b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	// Struct members are emitted in sorted key order.
	if !strings.Contains(s, "<member><name>query</name><value><string>a &amp; b</string></value></member>"+
		"<member><name>sublanguageid</name><value><string>eng</string></value></member>") {
		t.Errorf("unexpected struct encoding: %s", s)
	}
	if !strings.Contains(s, "<array><data>") {
		t.Errorf("array not encoded: %s", s)
	}
}

func TestDecodeResponseLogin(t *testing.T) {
	// Captured from the live api.opensubtitles.org endpoint.
	in := `<?xml version="1.0" encoding="utf-8"?><methodResponse><params><param><value><struct>` +
		`<member><name>token</name><value><string>3v3h7xjoGMutP44eLY3wW52DfUe</string></value></member>` +
		`<member><name>status</name><value><string>200 OK</string></value></member>` +
		`<member><name>seconds</name><value><double>0.002000</double></value></member>` +
		`</struct></value></param></params></methodResponse>`

	v, err := decodeResponse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map", v)
	}
	if got := mapStr(m, "token"); got != "3v3h7xjoGMutP44eLY3wW52DfUe" {
		t.Errorf("token = %q", got)
	}
	if got := mapStr(m, "status"); got != "200 OK" {
		t.Errorf("status = %q", got)
	}
	if got := mapFloat(m, "seconds"); got != 0.002 {
		t.Errorf("seconds = %v", got)
	}
}

func TestDecodeResponseArrayOfStructs(t *testing.T) {
	in := `<methodResponse><params><param><value><struct>` +
		`<member><name>status</name><value><string>200 OK</string></value></member>` +
		`<member><name>data</name><value><array><data>` +
		`<value><struct>` +
		`<member><name>SubFileName</name><value><string>one.srt</string></value></member>` +
		`<member><name>SubDownloadsCnt</name><value><string>183922</string></value></member>` +
		`<member><name>MatchedBy</name><value><string>moviehash</string></value></member>` +
		`</struct></value>` +
		`<value><struct>` +
		`<member><name>SubFileName</name><value><string>two.srt</string></value></member>` +
		`<member><name>SubDownloadsCnt</name><value><int>7</int></value></member>` +
		`</struct></value>` +
		`</data></array></value></member>` +
		`</struct></value></param></params></methodResponse>`

	v, err := decodeResponse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	arr, ok := m["data"].([]any)
	if !ok {
		t.Fatalf("data is %T, want []any", m["data"])
	}
	if len(arr) != 2 {
		t.Fatalf("got %d entries, want 2", len(arr))
	}
	first := arr[0].(map[string]any)
	if got := mapStr(first, "SubFileName"); got != "one.srt" {
		t.Errorf("name = %q", got)
	}
	// Numbers arrive as strings from this API; mapInt must cope with both.
	if got := mapInt(first, "SubDownloadsCnt"); got != 183922 {
		t.Errorf("downloads = %d", got)
	}
	second := arr[1].(map[string]any)
	if got := mapInt(second, "SubDownloadsCnt"); got != 7 {
		t.Errorf("int-typed downloads = %d", got)
	}
}

func TestDecodeResponseFault(t *testing.T) {
	in := `<methodResponse><fault><value><struct>` +
		`<member><name>faultString</name><value><string>bad token</string></value></member>` +
		`<member><name>faultCode</name><value><int>401</int></value></member>` +
		`</struct></value></fault></methodResponse>`

	if _, err := decodeResponse(strings.NewReader(in)); err == nil {
		t.Fatal("want error for fault response")
	} else if !strings.Contains(err.Error(), "bad token") {
		t.Errorf("error %q should mention the fault string", err)
	}
}

func TestDecodeResponseNoData(t *testing.T) {
	// A search with no hits returns data as boolean false, not an array.
	in := `<methodResponse><params><param><value><struct>` +
		`<member><name>status</name><value><string>200 OK</string></value></member>` +
		`<member><name>data</name><value><boolean>0</boolean></value></member>` +
		`</struct></value></param></params></methodResponse>`

	v, err := decodeResponse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if _, ok := m["data"].([]any); ok {
		t.Error("data should not decode as an array")
	}
	if b, ok := m["data"].(bool); !ok || b {
		t.Errorf("data = %v, want false", m["data"])
	}
}
