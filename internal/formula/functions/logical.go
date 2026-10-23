package functions

import (
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

func init() {
	reg(catLogic, "IF", 2, 3, "logical_test, value_if_true, [value_if_false]", "依條件傳回不同的值", func(a *eval.Args) eval.Value {
		cond, err := a.Bool(0, false)
		if err.IsErr() {
			return err
		}
		if cond {
			return a.Raw(1)
		}
		if a.Len() < 3 {
			return eval.Bool(false)
		}
		return a.Raw(2)
	})
	reg(catLogic, "IFS", 2, variadic, "logical_test1, value_if_true1, ...", "傳回第一個成立條件對應的值", func(a *eval.Args) eval.Value {
		if a.Len()%2 != 0 {
			return errValue
		}
		for i := 0; i < a.Len(); i += 2 {
			cond, err := a.Bool(i, false)
			if err.IsErr() {
				return err
			}
			if cond {
				return a.Raw(i + 1)
			}
		}
		return errNA
	})
	reg(catLogic, "AND", 1, variadic, "logical1, [logical2], ...", "所有引數都為 TRUE 時傳回 TRUE", func(a *eval.Args) eval.Value {
		return logical(a, func(acc, x bool) bool { return acc && x }, true)
	})
	reg(catLogic, "OR", 1, variadic, "logical1, [logical2], ...", "任一引數為 TRUE 時傳回 TRUE", func(a *eval.Args) eval.Value {
		return logical(a, func(acc, x bool) bool { return acc || x }, false)
	})
	reg(catLogic, "XOR", 1, variadic, "logical1, [logical2], ...", "奇數個引數為 TRUE 時傳回 TRUE", func(a *eval.Args) eval.Value {
		return logical(a, func(acc, x bool) bool { return acc != x }, false)
	})
	reg(catLogic, "NOT", 1, 1, "logical", "反轉邏輯值", func(a *eval.Args) eval.Value {
		b, err := a.Bool(0, false)
		if err.IsErr() {
			return err
		}
		return eval.Bool(!b)
	})
	reg(catLogic, "IFERROR", 2, 2, "value, value_if_error", "值為錯誤時改傳回指定的值", func(a *eval.Args) eval.Value {
		if a.Scalar(0).IsErr() {
			return a.Raw(1)
		}
		return a.Raw(0)
	})
	reg(catLogic, "IFNA", 2, 2, "value, value_if_na", "值為 #N/A 時改傳回指定的值", func(a *eval.Args) eval.Value {
		if v := a.Scalar(0); v.IsErr() && v.Str == model.ErrNA {
			return a.Raw(1)
		}
		return a.Raw(0)
	})
	reg(catLogic, "SWITCH", 3, variadic, "expression, value1, result1, ..., [default]", "依運算式的值傳回對應結果", func(a *eval.Args) eval.Value {
		x := a.Scalar(0)
		if x.IsErr() {
			return x
		}
		i := 1
		for ; i+1 < a.Len(); i += 2 {
			v := a.Scalar(i)
			if v.IsErr() {
				return v
			}
			if eval.Compare(x, v) == 0 && sameClass(x, v) {
				return a.Raw(i + 1)
			}
		}
		if i < a.Len() {
			return a.Raw(i)
		}
		return errNA
	})
	reg(catLogic, "TRUE", 0, 0, "", "傳回邏輯值 TRUE", func(*eval.Args) eval.Value { return eval.Bool(true) })
	reg(catLogic, "FALSE", 0, 0, "", "傳回邏輯值 FALSE", func(*eval.Args) eval.Value { return eval.Bool(false) })
}

// logical folds AND/OR/XOR: logicals and numbers count, text inside
// ranges is ignored, and no logical values at all is #VALUE!.
func logical(a *eval.Args, fold func(acc, x bool) bool, start bool) eval.Value {
	acc, seen := start, false
	var errv eval.Value
	a.EachValue(0, func(v eval.Value, fromRef bool) bool {
		switch v.Kind {
		case eval.KError:
			errv = v
			return false
		case eval.KBool, eval.KNumber:
			acc, seen = fold(acc, v.Num != 0), true
		case eval.KString:
			if fromRef {
				return true
			}
			b, err := eval.ToBool(v)
			if err.IsErr() {
				errv = err
				return false
			}
			acc, seen = fold(acc, b), true
		}
		return true
	})
	if errv.IsErr() {
		return errv
	}
	if !seen {
		return errValue
	}
	return eval.Bool(acc)
}

// sameClass treats empty as matching anything Compare considers equal,
// but keeps numbers and text distinct.
func sameClass(a, b eval.Value) bool {
	if a.Kind == eval.KEmpty || b.Kind == eval.KEmpty {
		return true
	}
	return a.Kind == b.Kind
}
