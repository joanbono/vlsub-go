package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// chunkSize is fixed by the OpenSubtitles hash specification.
const chunkSize = 65536

// ErrFileTooSmall is returned for files that cannot be hashed, since the
// algorithm reads a 64 KiB chunk from each end without overlapping.
var ErrFileTooSmall = errors.New("file is smaller than 128 KiB, too small to hash")

// HashFile returns the OpenSubtitles "moviehash" of path as a 16-digit hex
// string, along with the file size.
//
// The hash is the 64-bit wrapping sum of the file size and every
// little-endian uint64 in the first and last 64 KiB of the file. It matches a
// specific release rather than a title, which is why it finds subtitles that
// are already in sync. This is the same identifier vlsub sends.
func HashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	if st.IsDir() {
		return "", 0, fmt.Errorf("%s is a directory", path)
	}

	sum, err := hashAt(f, st.Size())
	if err != nil {
		return "", st.Size(), err
	}
	return fmt.Sprintf("%016x", sum), st.Size(), nil
}

func hashAt(r io.ReaderAt, size int64) (uint64, error) {
	if size < chunkSize*2 {
		return 0, ErrFileTooSmall
	}

	sum := uint64(size)
	buf := make([]byte, chunkSize)

	for _, off := range [2]int64{0, size - chunkSize} {
		if _, err := io.ReadFull(io.NewSectionReader(r, off, chunkSize), buf); err != nil {
			return 0, fmt.Errorf("reading chunk at offset %d: %w", off, err)
		}
		for i := 0; i < chunkSize; i += 8 {
			sum += binary.LittleEndian.Uint64(buf[i:])
		}
	}
	return sum, nil
}
