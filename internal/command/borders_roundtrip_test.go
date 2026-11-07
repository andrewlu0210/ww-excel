package command

import (
	"path/filepath"
	"testing"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func TestBorderXLSXRoundTrip(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Layout.Merges = []model.Range{{R0: 1, C0: 1, R1: 2, C1: 2}}
	outside := model.BorderLine{Style: "double", Color: "123456"}
	inside := model.BorderLine{Style: "mediumDashDot", Color: "ABCDEF"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 3, C1: 3}}, "框線", bold, BorderPatch{Top: &outside, Bottom: &outside, Left: &outside, Right: &outside, InsideH: &inside, InsideV: &inside})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	path := filepath.Join(t.TempDir(), "borders.xlsx")
	f, err := xlsxfile.Save(path, wb, nil, xlsxfile.Changes{})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	restored, src, err := xlsxfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	for _, rc := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {0, 1}, {4, 2}, {1, 4}} {
		if got, want := borderAt(restored, rc[0], rc[1]), borderAt(wb, rc[0], rc[1]); got != want {
			t.Errorf("%v got=%+v want=%+v", rc, got, want)
		}
	}
}
