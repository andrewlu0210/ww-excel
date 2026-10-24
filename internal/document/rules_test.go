package document

import (
	"path/filepath"
	"reflect"
	"testing"

	"ww-excel/internal/model"
)

func TestDataValidation(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 7, "蘋果")
	enter(t, d, 0, 1, 7, "香蕉")
	list := model.Validation{Type: "list", Formula1: "$H$1:$H$2", Error: "請從清單選擇"}
	must(t, d)(d.SetValidationCommand(0, []model.Range{rg(0, 0, 9, 0)}, &list))
	if got := d.ListValues(0, 3, 0, plain); !reflect.DeepEqual(got, []string{"蘋果", "香蕉"}) {
		t.Fatalf("list = %v", got)
	}
	if err := d.Validate(0, 3, 0, model.StringCell("香蕉"), plain); err != nil {
		t.Fatalf("valid item refused: %v", err)
	}
	if err := d.Validate(0, 3, 0, model.StringCell("西瓜"), plain); err == nil || !err.Stop || err.Message != "請從清單選擇" {
		t.Fatalf("invalid item: %+v", err)
	}
	whole := model.Validation{Type: "whole", Operator: "between", Formula1: "1", Formula2: "10", ErrorStyle: "warning"}
	must(t, d)(d.SetValidationCommand(0, []model.Range{rg(0, 1, 9, 1)}, &whole))
	if d.Validate(0, 0, 1, model.NumberCell(5), plain) != nil || d.Validate(0, 0, 1, model.NumberCell(5.5), plain) == nil {
		t.Fatal("whole number rule")
	}
	if err := d.Validate(0, 0, 1, model.NumberCell(11), plain); err == nil || err.Stop {
		t.Fatalf("warning style: %+v", err)
	}
	inline := model.Validation{Type: "list", Formula1: `"是,否"`}
	if got := d.listValues(&inline, 0, 0, 0, plain); !reflect.DeepEqual(got, []string{"是", "否"}) {
		t.Fatalf("inline list = %v", got)
	}
	custom := model.Validation{Type: "custom", Formula1: "C1>B1"}
	must(t, d)(d.SetValidationCommand(0, []model.Range{rg(0, 2, 9, 2)}, &custom))
	enter(t, d, 0, 2, 1, "4")
	if d.Validate(0, 2, 2, model.NumberCell(5), plain) != nil || d.Validate(0, 2, 2, model.NumberCell(3), plain) == nil {
		t.Fatal("custom rule is relative to the cell")
	}
	// Clearing part of a range splits the rule.
	must(t, d)(d.SetValidationCommand(0, []model.Range{rg(3, 0, 3, 0)}, nil))
	if d.ValidationAt(0, 3, 0) != nil || d.ValidationAt(0, 4, 0) == nil {
		t.Fatal("clear part of a rule")
	}
	// Inserting rows moves rules and their references.
	shift(t, d, 0, true, 0, 1)
	if v := d.ValidationAt(0, 1, 0); v == nil || v.Formula1 != "$H$2:$H$3" {
		t.Fatalf("after insert: %+v", v)
	}
	must(t, d)(d.SetValidationCommand(0, []model.Range{rg(0, 4, 9, 4)}, &inline))
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "dv"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		v := back.ValidationAt(0, 1, 0)
		if v == nil || v.Type != "list" || v.Formula1 != "$H$2:$H$3" || v.Error != "請從清單選擇" {
			t.Fatalf("%s: %+v", ext, v)
		}
		if got := back.ListValues(0, 1, 4, plain); !reflect.DeepEqual(got, []string{"是", "否"}) {
			t.Fatalf("%s inline list = %v", ext, got)
		}
		if w := back.ValidationAt(0, 1, 1); w == nil || w.Formula2 != "10" || w.ErrorStyle != "warning" {
			t.Fatalf("%s whole: %+v", ext, w)
		}
	}
}

func TestConditionalFormats(t *testing.T) {
	d := New()
	for r, v := range []string{"10", "50", "30", "50", "x"} {
		enter(t, d, 0, r, 0, v)
	}
	red := model.CFFormat{Color: "9C0006", Fill: "FFC7CE"}
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{rg(0, 0, 4, 0)}, model.CFRule{Type: "cell", Op: "greaterThan", Value1: "25", Format: red}))
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{rg(0, 0, 4, 0)}, model.CFRule{Type: "duplicate", Format: model.CFFormat{Bold: true}}))
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{rg(0, 1, 4, 1)}, model.CFRule{Type: "formula", Value1: "$A1>=30", Format: model.CFFormat{Fill: "00FF00"}}))
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{rg(0, 2, 4, 2)}, model.CFRule{Type: "3_color_scale"}))
	for r, v := range []string{"1", "2", "3"} {
		enter(t, d, 0, r, 2, v)
	}
	ev := d.CondFormats(0, plain)
	sh := d.Workbook().Sheets[0]
	at := func(r, c int) (CFResult, bool) { return ev.At(r, c, sh.Get(r, c)) }
	if res, ok := at(1, 0); !ok || res.Format.Fill != "FFC7CE" || !res.Format.Bold {
		t.Fatalf("A2 = %+v %v", res, ok)
	}
	if res, ok := at(0, 0); ok {
		t.Fatalf("A1 should not match: %+v", res)
	}
	if res, ok := at(2, 1); !ok || res.Format.Fill != "00FF00" {
		t.Fatalf("formula rule on blank B3 = %+v %v", res, ok)
	}
	if res, _ := at(0, 2); res.ScaleFill != "F8696B" {
		t.Fatalf("scale min = %+v", res)
	}
	if res, _ := at(2, 2); res.ScaleFill != "63BE7B" {
		t.Fatalf("scale max = %+v", res)
	}
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "cf"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		rules := back.Workbook().Sheets[0].Layout.CondFormats
		if len(rules) != 4 {
			t.Fatalf("%s: %d rules: %+v", ext, len(rules), rules)
		}
		bev := back.CondFormats(0, plain)
		bsh := back.Workbook().Sheets[0]
		if res, ok := bev.At(1, 0, bsh.Get(1, 0)); !ok || res.Format.Fill != "FFC7CE" || res.Format.Color != "9C0006" {
			t.Fatalf("%s A2 = %+v %v", ext, res, ok)
		}
		if res, ok := bev.At(2, 1, bsh.Get(2, 1)); !ok || res.Format.Fill != "00FF00" {
			t.Fatalf("%s formula rule = %+v %v", ext, res, ok)
		}
	}
}
