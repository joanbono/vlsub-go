package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The expected values below are derived by hand from the spec rather than
// copied from an implementation, so they check the algorithm and not just
// that it stayed the same.
func TestHashAt(t *testing.T) {
	const size = chunkSize * 2

	t.Run("all zero bytes sum to the file size", func(t *testing.T) {
		// Every uint64 is 0, so the sum collapses to size: 131072 = 0x20000.
		got, err := hashAt(bytes.NewReader(make([]byte, size)), size)
		if err != nil {
			t.Fatalf("hashAt: %v", err)
		}
		if want := uint64(0x20000); got != want {
			t.Errorf("got %#016x, want %#016x", got, want)
		}
	})

	t.Run("all one bits wrap around", func(t *testing.T) {
		// Each chunk holds 8192 uint64 of 2^64-1, summing to -8192 mod 2^64.
		// Two chunks give -16384, so the total is 131072-16384 = 114688.
		data := bytes.Repeat([]byte{0xFF}, size)
		got, err := hashAt(bytes.NewReader(data), size)
		if err != nil {
			t.Fatalf("hashAt: %v", err)
		}
		if want := uint64(0x1c000); got != want {
			t.Errorf("got %#016x, want %#016x", got, want)
		}
	})

	t.Run("only the two end chunks are read", func(t *testing.T) {
		// A middle section set to 0xFF must not change the result versus an
		// all-zero file of the same length.
		big := int64(chunkSize * 5)
		data := make([]byte, big)
		copy(data[chunkSize+8:big-chunkSize-8], bytes.Repeat([]byte{0xFF}, chunkSize))
		got, err := hashAt(bytes.NewReader(data), big)
		if err != nil {
			t.Fatalf("hashAt: %v", err)
		}
		if want := uint64(big); got != want {
			t.Errorf("middle bytes leaked into hash: got %#016x, want %#016x", got, want)
		}
	})

	t.Run("rejects short files", func(t *testing.T) {
		short := int64(chunkSize*2 - 1)
		if _, err := hashAt(bytes.NewReader(make([]byte, short)), short); !errors.Is(err, ErrFileTooSmall) {
			t.Errorf("got %v, want ErrFileTooSmall", err)
		}
	})
}

func TestHashFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "movie.mkv")
	if err := os.WriteFile(path, make([]byte, chunkSize*2), 0o644); err != nil {
		t.Fatal(err)
	}

	hash, size, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if want := "0000000000020000"; hash != want {
		t.Errorf("hash = %q, want %q", hash, want)
	}
	if want := int64(chunkSize * 2); size != want {
		t.Errorf("size = %d, want %d", size, want)
	}
	if len(hash) != 16 {
		t.Errorf("hash %q is %d chars, want 16", hash, len(hash))
	}
}
