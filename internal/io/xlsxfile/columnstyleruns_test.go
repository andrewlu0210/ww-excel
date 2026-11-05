package xlsxfile

import (
	"path/filepath"
	"reflect"
	"testing"
	"ww-excel/internal/model"
)

func TestColumnStyleRunsIncludeRemovedColumnsAndGaps(t *testing.T) {
	got := columnStyleRuns(map[int]uint32{5: 3}, map[int]uint32{0: 1, 1: 1, 3: 1, 4: 2})
	want := []columnStyleRun{{0, 1, 1}, {3, 3, 1}, {4, 4, 2}, {5, 5, 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
}
func TestFullWidthColumnStylesKeepExplicitCellFormats(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	st := wb.StyleTable()
	style := st.Default()
	style.Font.Bold = true
	bold := st.InternExplicit(style)
	style.Font.Bold = false
	style.Font.Color = "FF0000"
	red := st.InternExplicit(style)
	for col := 0; col < model.MaxCols; col++ {
		sh.Layout.ColStyles[col] = bold
	}
	sh.Layout.ColStyles[511] = red
	if len(columnStyleRuns(nil, sh.Layout.ColStyles)) != 3 {
		t.Fatal("full-width styles were not coalesced")
	}
	cell := model.NumberCell(42)
	cell.S = red
	sh.Set(0, 0, cell)
	filename := filepath.Join(t.TempDir(), "columns.xlsx")
	saved, err := Save(filename, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	back, base, err := Read(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	sh = back.Sheets[0]
	for _, col := range []int{0, 510, 512, model.MaxCols - 1} {
		if !back.StyleTable().Get(sh.Layout.ColStyles[col]).Font.Bold {
			t.Fatalf("lost column style at %d", col)
		}
	}
	if back.StyleTable().Get(sh.Get(0, 0).S).Font.Color != "FF0000" || back.StyleTable().Get(sh.Layout.ColStyles[511]).Font.Color != "FF0000" {
		t.Fatal("lost exception or explicit cell style")
	}
}
