package xlsxfile

import (
	"testing"
	"ww-excel/internal/model"
)

func TestRewrittenChartsSharingAnchor(t *testing.T) {
	chart := model.Object{ID: 1, Chart: &model.Chart{Title: "untouched"}}
	other := model.Object{ID: 2, W: 100}
	old := []model.Object{chart, other}
	if got := RewrittenCharts(old, old); len(got) != 0 {
		t.Fatalf("unchanged: %+v", got)
	}
	other.Anchor.Row = 2
	if got := RewrittenCharts(old, []model.Object{chart, other}); len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("shared anchor: %+v", got)
	}
	if got := RewrittenCharts(old, []model.Object{other}); len(got) != 0 {
		t.Fatalf("deleted chart: %+v", got)
	}
}
