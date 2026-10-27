package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ww-excel/internal/formula"
	"ww-excel/internal/model"
)

// BenchEventName carries synthetic block payloads for measuring event
// push throughput from Go to the frontend.
const BenchEventName = "bench:block"

type BenchEvent struct {
	Seq    int        `json:"seq"`
	Total  int        `json:"total"`
	SentAt int64      `json:"sentAt"` // unix ms
	Blocks []BlockDTO `json:"blocks"`
}

type MemStats struct {
	HeapAllocMB float64 `json:"heapAllocMB"`
	SysMB       float64 `json:"sysMB"`
	NumGC       uint32  `json:"numGC"`
}

// BenchService exposes IPC probes used by the v0.1 performance harness.
// Bench mode is enabled with WW_BENCH=1; results are written to
// WW_BENCH_OUT (default docs/perf-latest.md) and the app quits.
type BenchService struct {
	sheets *SheetService
	docs   *DocumentService
}

func NewBenchService(sheets *SheetService, docs *DocumentService) *BenchService {
	b := &BenchService{sheets: sheets, docs: docs}
	if b.BenchMode() || b.SelfTestMode() {
		// A broken run must never leave the app open.
		go func() {
			time.Sleep(3 * time.Minute)
			fmt.Fprintln(os.Stderr, "bench/self-test watchdog: exiting")
			os.Exit(2)
		}()
	}
	return b
}

// SaveCopy saves the document as name next to the self-test report,
// without dialogs, and returns the full path (self-test only).
func (b *BenchService) SaveCopy(ctx context.Context, name string) (string, error) {
	if !b.SelfTestMode() {
		return "", fmt.Errorf("SaveCopy is only available in self-test mode")
	}
	path, err := filepath.Abs(filepath.Join(filepath.Dir(benchOutPath()), filepath.Base(name)))
	if err != nil {
		return "", err
	}
	if !b.docs.saveTo(ctx, path) {
		return "", fmt.Errorf("save %s failed", path)
	}
	return path, nil
}

// BenchMode reports whether the frontend should run the benchmark
// suite automatically on startup.
func (b *BenchService) BenchMode() bool {
	return os.Getenv("WW_BENCH") == "1"
}

// SelfTestMode reports whether the frontend should run the scripted
// editing self-test (WW_SELFTEST=1) and report the results.
func (b *BenchService) SelfTestMode() bool {
	return os.Getenv("WW_SELFTEST") == "1"
}

// Ping is an empty round trip for measuring call latency.
func (b *BenchService) Ping() int64 {
	return time.Now().UnixMilli()
}

// Mem returns current Go heap statistics.
func (b *BenchService) Mem() MemStats {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemStats{
		HeapAllocMB: float64(m.HeapAlloc) / (1 << 20),
		SysMB:       float64(m.Sys) / (1 << 20),
		NumGC:       m.NumGC,
	}
}

// EmitBurst pushes count events, each carrying blocksPerEvent real
// blocks from the top of sheet 0.
func (b *BenchService) EmitBurst(ctx context.Context, count, blocksPerEvent int) {
	refs := make([]BlockRef, blocksPerEvent)
	for i := range refs {
		refs[i] = BlockRef{Row: i}
	}
	blocks := b.sheets.GetBlocks(ctx, 0, refs, 0).Blocks

	app := application.Get()
	for i := 0; i < count; i++ {
		app.Event.Emit(BenchEventName, BenchEvent{
			Seq: i, Total: count, SentAt: time.Now().UnixMilli(), Blocks: blocks,
		})
	}
}

func benchOutPath() string {
	if out := os.Getenv("WW_BENCH_OUT"); out != "" {
		return out
	}
	return "docs/perf-latest.md"
}

// RecalcChain builds a separate workbook with a chain of n formulas
// (A2=A1+1, A3=A2+1, ...) and returns how long one full recalculation
// after changing A1 takes, in milliseconds.
func (b *BenchService) RecalcChain(n int) float64 {
	wb := model.NewWorkbook()
	e := formula.New(wb)
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(1))
	for r := 1; r < n; r++ {
		f, err := e.Compile(fmt.Sprintf("A%d+1", r), 0)
		if err != nil {
			return -1
		}
		sh.Set(r, 0, model.Cell{F: f})
	}
	e.Rebuild()
	start := time.Now()
	sh.Set(0, 0, model.NumberCell(2))
	e.Update([]formula.Key{{Row: 0, Col: 0}})
	return float64(time.Since(start).Microseconds()) / 1000
}

// SaveSnapshot writes a PNG data URL next to the bench report so grid
// rendering can be checked without screen-capture permissions.
func (b *BenchService) SaveSnapshot(name, dataURL string) error {
	_, payload, ok := strings.Cut(dataURL, ",")
	if !ok {
		return fmt.Errorf("snapshot %q: not a data URL", name)
	}
	png, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("snapshot %q: %w", name, err)
	}
	path := filepath.Join(filepath.Dir(benchOutPath()), filepath.Base(name)+".png")
	return os.WriteFile(path, png, 0o644)
}

// ReportBench writes the frontend-generated markdown report and quits
// when running in bench or self-test mode.
func (b *BenchService) ReportBench(report string) error {
	if err := os.WriteFile(benchOutPath(), []byte(report), 0o644); err != nil {
		return fmt.Errorf("write bench report: %w", err)
	}
	if b.BenchMode() || b.SelfTestMode() {
		go func() {
			time.Sleep(200 * time.Millisecond)
			application.Get().Quit()
		}()
		// Quit runs on the UI thread, which a runaway page can starve.
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}
	return nil
}

// FixturePath selects an explicit workbook for read-only display compatibility checks.
func (b *BenchService) FixturePath() string {
	if !b.SelfTestMode() {
		return ""
	}
	return os.Getenv("WW_XLSX_FIXTURE")
}
