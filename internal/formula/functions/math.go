package functions

import (
	"math"
	"sort"

	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

func init() {
	reg(catMath, "SUM", 1, variadic, "number1, [number2], ...", "加總所有數字", func(a *eval.Args) eval.Value {
		total := 0.0
		if err := a.Numbers(0, func(n float64) { total += n }); err.IsErr() {
			return err
		}
		return eval.Number(total)
	})
	reg(catMath, "AVERAGE", 1, variadic, "number1, [number2], ...", "傳回數字的平均值", func(a *eval.Args) eval.Value {
		total, n := 0.0, 0
		if err := a.Numbers(0, func(x float64) { total += x; n++ }); err.IsErr() {
			return err
		}
		if n == 0 {
			return errDiv0
		}
		return eval.Number(total / float64(n))
	})
	reg(catMath, "MIN", 1, variadic, "number1, [number2], ...", "傳回最小值", func(a *eval.Args) eval.Value {
		return extreme(a, func(x, y float64) bool { return x < y })
	})
	reg(catMath, "MAX", 1, variadic, "number1, [number2], ...", "傳回最大值", func(a *eval.Args) eval.Value {
		return extreme(a, func(x, y float64) bool { return x > y })
	})
	reg(catMath, "MEDIAN", 1, variadic, "number1, [number2], ...", "傳回中位數", func(a *eval.Args) eval.Value {
		var xs []float64
		if err := a.Numbers(0, func(x float64) { xs = append(xs, x) }); err.IsErr() {
			return err
		}
		if len(xs) == 0 {
			return errNum
		}
		sort.Float64s(xs)
		mid := len(xs) / 2
		if len(xs)%2 == 1 {
			return eval.Number(xs[mid])
		}
		return eval.Number((xs[mid-1] + xs[mid]) / 2)
	})
	reg(catMath, "PRODUCT", 1, variadic, "number1, [number2], ...", "傳回所有數字的乘積", func(a *eval.Args) eval.Value {
		p, n := 1.0, 0
		if err := a.Numbers(0, func(x float64) { p *= x; n++ }); err.IsErr() {
			return err
		}
		if n == 0 {
			return eval.Number(0)
		}
		return eval.Number(p)
	})
	reg(catMath, "COUNT", 1, variadic, "value1, [value2], ...", "計算含有數字的儲存格個數", func(a *eval.Args) eval.Value {
		n := 0
		a.EachValue(0, func(v eval.Value, fromRef bool) bool {
			switch {
			case v.Kind == eval.KNumber:
				n++
			case !fromRef && v.Kind == eval.KBool:
				n++
			case !fromRef && v.Kind == eval.KString:
				if _, ok := model.ParseNumber(v.Str); ok {
					n++
				}
			}
			return true
		})
		return eval.Number(float64(n))
	})
	reg(catMath, "COUNTA", 1, variadic, "value1, [value2], ...", "計算非空白儲存格的個數", func(a *eval.Args) eval.Value {
		n := 0
		a.EachValue(0, func(v eval.Value, fromRef bool) bool {
			if !fromRef || v.Kind != eval.KEmpty {
				n++
			}
			return true
		})
		return eval.Number(float64(n))
	})
	reg(catMath, "COUNTBLANK", 1, 1, "range", "計算範圍中空白儲存格的個數", func(a *eval.Args) eval.Value {
		v := a.Raw(0)
		if v.Kind != eval.KRef {
			return errValue
		}
		r := v.Ref
		total := r.Rows() * r.Cols()
		a.EachValue(0, func(v eval.Value, _ bool) bool {
			if !(v.Kind == eval.KString && v.Str == "") {
				total--
			}
			return true
		})
		return eval.Number(float64(total))
	})

	reg(catMath, "ROUND", 2, 2, "number, num_digits", "將數字四捨五入到指定位數", func(a *eval.Args) eval.Value {
		n, d, err := numDigits(a)
		if err.IsErr() {
			return err
		}
		return eval.Number(numfmt.Round(n, d))
	})
	reg(catMath, "ROUNDUP", 2, 2, "number, num_digits", "將數字無條件進位（遠離 0）", func(a *eval.Args) eval.Value {
		n, d, err := numDigits(a)
		if err.IsErr() {
			return err
		}
		return eval.Number(roundWith(n, d, func(x float64) float64 {
			if x < 0 {
				return math.Floor(x)
			}
			return math.Ceil(x)
		}))
	})
	reg(catMath, "ROUNDDOWN", 2, 2, "number, num_digits", "將數字無條件捨去（趨向 0）", func(a *eval.Args) eval.Value {
		n, d, err := numDigits(a)
		if err.IsErr() {
			return err
		}
		return eval.Number(roundWith(n, d, math.Trunc))
	})
	reg(catMath, "TRUNC", 1, 2, "number, [num_digits]", "將數字截去小數", func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		d, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		return eval.Number(roundWith(n, d, math.Trunc))
	})
	unary(catMath, "INT", "將數字向下取整數", func(n float64) eval.Value { return eval.Number(math.Floor(n)) })
	unary(catMath, "ABS", "傳回絕對值", func(n float64) eval.Value { return eval.Number(math.Abs(n)) })
	unary(catMath, "SIGN", "傳回數字的正負號（1、0 或 -1）", func(n float64) eval.Value {
		switch {
		case n > 0:
			return eval.Number(1)
		case n < 0:
			return eval.Number(-1)
		}
		return eval.Number(0)
	})
	unary(catMath, "SQRT", "傳回平方根", func(n float64) eval.Value {
		if n < 0 {
			return errNum
		}
		return eval.Number(math.Sqrt(n))
	})
	unary(catMath, "EXP", "傳回 e 的 n 次方", func(n float64) eval.Value { return eval.Number(math.Exp(n)) })
	unary(catMath, "LN", "傳回自然對數", func(n float64) eval.Value {
		if n <= 0 {
			return errNum
		}
		return eval.Number(math.Log(n))
	})
	unary(catMath, "LOG10", "傳回以 10 為底的對數", func(n float64) eval.Value {
		if n <= 0 {
			return errNum
		}
		return eval.Number(math.Log10(n))
	})
	reg(catMath, "LOG", 1, 2, "number, [base]", "傳回指定底數的對數（預設 10）", func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		base, err := a.Number(1, 10)
		if err.IsErr() {
			return err
		}
		if n <= 0 || base <= 0 {
			return errNum
		}
		if base == 1 {
			return errDiv0
		}
		return eval.Number(math.Log(n) / math.Log(base))
	})
	reg(catMath, "PI", 0, 0, "", "傳回圓周率 π", func(*eval.Args) eval.Value { return eval.Number(math.Pi) })
	reg(catMath, "MOD", 2, 2, "number, divisor", "傳回相除後的餘數（正負號與除數相同）", func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		d, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		if d == 0 {
			return errDiv0
		}
		return eval.Number(numfmt.Normalize(n - d*math.Floor(n/d)))
	})
	reg(catMath, "POWER", 2, 2, "number, power", "傳回數字的乘冪", func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		p, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		if n == 0 && p == 0 {
			return errNum
		}
		if n == 0 && p < 0 {
			return errDiv0
		}
		return eval.Number(math.Pow(n, p))
	})
	reg(catMath, "CEILING", 2, 2, "number, significance", "將數字無條件進位到最接近的基數倍數", func(a *eval.Args) eval.Value {
		n, sig, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		if sig == 0 {
			return eval.Number(0)
		}
		if n > 0 && sig < 0 {
			return errNum
		}
		return eval.Number(numfmt.Normalize(math.Ceil(numfmt.Normalize(n/sig)) * sig))
	})
	reg(catMath, "FLOOR", 2, 2, "number, significance", "將數字無條件捨去到最接近的基數倍數", func(a *eval.Args) eval.Value {
		n, sig, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		if sig == 0 {
			return errDiv0
		}
		if n > 0 && sig < 0 {
			return errNum
		}
		return eval.Number(numfmt.Normalize(math.Floor(numfmt.Normalize(n/sig)) * sig))
	})
	regVolatile(catMath, "RAND", 0, 0, "", "傳回 0 到 1 之間的隨機數", func(a *eval.Args) eval.Value {
		return eval.Number(a.Ev().Rand())
	})
	regVolatile(catMath, "RANDBETWEEN", 2, 2, "bottom, top", "傳回指定範圍內的隨機整數", func(a *eval.Args) eval.Value {
		lo, hi, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		lo, hi = math.Ceil(lo), math.Floor(hi)
		if lo > hi {
			return errNum
		}
		return eval.Number(lo + math.Floor(a.Ev().Rand()*(hi-lo+1)))
	})

	reg(catMath, "SUMIF", 2, 3, "range, criteria, [sum_range]", "加總符合條件的儲存格", func(a *eval.Args) eval.Value {
		return conditional(a, 0, 1, 2, func(vals []float64) eval.Value { return eval.Number(sum(vals)) })
	})
	reg(catMath, "AVERAGEIF", 2, 3, "range, criteria, [average_range]", "傳回符合條件的儲存格的平均值", func(a *eval.Args) eval.Value {
		return conditional(a, 0, 1, 2, average)
	})
	reg(catMath, "COUNTIF", 2, 2, "range, criteria", "計算符合條件的儲存格個數", func(a *eval.Args) eval.Value {
		g, err := a.Grid(0)
		if err.IsErr() {
			return err
		}
		crit := eval.ParseCriteria(a.Scalar(1))
		n := 0
		for r := 0; r < g.Rows(); r++ {
			for c := 0; c < g.Cols(); c++ {
				if crit.Match(g.At(r, c)) {
					n++
				}
			}
		}
		return eval.Number(float64(n))
	})
	reg(catMath, "SUMIFS", 3, variadic, "sum_range, criteria_range1, criteria1, ...", "加總符合多個條件的儲存格", func(a *eval.Args) eval.Value {
		return multiConditional(a, 0, 1, func(vals []float64) eval.Value { return eval.Number(sum(vals)) })
	})
	reg(catMath, "AVERAGEIFS", 3, variadic, "average_range, criteria_range1, criteria1, ...", "傳回符合多個條件的儲存格的平均值", func(a *eval.Args) eval.Value {
		return multiConditional(a, 0, 1, average)
	})
	reg(catMath, "COUNTIFS", 2, variadic, "criteria_range1, criteria1, ...", "計算符合多個條件的儲存格個數", func(a *eval.Args) eval.Value {
		return multiConditional(a, -1, 0, func(vals []float64) eval.Value { return eval.Number(float64(len(vals))) })
	})
	eval.Register(eval.Def{
		Name: "SUMPRODUCT", Min: 1, Max: variadic, ArrayArgs: true, Category: catMath,
		Sig: "array1, [array2], ...", Desc: "傳回對應陣列元素乘積的總和",
		Fn: sumProduct,
	})
}

