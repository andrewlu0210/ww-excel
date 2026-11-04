package model

import (
	"reflect"
	"testing"
)

func TestPivotClonePreservesEmptyFieldsWithoutAliasing(t *testing.T) {
	p := Pivot{Name: "P", Rows: []string{"A"}, Columns: []string{}, Values: []PivotValue{{Field: "B", Function: "sum"}}, Filters: map[string][]string{"A": {"one"}}}
	clone := p.Clone()
	if !reflect.DeepEqual(p, clone) {
		t.Fatal("cloning changed pivot metadata")
	}
	clone.Rows[0] = "changed"
	clone.Values[0].Function = "count"
	clone.Filters["A"][0] = "two"
	if p.Rows[0] != "A" || p.Values[0].Function != "sum" || p.Filters["A"][0] != "one" {
		t.Fatal("clone aliases original")
	}
	if !reflect.DeepEqual(Pivot{}, (Pivot{}).Clone()) {
		t.Fatal("empty pivot clone differs")
	}
}
