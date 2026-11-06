package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
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
// Normal windows may query modes; all probes and outputs require a test mode.
type BenchService struct {
	bench, selfTest bool
	output          string
	busy            atomic.Bool
	emit            func(BenchEvent)
	finish          func()
	sheets          *SheetService
	docs            *DocumentService
}

func NewBenchService(sheets *SheetService, docs *DocumentService) *BenchService {
	b := &BenchService{
		sheets: sheets, docs: docs,
		bench: os.Getenv("WW_BENCH") == "1", selfTest: os.Getenv("WW_SELFTEST") == "1",
		output: benchOutPath(), finish: quitBenchApp,
	}
	b.emit = func(event BenchEvent) { application.Get().Event.Emit(BenchEventName, event) }
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
func (b *BenchService) SaveCopy(ctx context.Context, name string, allowChartRewrite bool) (string, error) {
	done, err := b.begin(ctx, true)
	if err != nil {
		return "", err
	}
	defer done()
	path, err := b.outputFile(name, "")
	if err != nil {
		return "", err
	}
	if allowChartRewrite {
		// Explicit opt-in for fixture round trips that intentionally rewrite charts.
		// Keep native compatibility dialogs in the normal save path.
		s := b.docs.at(ctx)
		func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			err = s.doc.SaveAs(path)
		}()
		if err != nil {
			return "", err
		}
		s.PublishState()
	} else if !b.docs.saveTo(ctx, path) {
		return "", fmt.Errorf("save %s failed", path)
	}
	return path, nil
}

// BenchMode reports whether the frontend should run the benchmark
// suite automatically on startup.
func (b *BenchService) BenchMode() bool {
	return b.bench
}

// SelfTestMode reports whether the frontend should run the scripted
// editing self-test (WW_SELFTEST=1) and report the results.
func (b *BenchService) SelfTestMode() bool {
	return b.selfTest
}

// Ping is an empty round trip for measuring call latency.
func (b *BenchService) Ping() (int64, error) {
	if err := b.enabled(false); err != nil {
		return 0, err
	}
	return time.Now().UnixMilli(), nil
}

// Mem returns current Go heap statistics.
func (b *BenchService) Mem(ctx context.Context) (MemStats, error) {
	done, err := b.begin(ctx, false)
	if err != nil {
		return MemStats{}, err
	}
	defer done()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemStats{
		HeapAllocMB: float64(m.HeapAlloc) / (1 << 20),
		SysMB:       float64(m.Sys) / (1 << 20),
		NumGC:       m.NumGC,
	}, ctx.Err()
}

// EmitBurst pushes count events, each carrying blocksPerEvent real
// blocks from the top of sheet 0.
func (b *BenchService) EmitBurst(ctx context.Context, count, blocksPerEvent int) error {
	done, err := b.begin(ctx, false)
	if err != nil {
		return err
	}
	defer done()
	if count < 1 || count > maxBenchEvents || blocksPerEvent < 0 || blocksPerEvent > maxBenchBlocks {
		return fmt.Errorf("invalid event count or blocks per event")
	}
	if blocksPerEvent > 0 && count > maxBenchBlockCopies/blocksPerEvent {
		return fmt.Errorf("test event block count exceeds limit")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	blocks := []BlockDTO{}
	if blocksPerEvent > 0 {
		refs := make([]BlockRef, blocksPerEvent)
		for i := range refs {
			refs[i] = BlockRef{Row: i}
		}
		blocks = b.sheets.GetBlocks(ctx, 0, refs, 0).Blocks
	}
	size, err := benchBlockBytes(blocks)
	if err != nil {
		return err
	}
	if count > maxBenchBurstBytes/size {
		return fmt.Errorf("test event burst exceeds byte limit")
	}
	if b.emit == nil {
		return fmt.Errorf("test event emitter is unavailable")
	}
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.emit(BenchEvent{Seq: i, Total: count, SentAt: time.Now().UnixMilli(), Blocks: blocks})
	}
	return nil
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
func (b *BenchService) RecalcChain(ctx context.Context, n int) (float64, error) {
	done, err := b.begin(ctx, false)
	if err != nil {
		return 0, err
	}
	defer done()
	if n < 1 || n > maxBenchChain {
		return 0, fmt.Errorf("test formula chain must contain 1..%d cells", maxBenchChain)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	wb := model.NewWorkbook()
	e := formula.New(wb)
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(1))
	for r := 1; r < n; r++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		f, err := e.Compile(fmt.Sprintf("A%d+1", r), 0)
		if err != nil {
			return 0, err
		}
		sh.Set(r, 0, model.Cell{F: f})
	}
	if err := e.RebuildContext(ctx); err != nil {
		return 0, err
	}
	start := time.Now()
	sh.Set(0, 0, model.NumberCell(2))
	e.Update([]formula.Key{{Row: 0, Col: 0}})
	return float64(time.Since(start).Microseconds()) / 1000, ctx.Err()
}

// SaveSnapshot writes a PNG data URL next to the bench report so grid
// rendering can be checked without screen-capture permissions.
func (b *BenchService) SaveSnapshot(ctx context.Context, name, dataURL string) error {
	done, err := b.begin(ctx, false)
	if err != nil {
		return err
	}
	defer done()
	path, err := b.outputFile(name, ".png")
	if err != nil {
		return err
	}
	data, err := benchPNG(dataURL)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ReportBench writes the frontend-generated markdown report and quits
// when running in bench or self-test mode.
func (b *BenchService) ReportBench(ctx context.Context, report string) error {
	done, err := b.begin(ctx, false)
	if err != nil {
		return err
	}
	defer done()
	if len(report) > maxBenchReportBytes {
		return fmt.Errorf("test report exceeds %d bytes", maxBenchReportBytes)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.WriteFile(b.output, []byte(report), 0600); err != nil {
		return fmt.Errorf("write bench report: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.finish != nil {
		b.finish()
	}
	return nil
}

// The test-only shutdown is called only after an authorized report was written.
func quitBenchApp() {
	go func() { time.Sleep(200 * time.Millisecond); application.Get().Quit() }()
	// Quit runs on the UI thread, which a runaway page can starve.
	go func() { time.Sleep(2 * time.Second); os.Exit(0) }()
}

// FixturePath selects an explicit workbook for read-only display compatibility checks.
func (b *BenchService) FixturePath() string {
	if !b.SelfTestMode() {
		return ""
	}
	return os.Getenv("WW_XLSX_FIXTURE")
}

// SavePDF exports a real PDF beside the self-test report, without a native dialog.
func (b *BenchService) SavePDF(ctx context.Context, name string, sheet int, charts map[int]string) (string, error) {
	done, err := b.begin(ctx, true)
	if err != nil {
		return "", err
	}
	defer done()
	target, err := b.outputFile(name, ".pdf")
	if err != nil {
		return "", err
	}
	if sheet < 0 || len(charts) > maxBenchChartImages {
		return "", fmt.Errorf("invalid test PDF parameters")
	}
	left := maxBenchChartURLs
	for id, url := range charts {
		if id < 0 || len(url) > left {
			return "", fmt.Errorf("test chart images exceed limit")
		}
		left -= len(url)
		if _, err = benchPNG(url); err != nil {
			return "", err
		}
	}
	data, _, err := renderPDF(b.docs.at(ctx), sheet, charts)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		return "", err
	}
	return target, nil
}
