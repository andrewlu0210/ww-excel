package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestBenchNormalModeCannotWrite(t *testing.T) {
	t.Setenv("WW_BENCH", "0")
	t.Setenv("WW_SELFTEST", "0")
	dir := t.TempDir()
	report := filepath.Join(dir, "report.md")
	t.Setenv("WW_BENCH_OUT", report)
	if err := os.WriteFile(report, []byte("keep original"), 0600); err != nil {
		t.Fatal(err)
	}
	b := NewBenchService(nil, nil)
	if err := b.SaveSnapshot(context.Background(), "forbidden", "data:image/png;base64,aGVsbG8="); err == nil {
		t.Error("normal mode accepted snapshot write")
	}
	if err := b.ReportBench(context.Background(), "forbidden"); err == nil {
		t.Error("normal mode accepted report write")
	}
	if _, err := b.SaveCopy(context.Background(), "forbidden.xlsx", true); err == nil {
		t.Error("normal mode accepted document copy")
	}
	if _, err := b.SavePDF(context.Background(), "forbidden.pdf", 0, nil); err == nil {
		t.Error("normal mode accepted PDF export")
	}
	data, err := os.ReadFile(report)
	if err != nil || string(data) != "keep original" {
		t.Errorf("report modified: %q, %v", data, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("normal mode wrote files: %v", entries)
	}
}

func TestBenchNormalModeRejectsProbesAndCannotBeEnabledByEnvironment(t *testing.T) {
	t.Setenv("WW_BENCH", "0")
	t.Setenv("WW_SELFTEST", "0")
	b := NewBenchService(nil, nil)
	t.Setenv("WW_BENCH", "1")
	t.Setenv("WW_SELFTEST", "1")
	if b.BenchMode() || b.SelfTestMode() || b.FixturePath() != "" {
		t.Fatal("mode changed after startup")
	}
	if _, err := b.Ping(); !errors.Is(err, errBenchDisabled) {
		t.Fatal(err)
	}
	if _, err := b.Mem(context.Background()); !errors.Is(err, errBenchDisabled) {
		t.Fatal(err)
	}
	if err := b.EmitBurst(context.Background(), math.MaxInt, math.MaxInt); !errors.Is(err, errBenchDisabled) {
		t.Fatal(err)
	}
	if _, err := b.RecalcChain(context.Background(), math.MaxInt); !errors.Is(err, errBenchDisabled) {
		t.Fatal(err)
	}
}

func testBench(t *testing.T, selfTest bool) *BenchService {
	t.Helper()
	return &BenchService{bench: !selfTest, selfTest: selfTest, output: filepath.Join(t.TempDir(), "report.md")}
}
func pngURL(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
func TestBenchModesAndOutput(t *testing.T) {
	for _, selfTest := range []bool{false, true} {
		t.Run(fmt.Sprint(selfTest), func(t *testing.T) {
			b := testBench(t, selfTest)
			quit := 0
			b.finish = func() { quit++ }
			if _, err := b.Ping(); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Mem(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := b.SaveSnapshot(context.Background(), "grid", pngURL(t)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(b.output), "grid.png")); err != nil {
				t.Fatal(err)
			}
			report := strings.Repeat("x", maxBenchReportBytes)
			if err := b.ReportBench(context.Background(), report); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(b.output)
			if string(data) != report || quit != 1 {
				t.Fatal("report/quit failed")
			}
			if !selfTest {
				if _, err := b.SaveCopy(context.Background(), "copy.xlsx", false); !errors.Is(err, errSelfTestOnly) {
					t.Fatal(err)
				}
				if _, err := b.SavePDF(context.Background(), "copy.pdf", 0, nil); !errors.Is(err, errSelfTestOnly) {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestBenchInvalidParametersDoNotEmitOrAllocateWork(t *testing.T) {
	b := testBench(t, false)
	emitted := 0
	b.emit = func(BenchEvent) { emitted++ }
	for _, p := range [][2]int{{-1, 1}, {0, 1}, {maxBenchEvents + 1, 1}, {math.MaxInt, math.MaxInt}, {1, -1}, {1, maxBenchBlocks + 1}, {maxBenchEvents, maxBenchBlocks}} {
		if err := b.EmitBurst(context.Background(), p[0], p[1]); err == nil {
			t.Fatalf("accepted burst %v", p)
		}
	}
	for _, n := range []int{-1, 0, maxBenchChain + 1, math.MaxInt} {
		if _, err := b.RecalcChain(context.Background(), n); err == nil {
			t.Fatalf("accepted chain %d", n)
		}
	}
	if emitted != 0 || b.busy.Load() {
		t.Fatal("invalid work emitted events or held gate")
	}
	if err := b.EmitBurst(context.Background(), maxBenchEvents, 0); err != nil || emitted != maxBenchEvents {
		t.Fatalf("boundary burst: %v, %d", err, emitted)
	}
	if _, err := b.RecalcChain(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}
func TestBenchPayloadAndImageLimits(t *testing.T) {
	b := testBench(t, false)
	b.finish = func() { t.Error("rejected report attempted quit") }
	for _, test := range []struct{ name, url string }{
		{"oversized", "data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(maxBenchSnapshotBytes)+1)},
		{"invalid base64", "data:image/png;base64,%%%"},
		{"wrong type", "data:text/plain;base64,aGVsbG8="},
		{"not a PNG", "data:image/png;base64,aGVsbG8="},
		{"../escape", pngURL(t)}, {"", pngURL(t)},
	} {
		if err := b.SaveSnapshot(context.Background(), test.name, test.url); err == nil {
			t.Fatalf("accepted %s", test.name)
		}
	}
	if err := b.ReportBench(context.Background(), strings.Repeat("x", maxBenchReportBytes+1)); err == nil {
		t.Fatal("oversized report accepted")
	}
	if entries, _ := os.ReadDir(filepath.Dir(b.output)); len(entries) != 0 {
		t.Fatalf("rejected request wrote files: %v", entries)
	}
	if _, err := benchBlockBytes([]BlockDTO{{Cells: []CellDTO{{V: strings.Repeat("x", maxBenchEventBytes)}}}}); err == nil {
		t.Fatal("oversized cell accepted")
	}
	if _, err := benchBlockBytes([]BlockDTO{{Cells: []CellDTO{{Runs: []TextRunDTO{{Text: strings.Repeat("x", maxBenchEventBytes)}}}}}}); err == nil {
		t.Fatal("oversized rich text accepted")
	}
}
func TestBenchPDFAndCopyValidateBeforeServices(t *testing.T) {
	b := testBench(t, true)
	if _, err := b.SaveCopy(context.Background(), "../escape.xlsx", true); err == nil {
		t.Fatal("unsafe copy filename")
	}
	for _, test := range []struct {
		name   string
		sheet  int
		charts map[int]string
	}{
		{"../escape.pdf", 0, nil}, {"report.pdf", -1, nil}, {"report.pdf", 0, map[int]string{1: "not a PNG"}},
	} {
		if _, err := b.SavePDF(context.Background(), test.name, test.sheet, test.charts); err == nil {
			t.Fatal("invalid PDF parameters")
		}
	}
	charts := map[int]string{}
	for i := 0; i <= maxBenchChartImages; i++ {
		charts[i] = ""
	}
	if _, err := b.SavePDF(context.Background(), "large.pdf", 0, charts); err == nil {
		t.Fatal("chart count not limited")
	}
}
func TestBenchCancellationAndConcurrentGate(t *testing.T) {
	b := testBench(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.SaveSnapshot(ctx, "grid", pngURL(t)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := b.RecalcChain(ctx, maxBenchChain); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	b.emit = func(BenchEvent) { close(entered); <-release }
	done := make(chan error, 1)
	go func() { done <- b.EmitBurst(context.Background(), 1, 0) }()
	<-entered
	if _, err := b.Mem(context.Background()); !errors.Is(err, errBenchBusy) {
		t.Fatal(err)
	}
	if err := b.SaveSnapshot(context.Background(), "busy", pngURL(t)); !errors.Is(err, errBenchBusy) {
		t.Fatal(err)
	}
	if _, err := b.RecalcChain(context.Background(), 1); !errors.Is(err, errBenchBusy) {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	emitted := 0
	b.emit = func(BenchEvent) { emitted++; cancel() }
	if err := b.EmitBurst(ctx, 10, 0); !errors.Is(err, context.Canceled) || emitted != 1 {
		t.Fatalf("burst ignored cancellation: %v, %d", err, emitted)
	}
	if _, err := b.RecalcChain(context.Background(), 1); err != nil {
		t.Fatal("gate not released", err)
	}
}

func TestBenchBurstByteBudgetAndValidBlocks(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	b := testBench(t, false)
	b.sheets = NewSheetService(s)
	emitted := 0
	b.emit = func(BenchEvent) { emitted++ }
	if err := b.EmitBurst(context.Background(), 1, maxBenchBlocks); err != nil || emitted != 1 {
		t.Fatalf("valid block boundary: %v, %d", err, emitted)
	}
	s.change(false, func(d *document.Document) (document.Change, bool) {
		for row := 0; row < 64; row++ {
			for col := 0; col < 2; col++ {
				d.Workbook().Sheets[0].Set(row, col, model.StringCell(strings.Repeat("a", 500)))
			}
		}
		return document.Change{}, false
	})
	emitted = 0
	if err := b.EmitBurst(context.Background(), maxBenchEvents, 1); err == nil || emitted != 0 {
		t.Fatalf("oversized burst emitted events: %v, %d", err, emitted)
	}
	if err := b.EmitBurst(context.Background(), 1, 1); err != nil || emitted != 1 {
		t.Fatalf("rejected burst damaged next call: %v, %d", err, emitted)
	}
}
func TestBenchOversizedPNGDimensions(t *testing.T) {
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(pngURL(t), "data:image/png;base64,"))
	binary.BigEndian.PutUint32(raw[16:20], 100_000)
	binary.BigEndian.PutUint32(raw[29:33], crc32.ChecksumIEEE(raw[12:29]))
	if _, err := benchPNG("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("oversized PNG dimensions accepted")
	}
}
func TestBenchReportFailureDoesNotQuitAndGateRecovers(t *testing.T) {
	b := testBench(t, false)
	quit := false
	b.finish = func() { quit = true }
	original := b.output
	b.output = filepath.Dir(b.output)
	if err := b.ReportBench(context.Background(), "report"); err == nil || quit {
		t.Fatal("failed report triggered quit")
	}
	b.output = original
	if err := b.ReportBench(context.Background(), "report"); err != nil || !quit {
		t.Fatal("report retry failed", err)
	}
}

func TestBenchSelfTestCopyAndPDFRemainAvailable(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	b := testBench(t, true)
	b.docs = NewDocumentService(s, &fakeDialogs{})
	if _, err := NewSheetService(s).SetCell(context.Background(), 0, 0, 0, "test output"); err != nil {
		t.Fatal(err)
	}
	path, err := b.SaveCopy(context.Background(), "copy.xlsx", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	path, err = b.SavePDF(context.Background(), "copy", 0, map[int]string{1: pngURL(t)})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("valid PDF failed: %v", err)
	}
}

func TestBenchGateReleasesAfterEmitterPanic(t *testing.T) {
	b := testBench(t, false)
	b.emit = func(BenchEvent) { panic("injected emitter failure") }
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected injected panic")
			}
		}()
		_ = b.EmitBurst(context.Background(), 1, 0)
	}()
	b.emit = func(BenchEvent) {}
	if err := b.EmitBurst(context.Background(), 1, 0); err != nil {
		t.Fatal("failed emitter left the probe gate locked", err)
	}
}
