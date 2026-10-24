package wwx

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"ww-excel/internal/model"
)

func TestRoundTrip(t *testing.T) {
	wb := model.NewWorkbook()
	s := wb.Sheets[0]
	s.Set(0, 0, model.StringCell("42")) // text that looks like a number
	s.Set(0, 1, model.NumberCell(42))
	s.Set(model.MaxRows-1, model.MaxCols-1, model.StringCell("台北\n多行"))
	s.Set(1, 0, model.BoolCell(false))
	s.Set(1, 1, model.ErrorCell(model.ErrNA))
	f := model.NumberCell(84)
	f.F = &model.Formula{Text: "B1*2"}
	f.S = wb.StyleTable().Intern(model.Style{Font: model.Font{Name: "新細明體", Size: 12, Bold: true}, Fill: "FFFF00", NumFmt: "#,##0"})
	s.Set(1, 2, f)
	s.Layout.Cols[2] = model.Size{Px: 150}
	s.Layout.Rows[0] = model.Size{Px: 30, Hidden: true}
	s.Layout.ColStyles[4] = f.S
	s.Layout.Merges = []model.Range{{R0: 5, C0: 0, R1: 6, C1: 3}}
	wb.Sheets = append(wb.Sheets, model.NewSheet("第二頁"))

	var buf bytes.Buffer
	if err := Write(&buf, wb); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	gs := got.Sheets[0]
	if gs.Name != "工作表1" {
		t.Errorf("name = %q", gs.Name)
	}
	for _, rc := range [][2]int{{0, 0}, {0, 1}, {model.MaxRows - 1, model.MaxCols - 1}, {1, 0}, {1, 1}, {1, 2}} {
		if a, b := gs.Get(rc[0], rc[1]), s.Get(rc[0], rc[1]); !a.SameInput(b) || !a.SameValue(b) || a.S != b.S {
			t.Errorf("(%d,%d) = %+v, want %+v", rc[0], rc[1], a, b)
		}
	}
	st := got.Styles.Get(gs.Get(1, 2).S)
	if !st.Font.Bold || st.Fill != "FFFF00" || st.NumFmt != "#,##0" || st.Font.Name != "新細明體" {
		t.Errorf("style = %+v", st)
	}
	l := gs.Layout
	if l.ColWidth(2) != 150 || l.RowHeight(0) != 0 || l.Rows[0].Px != 30 || l.ColStyles[4] != f.S || len(l.Merges) != 1 {
		t.Errorf("layout = %+v", l)
	}
	if len(got.Sheets) != 2 || got.Sheets[1].Name != "第二頁" {
		t.Errorf("sheets = %d", len(got.Sheets))
	}
}

func TestReadVersion1(t *testing.T) {
	wb, err := Read(gz(`{"format":"ww-excel","version":1,"sheets":[{"name":"a","cells":[{"r":0,"c":0,"n":5}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if wb.Sheets[0].Get(0, 0).Num != 5 || wb.StyleTable().Len() != 1 {
		t.Fatal("v1 file")
	}
}

func gz(s string) *bytes.Buffer {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(s))
	zw.Close()
	return &buf
}

func TestReadRejects(t *testing.T) {
	tests := map[string]*bytes.Buffer{
		"not gzip":     bytes.NewBufferString("hello"),
		"wrong format": gz(`{"format":"other","version":1,"sheets":[{"name":"a","cells":[]}]}`),
		"newer":        gz(`{"format":"ww-excel","version":99,"sheets":[{"name":"a","cells":[]}]}`),
		"no sheets":    gz(`{"format":"ww-excel","version":1,"sheets":[]}`),
		"out of range": gz(`{"format":"ww-excel","version":1,"sheets":[{"name":"a","cells":[{"r":-1,"c":0,"n":1}]}]}`),
	}
	for name, in := range tests {
		if _, err := Read(in); err == nil {
			t.Errorf("%s: expected error", name)
		} else if !strings.Contains(err.Error(), "wwx") && !strings.Contains(err.Error(), "range") {
			t.Errorf("%s: unhelpful error %v", name, err)
		}
	}
}
