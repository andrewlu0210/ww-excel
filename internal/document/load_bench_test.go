package document

import (
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"ww-excel/internal/recovery"
)

// TestLoadBigFile measures opening WW_BIG_XLSX (see tools/bigfile).
func TestLoadBigFile(t *testing.T) {
	path := os.Getenv("WW_BIG_XLSX")
	if path == "" {
		t.Skip("set WW_BIG_XLSX")
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	d, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if out := os.Getenv("WW_HEAP_OUT"); out != "" {
		f, _ := os.Create(out)
		_ = pprof.WriteHeapProfile(f)
		f.Close()
	}
	rows, cols := d.Workbook().Sheets[0].UsedRange()
	t.Logf("%d×%d loaded in %v, heap %.1f MB", rows, cols, elapsed, float64(after.HeapAlloc-before.HeapAlloc)/(1<<20))

	// An autosave snapshot of the same workbook.
	start = time.Now()
	data, err := recovery.Encode(d.Workbook())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("autosave snapshot in %v, %.1f MB", time.Since(start), float64(len(data))/(1<<20))
	runtime.KeepAlive(d)
}
