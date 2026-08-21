package main

import "testing"

func TestMergeSplitCues(t *testing.T) {
	t.Run("merges consecutive cues sharing a timing", func(t *testing.T) {
		in := "1\n00:00:10,710 --> 00:00:14,540\nAll right.\n\n" +
			"2\n00:00:14,710 --> 00:00:17,940\nHere we are, gentlemen,\n\n" +
			"3\n00:00:14,710 --> 00:00:17,940\nthe Gates of Elzebub.\n\n"

		want := "1\n00:00:10,710 --> 00:00:14,540\nAll right.\n\n" +
			"2\n00:00:14,710 --> 00:00:17,940\nHere we are, gentlemen,\nthe Gates of Elzebub.\n\n"

		got, n := MergeSplitCues(in)
		if n != 1 {
			t.Errorf("merged %d cues, want 1", n)
		}
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("renumbers after merging", func(t *testing.T) {
		in := "1\n00:00:01,000 --> 00:00:02,000\nA\n\n" +
			"2\n00:00:01,000 --> 00:00:02,000\nB\n\n" +
			"3\n00:00:03,000 --> 00:00:04,000\nC\n\n"
		got, n := MergeSplitCues(in)
		if n != 1 {
			t.Fatalf("merged %d, want 1", n)
		}
		want := "1\n00:00:01,000 --> 00:00:02,000\nA\nB\n\n" +
			"2\n00:00:03,000 --> 00:00:04,000\nC\n\n"
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("leaves healthy input byte-identical", func(t *testing.T) {
		in := "1\n00:00:01,000 --> 00:00:02,000\nTwo real\nlines here\n\n" +
			"2\n00:00:03,000 --> 00:00:04,000\nAnother\n\n"
		got, n := MergeSplitCues(in)
		if n != 0 {
			t.Errorf("merged %d cues, want 0", n)
		}
		if got != in {
			t.Errorf("clean input was rewritten:\ngot:\n%q\nwant:\n%q", got, in)
		}
	})

	t.Run("handles CRLF and a BOM", func(t *testing.T) {
		in := "\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nA\r\n\r\n" +
			"2\r\n00:00:01,000 --> 00:00:02,000\r\nB\r\n"
		got, n := MergeSplitCues(in)
		if n != 1 {
			t.Fatalf("merged %d, want 1", n)
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\nA\nB\n\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("does not merge identical timings that are not adjacent", func(t *testing.T) {
		in := "1\n00:00:01,000 --> 00:00:02,000\nA\n\n" +
			"2\n00:00:09,000 --> 00:00:10,000\nB\n\n" +
			"3\n00:00:01,000 --> 00:00:02,000\nC\n\n"
		if _, n := MergeSplitCues(in); n != 0 {
			t.Errorf("merged %d cues, want 0", n)
		}
	})
}
