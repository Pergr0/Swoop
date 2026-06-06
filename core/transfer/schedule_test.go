package transfer

import (
	"testing"

	"swoop/core/protocol"
)

func TestReservedLargeStreams(t *testing.T) {
	tests := []struct {
		streams int
		want    int
	}{
		{1, 1},
		{2, 1},
		{4, 1},
		{8, 2},
		{16, 4},
	}
	for _, tc := range tests {
		if got := reservedLargeStreams(tc.streams); got != tc.want {
			t.Errorf("reservedLargeStreams(%d) = %d, want %d", tc.streams, got, tc.want)
		}
	}
}

func TestPartitionChunks(t *testing.T) {
	chunkSize := int64(4 << 20)
	files := []protocol.FileMeta{
		{Name: "tiny.txt", Size: 100},
		{Name: "big.bin", Size: chunkSize + 1},
		{Name: "mid.bin", Size: chunkSize - 1},
	}
	chunks := buildChunks(files, chunkSize)

	large, small := partitionChunks(files, chunks, chunkSize)
	if len(small) != 2 {
		t.Fatalf("small chunks = %d, want 2 (tiny + mid)", len(small))
	}
	if len(large) != 2 {
		t.Fatalf("large chunks = %d, want 2 (two ranges of big.bin)", len(large))
	}
	for _, c := range large {
		if files[c.fileIndex].Name != "big.bin" {
			t.Fatalf("large chunk from %q, want big.bin", files[c.fileIndex].Name)
		}
	}
}
