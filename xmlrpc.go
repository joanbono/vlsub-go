package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// A minimal XML-RPC codec. The OpenSubtitles.org API uses only strings, ints,
// doubles, booleans, structs and arrays, so that is all this supports.

func encodeRequest(method string, params ...any) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><methodCall><methodName>`)
	b.WriteString(method)
	b.WriteString(`</methodName><params>`)
	for _, p := range params {
		b.WriteString("<param>")
		if err := encodeValue(&b, p); err != nil {
			return nil, err
		}
		b.WriteString("</param>")
	}
	b.WriteString("</params></methodCall>")
	return []byte(b.String()), nil
}

func encodeValue(b *strings.Builder, v any) error {
	b.WriteString("<value>")
	switch t := v.(type) {
	case string:
		b.WriteString("<string>")
		if err := xml.EscapeText(b, []byte(t)); err != nil {
			return err
		}
		b.WriteString("</string>")
	case int:
		fmt.Fprintf(b, "<int>%d</int>", t)
	case int64:
		fmt.Fprintf(b, "<int>%d</int>", t)
	case float64:
		fmt.Fprintf(b, "<double>%g</double>", t)
	case bool:
		n := 0
		if t {
			n = 1
		}
		fmt.Fprintf(b, "<boolean>%d</boolean>", n)
	case []any:
		b.WriteString("<array><data>")
		for _, e := range t {
			if err := encodeValue(b, e); err != nil {
				return err
			}
		}
		b.WriteString("</data></array>")
	case map[string]any:
		b.WriteString("<struct>")
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministic output, which keeps tests stable
		for _, k := range keys {
			b.WriteString("<member><name>")
			if err := xml.EscapeText(b, []byte(k)); err != nil {
				return err
			}
			b.WriteString("</name>")
			if err := encodeValue(b, t[k]); err != nil {
				return err
			}
			b.WriteString("</member>")
		}
		b.WriteString("</struct>")
	default:
		return fmt.Errorf("xmlrpc: cannot encode %T", v)
	}
	b.WriteString("</value>")
	return nil
}

// decodeResponse returns the single value carried by a methodResponse, or an
// error describing the fault if the server sent one.
func decodeResponse(r io.Reader) (any, error) {
	d := xml.NewDecoder(r)
	faulted := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("xmlrpc: response carried no value")
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "fault":
			faulted = true
		case "value":
			v, err := decodeValue(d)
			if err != nil {
				return nil, err
			}
			if faulted {
				return nil, fmt.Errorf("xmlrpc fault: %v", v)
			}
			return v, nil
		}
	}
}

// decodeValue reads the body of a <value> element whose start tag has already
// been consumed. An untyped value is treated as a string, per the spec.
func decodeValue(d *xml.Decoder) (any, error) {
	var (
		result any
		typed  bool
		chars  strings.Builder
	)
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if !typed {
				chars.Write(t)
			}
		case xml.StartElement:
			v, err := decodeTyped(d, t)
			if err != nil {
				return nil, err
			}
			result, typed = v, true
		case xml.EndElement:
			if t.Name.Local == "value" {
				if typed {
					return result, nil
				}
				return chars.String(), nil
			}
		}
	}
}

func decodeTyped(d *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "string", "dateTime.iso8601", "base64":
		var s string
		err := d.DecodeElement(&s, &se)
		return s, err
	case "int", "i4", "i8":
		s, err := elementText(d, se)
		if err != nil {
			return nil, err
		}
		return strconv.ParseInt(s, 10, 64)
	case "double":
		s, err := elementText(d, se)
		if err != nil {
			return nil, err
		}
		return strconv.ParseFloat(s, 64)
	case "boolean":
		s, err := elementText(d, se)
		if err != nil {
			return nil, err
		}
		return s == "1" || strings.EqualFold(s, "true"), nil
	case "array":
		return decodeArray(d)
	case "struct":
		return decodeStruct(d)
	default:
		// Unknown or <nil/>: consume it and report nothing.
		return nil, d.Skip()
	}
}

func elementText(d *xml.Decoder, se xml.StartElement) (string, error) {
	var s string
	if err := d.DecodeElement(&s, &se); err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func decodeArray(d *xml.Decoder) (any, error) {
	out := []any{}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "value" {
				v, err := decodeValue(d)
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
		case xml.EndElement:
			if t.Name.Local == "array" {
				return out, nil
			}
		}
	}
}

func decodeStruct(d *xml.Decoder) (any, error) {
	out := map[string]any{}
	var name string
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "name":
				s, err := elementText(d, t)
				if err != nil {
					return nil, err
				}
				name = s
			case "value":
				v, err := decodeValue(d)
				if err != nil {
					return nil, err
				}
				out[name] = v
			}
		case xml.EndElement:
			if t.Name.Local == "struct" {
				return out, nil
			}
		}
	}
}

// OpenSubtitles returns most numbers as strings, so these accessors accept
// either form.

func mapStr(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

func mapInt(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	default:
		return 0
	}
}

func mapFloat(m map[string]any, k string) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}
