// Package functions implements worksheet functions. Importing it (even
// with a blank import) registers them with the evaluator.
package functions

import (
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

const variadic = -1

func reg(cat, name string, min, max int, sig, desc string, fn eval.Fn) {
	eval.Register(eval.Def{Name: name, Min: min, Max: max, Sig: sig, Desc: desc, Category: cat, Fn: fn})
}

func regVolatile(cat, name string, min, max int, sig, desc string, fn eval.Fn) {
	eval.Register(eval.Def{Name: name, Min: min, Max: max, Sig: sig, Desc: desc, Category: cat, Fn: fn, Volatile: true})
}

// unary registers a one-number function such as ABS.
func unary(cat, name, desc string, f func(float64) eval.Value) {
	reg(cat, name, 1, 1, "number", desc, func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		return f(n)
	})
}

var (
	errValue = eval.Err(model.ErrValue)
	errNum   = eval.Err(model.ErrNum)
	errDiv0  = eval.Err(model.ErrDiv0)
	errNA    = eval.Err(model.ErrNA)
	errRef   = eval.Err(model.ErrRef)
)

const (
	catMath   = "數學與統計"
	catLogic  = "邏輯"
	catLookup = "查閱與參照"
	catText   = "文字"
	catDate   = "日期與時間"
	catInfo   = "資訊"
)
