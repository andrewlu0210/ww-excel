package importguard

import (
	"context"
	"strings"
	"testing"

	"ww-excel/internal/model"
)

func TestWorkbookResources(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*model.Workbook)
	}{
		{"cell text", func(wb *model.Workbook) { wb.Sheets[0].Set(0, 0, model.StringCell(strings.Repeat("a", 32768))) }},
		{"formula", func(wb *model.Workbook) {
			wb.Sheets[0].Set(0, 0, model.Cell{F: &model.Formula{Text: strings.Repeat("1+", 2100) + "1"}})
		}},
		{"huge object", func(wb *model.Workbook) { wb.Sheets[0].Layout.Objects = []model.Object{{W: 1e15, H: 10}} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wb := model.NewWorkbook()
			tt.mutate(wb)
			if err := Workbook(context.Background(), wb); err == nil {
				t.Fatal("resource limit bypassed")
			}
		})
	}
	wb := model.NewWorkbook()
	wb.Sheets[0].Set(0, 0, model.StringCell(strings.Repeat("字", 32767)))
	if err := Workbook(context.Background(), wb); err != nil {
		t.Fatal(err)
	}
}

type cancelReader struct {
	r      *strings.Reader
	cancel context.CancelFunc
}

func (r *cancelReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p[:min(len(p), 16)])
	r.cancel()
	return n, e
}
func TestCancellationDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &cancelReader{r: strings.NewReader(strings.Repeat("x", 100)), cancel: cancel}
	if f, err := TempCopy(ctx, r, 200, "bytes"); err == nil {
		f.Close()
		t.Fatal("mid-copy cancellation ignored")
	}
	ctx, cancel = context.WithCancel(context.Background())
	r = &cancelReader{r: strings.NewReader(`{"x":"` + strings.Repeat("a", 1000) + `"}`), cancel: cancel}
	if f, err := JSON(ctx, r); err == nil {
		f.Close()
		t.Fatal("mid-JSON cancellation ignored")
	}
}
