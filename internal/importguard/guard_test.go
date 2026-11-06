package importguard

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func limitsContext(edit func(*Limits)) context.Context {
	l := DefaultLimits()
	edit(&l)
	return WithLimits(context.Background(), l)
}
func requireLimit(t *testing.T, err error) {
	t.Helper()
	var l *LimitError
	if !errors.As(err, &l) {
		t.Fatalf("expected limit error, got %v", err)
	}
}
func TestReaderBoundariesAndCancellation(t *testing.T) {
	for _, n := range []int{0, 7, 8, 9} {
		r := Reader(context.Background(), strings.NewReader(strings.Repeat("x", n)), 8, "bytes")
		_, err := io.ReadAll(r)
		if n <= 8 && err != nil {
			t.Fatal(err)
		}
		if n > 8 {
			requireLimit(t, err)
			_, err = r.Read(make([]byte, 16))
			requireLimit(t, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := io.ReadAll(Reader(ctx, strings.NewReader("x"), 8, "bytes"))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestJSONBudgets(t *testing.T) {
	tests := []struct {
		name, src string
		edit      func(*Limits)
		valid     bool
	}{
		{"string boundary", `{"s":"12345678"}`, func(l *Limits) { l.String = 8 }, true},
		{"string over", `{"s":"123456789"}`, func(l *Limits) { l.String = 8 }, false},
		{"escaped quote", `{"s":"a\"123456"}`, func(l *Limits) { l.String = 8 }, false},
		{"binary separate budget", `{"Data":"123456789"}`, func(l *Limits) { l.String = 8; l.Data = 8 }, true},
		{"binary over", `{"Data":"123456789"}`, func(l *Limits) { l.String = 8; l.Data = 4 }, false},
		{"depth boundary", `[[0]]`, func(l *Limits) { l.Depth = 2 }, true},
		{"depth over", `[[[0]]]`, func(l *Limits) { l.Depth = 2 }, false},
		{"records over", `[{},{},{}]`, func(l *Limits) { l.Records = 3 }, false},
		{"cells boundary", `{"sheets":[{"cells":[{},{}]}]}`, func(l *Limits) { l.Cells = 2 }, true},
		{"cells over", `{"sheets":[{"cells":[{},{},{}]}]}`, func(l *Limits) { l.Cells = 2 }, false},
		{"sheets over", `{"sheets":[{},{}]}`, func(l *Limits) { l.Sheets = 1 }, false},
		{"tokens over", `[1,2,3]`, func(l *Limits) { l.Tokens = 4 }, false},
		{"expanded over", `[1,2,3]`, func(l *Limits) { l.Expanded = 6 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := JSON(limitsContext(tt.edit), strings.NewReader(tt.src))
			if tt.valid {
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(f.Name())
				defer f.Close()
				got, _ := io.ReadAll(f)
				if string(got) != tt.src {
					t.Fatal("snapshot differs")
				}
			} else {
				requireLimit(t, err)
			}
		})
	}
	for _, src := range []string{"", `{"a":`, "{} {}", "[1,]"} {
		if f, err := JSON(context.Background(), strings.NewReader(src)); err == nil {
			f.Close()
			os.Remove(f.Name())
			t.Fatalf("accepted malformed JSON %q", src)
		}
	}
}
func makeZIP(t *testing.T, parts map[string]string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "zip")
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, s := range parts {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		io.WriteString(w, s)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
func TestZIPBudgets(t *testing.T) {
	xml := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row><c r="A1"><v>1</v></c><c r="B1"><v>2</v></c></row></sheetData></worksheet>`
	tests := []struct {
		name  string
		parts map[string]string
		edit  func(*Limits)
		valid bool
	}{
		{"valid", map[string]string{"xl/worksheets/sheet1.xml": xml}, func(*Limits) {}, true},
		{"part boundary", map[string]string{"media.bin": "12345678"}, func(l *Limits) { l.Part = 8 }, true},
		{"part over", map[string]string{"media.bin": "123456789"}, func(l *Limits) { l.Part = 8 }, false},
		{"total over", map[string]string{"a.bin": "12345", "b.bin": "67890"}, func(l *Limits) { l.Expanded = 9 }, false},
		{"parts over", map[string]string{"a.bin": "a", "b.bin": "b"}, func(l *Limits) { l.Parts = 1 }, false},
		{"cells over", map[string]string{"sheet.xml": xml}, func(l *Limits) { l.Cells = 1 }, false},
		{"depth over", map[string]string{"a.xml": "<a><b><c/></b></a>"}, func(l *Limits) { l.Depth = 2 }, false},
		{"token over", map[string]string{"a.xml": "<a>123456789</a>"}, func(l *Limits) { l.String = 8 }, false},
		{"record over", map[string]string{"a.xml": "<a><b/><c/></a>"}, func(l *Limits) { l.Records = 2 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ZIP(limitsContext(tt.edit), makeZIP(t, tt.parts))
			if tt.valid {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireLimit(t, err)
			}
		})
	}
	for _, src := range []string{"<a>", "<a></b>", "<a/><b/>", ""} {
		if err := ZIP(context.Background(), makeZIP(t, map[string]string{"a.xml": src})); err == nil {
			t.Fatalf("accepted malformed XML %q", src)
		}
	}
}
func TestZIPForgedDirectoryCount(t *testing.T) {
	f := makeZIP(t, map[string]string{"a": "1", "b": "2"})
	st, _ := f.Stat()
	var e [22]byte
	f.ReadAt(e[:], st.Size()-22)
	binary.LittleEndian.PutUint16(e[8:], 1)
	binary.LittleEndian.PutUint16(e[10:], 1)
	f.WriteAt(e[:], st.Size()-22)
	requireLimit(t, ZIP(limitsContext(func(l *Limits) { l.Parts = 1 }), f))
}
func TestZIPCancelAndSnapshotCleanup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ZIP(ctx, makeZIP(t, map[string]string{"a.xml": "<a/>"})); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := TempCopy(ctx, bytes.NewBufferString("hello"), 64, "bytes"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := JSON(context.Background(), strings.NewReader(`{"invalid"`)); err == nil {
		t.Fatal("accepted invalid JSON")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("leaked snapshots: %v", entries)
	}
}

func TestXMLTokenLimitsCannotBeResetByDelimiters(t *testing.T) {
	for _, src := range []string{
		`<a attr="` + strings.Repeat(">", 100) + `"/>`,
		`<a><![CDATA[` + strings.Repeat("<>", 100) + `]]></a>`,
		`<a><!--` + strings.Repeat("<>", 100) + `--></a>`,
		`<?xml ` + strings.Repeat(">", 100) + `?><a/>`,
	} {
		err := ZIP(limitsContext(func(l *Limits) { l.String = 32 }), makeZIP(t, map[string]string{"a.xml": src}))
		requireLimit(t, err)
	}
}
