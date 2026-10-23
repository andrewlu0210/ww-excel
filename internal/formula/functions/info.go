package functions

import (
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

func init() {
	is := func(name, desc string, test func(v eval.Value) bool) {
		reg(catInfo, name, 1, 1, "value", desc, func(a *eval.Args) eval.Value {
			return eval.Bool(test(a.Scalar(0)))
		})
	}
	is("ISBLANK", "值為空白儲存格時傳回 TRUE", func(v eval.Value) bool { return v.Kind == eval.KEmpty })
	is("ISNUMBER", "值為數字時傳回 TRUE", func(v eval.Value) bool { return v.Kind == eval.KNumber })
	is("ISTEXT", "值為文字時傳回 TRUE", func(v eval.Value) bool { return v.Kind == eval.KString })
	is("ISNONTEXT", "值不是文字時傳回 TRUE", func(v eval.Value) bool { return v.Kind != eval.KString })
	is("ISLOGICAL", "值為邏輯值時傳回 TRUE", func(v eval.Value) bool { return v.Kind == eval.KBool })
	is("ISERROR", "值為任何錯誤值時傳回 TRUE", func(v eval.Value) bool { return v.IsErr() })
	is("ISERR", "值為 #N/A 以外的錯誤值時傳回 TRUE", func(v eval.Value) bool { return v.IsErr() && v.Str != model.ErrNA })
	is("ISNA", "值為 #N/A 時傳回 TRUE", func(v eval.Value) bool { return v.IsErr() && v.Str == model.ErrNA })
	reg(catInfo, "NA", 0, 0, "", "傳回錯誤值 #N/A", func(*eval.Args) eval.Value { return errNA })
}
