package wwx

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"ww-excel/internal/importguard"
)

func compressed(t *testing.T, src string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	io.WriteString(z, src)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestExpansionBombRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	// Stream a highly compressible 8 MiB cell, without allocating that cell.
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	io.WriteString(z, `{"format":"ww-excel","sheets":[{"cells":[{"c":0,"r":0,"s":"`)
	chunk := strings.Repeat("A", 8192)
	for i := 0; i < 1024; i++ {
		io.WriteString(z, chunk)
	}
	io.WriteString(z, "\"}],\"name\":\"Probe\"}],\"version\":2}\n")
	z.Close()
	t.Logf("compressed %d bytes, cell expands to 8388608 bytes", b.Len())
	wb, err := Read(bytes.NewReader(b.Bytes()))
	var limit *importguard.LimitError
	if wb != nil || !errors.As(err, &limit) {
		t.Fatalf("accepted expansion: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}
func TestReadLimitsCancellationAndChecksum(t *testing.T) {
	base := `{"format":"ww-excel","version":2,"sheets":[{"name":"a","cells":[{"r":0,"c":0,"s":"hello"}]}]}`
	for _, edit := range []func(*importguard.Limits){func(l *importguard.Limits) { l.Compressed = 8 }, func(l *importguard.Limits) { l.Expanded = 32 }, func(l *importguard.Limits) { l.Cells = 0 }, func(l *importguard.Limits) { l.Sheets = 0 }} {
		l := importguard.DefaultLimits()
		edit(&l)
		wb, err := ReadContext(importguard.WithLimits(context.Background(), l), bytes.NewReader(compressed(t, base)))
		var limit *importguard.LimitError
		if wb != nil || !errors.As(err, &limit) {
			t.Fatalf("limit not enforced: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadContext(ctx, bytes.NewReader(compressed(t, base))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	corrupt := compressed(t, base)
	corrupt[len(corrupt)-8] ^= 1
	if _, err := Read(bytes.NewReader(corrupt)); err == nil {
		t.Fatal("accepted invalid gzip checksum")
	}
	if _, err := Read(bytes.NewReader(compressed(t, base+"{}"))); err == nil {
		t.Fatal("accepted trailing document")
	}
}
