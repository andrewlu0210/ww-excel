package formula

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"ww-excel/internal/model"
)

func TestImportRebuildCancellationAndHealthyEdits(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		t.Run(fmt.Sprintf("parallel=%v", parallel), func(t *testing.T) {
			wb := model.NewWorkbook()
			count := 1
			if parallel {
				count = 1024
			}
			for row := 0; row < count; row++ {
				wb.Sheets[0].Set(row, 0, model.Cell{F: &model.Formula{Text: "RAND()"}})
			}
			engine := New(wb)
			ctx, cancel := context.WithCancel(context.Background())
			var once sync.Once
			engine.Rand = func() float64 { once.Do(cancel); return 0.5 }
			if err := engine.RebuildContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation not propagated: %v", err)
			}
			// Import-only accounting must not leak into later engine use.
			engine.Rand = func() float64 { return 0.25 }
			engine.Rebuild()
			if got := wb.Sheets[0].Get(0, 0); got.Kind != model.KindNumber || got.Num != 0.25 {
				t.Fatalf("engine remains cancelled: %+v", got)
			}
		})
	}
}