func sum(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

func average(xs []float64) eval.Value {
	if len(xs) == 0 {
		return errDiv0
	}
	return eval.Number(sum(xs) / float64(len(xs)))
}

func extreme(a *eval.Args, better func(x, y float64) bool) eval.Value {
	best, found := 0.0, false
	err := a.Numbers(0, func(x float64) {
		if !found || better(x, best) {
			best, found = x, true
		}
	})
	if err.IsErr() {
		return err
	}
	return eval.Number(best)
}

func numDigits(a *eval.Args) (float64, int, eval.Value) {
	n, err := a.Number(0, 0)
	if err.IsErr() {
		return 0, 0, err
	}
	d, err := a.Int(1, 0)
	return n, d, err
}

func twoNumbers(a *eval.Args) (float64, float64, eval.Value) {
	x, err := a.Number(0, 0)
	if err.IsErr() {
		return 0, 0, err
	}
	y, err := a.Number(1, 0)
	return x, y, err
}

// roundWith scales by 10^d, applies f at 15 significant digits so
// binary noise (2.675 -> 2.67499...) does not change the result.
func roundWith(n float64, d int, f func(float64) float64) float64 {
	scale := math.Pow(10, float64(d))
	return numfmt.Normalize(f(numfmt.Normalize(n*scale)) / scale)
}

// conditional implements SUMIF / AVERAGEIF: the criteria range and the
// value range are aligned at their top-left cells.
func conditional(a *eval.Args, rangeArg, critArg, valArg int, finish func([]float64) eval.Value) eval.Value {
	g, err := a.Grid(rangeArg)
	if err.IsErr() {
		return err
	}
	vals := g
	if a.Has(valArg) {
		if vals, err = a.Grid(valArg); err.IsErr() {
			return err
		}
	}
	crit := eval.ParseCriteria(a.Scalar(critArg))
	var picked []float64
	for r := 0; r < g.Rows(); r++ {
		for c := 0; c < g.Cols(); c++ {
			if !crit.Match(g.At(r, c)) {
				continue
			}
			if r >= vals.Rows() || c >= vals.Cols() {
				continue
			}
			if v := vals.At(r, c); v.Kind == eval.KNumber {
				picked = append(picked, v.Num)
			}
		}
	}
	return finish(picked)
}

// multiConditional implements the *IFS family. valArg < 0 means count
// matching positions (COUNTIFS).
func multiConditional(a *eval.Args, valArg, firstCrit int, finish func([]float64) eval.Value) eval.Value {
	if (a.Len()-firstCrit)%2 != 0 {
		return errValue
	}
	type pair struct {
		g    eval.Grid
		crit eval.Criteria
	}
	var pairs []pair
	for i := firstCrit; i < a.Len(); i += 2 {
		g, err := a.Grid(i)
		if err.IsErr() {
			return err
		}
		pairs = append(pairs, pair{g, eval.ParseCriteria(a.Scalar(i + 1))})
	}
	rows, cols := pairs[0].g.Rows(), pairs[0].g.Cols()
	var vals eval.Grid
	if valArg >= 0 {
		var err eval.Value
		if vals, err = a.Grid(valArg); err.IsErr() {
			return err
		}
		rows, cols = max(rows, vals.Rows()), max(cols, vals.Cols())
	}
	var picked []float64
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			ok := true
			for _, p := range pairs {
				v := eval.Empty
				if r < p.g.Rows() && c < p.g.Cols() {
					v = p.g.At(r, c)
				}
				if !p.crit.Match(v) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			if valArg < 0 {
				picked = append(picked, 1)
			} else if r < vals.Rows() && c < vals.Cols() {
				if v := vals.At(r, c); v.Kind == eval.KNumber {
					picked = append(picked, v.Num)
				}
			}
		}
	}
	return finish(picked)
}

func sumProduct(a *eval.Args) eval.Value {
	var arrays []eval.Value
	rows, cols := -1, -1
	for i := 0; i < a.Len(); i++ {
		v := a.Scalar(i)
		if v.IsErr() {
			return v
		}
		r, c := 1, 1
		if v.Kind == eval.KArray {
			r, c = v.Arr.Rows, v.Arr.Cols
		}
		if rows >= 0 && (r != rows || c != cols) {
			return errValue
		}
		rows, cols = r, c
		arrays = append(arrays, v)
	}
	total := 0.0
	for i := 0; i < rows*cols; i++ {
		p := 1.0
		for _, v := range arrays {
			el := v
			if v.Kind == eval.KArray {
				el = v.Arr.V[i]
			}
			switch el.Kind {
			case eval.KError:
				return el
			case eval.KNumber:
				p *= el.Num
			default:
				p = 0 // text, logicals and blanks count as 0
			}
		}
		total += p
	}
	return eval.Number(total)
}
