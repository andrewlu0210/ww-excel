package printing

import (
	"bytes"
	"os"
	"testing"
	"time"

	"ww-excel/internal/model"
)

type stubSource struct{ sh *model.Sheet }

func (s stubSource) View(row, col int) (CellView, bool) {
	c := s.sh.Get(row, col)
	if !c.HasContent() {
		return CellView{}, false
	}
	st := model.Style{Font: model.Font{Size: 11}}
	if row == 0 {
		st.Font.Bold, st.Fill = true, "D9E1F2"
		st.Border.Bottom = model.BorderLine{Style: "thin"}
	}
	return CellView{Text: c.Display(), Numeric: c.Kind == model.KindNumber, Style: st}, true
}

func (s stubSource) ObjectImage(model.Object) []byte { return nil }

func sheetWith(rows, cols int) *model.Sheet {
	sh := model.NewSheet("工作表1")
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if r == 0 {
				sh.Set(r, c, model.StringCell("標題 Title"))
			} else {
				sh.Set(r, c, model.NumberCell(float64(r*c)))
			}
		}
	}
	return sh
}

func TestPaginate(t *testing.T) {
	sh := sheetWith(100, 12)
	ps := model.DefaultPageSetup()
	plan, err := Paginate(sh, ps)
	if err != nil {
		t.Fatal(err)
	}
	// A4 portrait: 0.7" margins leave ~494pt = 10 default columns
	// (48pt each); 100 rows of 15pt in ~734pt is 3 bands.
	if len(plan.Pages) != 2*3 || plan.Pages[0].Cols != [2]int{0, 9} || plan.Pages[1].Cols != [2]int{0, 9} || plan.Pages[3].Cols[0] != 10 {
		t.Fatalf("pages = %+v", plan.Pages)
	}
	ps.FitWide, ps.FitTall = 1, 0
	plan, _ = Paginate(sh, ps)
	if len(plan.Pages) != 2 || plan.Pages[0].Cols != [2]int{0, 11} || plan.Scale >= 1 {
		t.Fatalf("fit to 1 page wide: %d pages, scale %v", len(plan.Pages), plan.Scale)
	}
	ps = model.DefaultPageSetup()
	ps.PrintArea = &model.Range{R0: 0, C0: 0, R1: 4, C1: 2}
	plan, _ = Paginate(sh, ps)
	if len(plan.Pages) != 1 || plan.Pages[0].Rows != [2]int{0, 4} {
		t.Fatalf("print area: %+v", plan.Pages)
	}
	if _, err := Paginate(model.NewSheet("空"), model.DefaultPageSetup()); err != ErrNothing {
		t.Fatalf("empty sheet: %v", err)
	}
}

func TestRenderPDF(t *testing.T) {
	sh := sheetWith(60, 4)
	sh.Layout.Merges = []model.Range{{R0: 2, C0: 0, R1: 2, C1: 1}}
	ps := model.DefaultPageSetup()
	ps.Gridlines, ps.Headings, ps.HeaderText = true, true, "&A"
	plan, err := Paginate(sh, ps)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	data, err := Render(sh, plan, stubSource{sh}, Info{FileName: "test.xlsx", SheetName: "工作表1", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) || bytes.Count(data, []byte("/Type /Page\n")) < 0 {
		t.Fatalf("not a PDF")
	}
	if pages := bytes.Count(data, []byte("/Type /Page ")) + bytes.Count(data, []byte("/Type /Page\n")); pages == 0 && len(plan.Pages) == 0 {
		t.Fatal("no pages")
	}
	t.Logf("%d pages, %d KB, %v", len(plan.Pages), len(data)/1024, time.Since(start))
	if out := os.Getenv("WW_PDF_OUT"); out != "" {
		_ = os.WriteFile(out, data, 0o644)
	}
}
