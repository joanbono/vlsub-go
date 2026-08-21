package main

import "testing"

func TestEnsureUTF8(t *testing.T) {
	t.Run("valid UTF-8 passes through", func(t *testing.T) {
		in := []byte("Ja, això és català — ¿verdad?")
		got, converted := EnsureUTF8(in)
		if converted {
			t.Error("valid UTF-8 should not be converted")
		}
		if got != string(in) {
			t.Errorf("got %q, want %q", got, string(in))
		}
	})

	t.Run("strips a BOM", func(t *testing.T) {
		got, _ := EnsureUTF8([]byte("\ufeffhello"))
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("transcodes latin-1 range", func(t *testing.T) {
		// 0xE0 is a lone invalid UTF-8 byte, and Windows-1252 reads it as "à".
		got, converted := EnsureUTF8([]byte{'c', 'a', 'f', 0xE8})
		if !converted {
			t.Fatal("expected a conversion")
		}
		if got != "cafè" {
			t.Errorf("got %q, want %q", got, "cafè")
		}
	})

	t.Run("transcodes the cp1252 high range", func(t *testing.T) {
		// 0x92 is a right single quote in Windows-1252, not in ISO-8859-1.
		got, converted := EnsureUTF8([]byte{'i', 't', 0x92, 's'})
		if !converted {
			t.Fatal("expected a conversion")
		}
		if got != "it’s" {
			t.Errorf("got %q, want %q", got, "it’s")
		}
	})
}
