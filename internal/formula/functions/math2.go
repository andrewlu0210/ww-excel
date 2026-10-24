package functions

import (
	"math"
	"strconv"
	"strings"

	"ww-excel/internal/formula/eval"
	"ww-excel/internal/numfmt"
)

func init() {
	trig := func(name, desc string, f func(float64) float64, ok func(float64) bool) {
		unary(catMath, name, desc, func(x float64) eval.Value {
			if ok != nil && !ok(x) {
				return errNum
			}
			return eval.Number(f(x))
		})
	}
	unit := func(x float64) bool { return x >= -1 && x <= 1 }
	trig("SIN", "傳回角度（弧度）的正弦值", math.Sin, nil)
	trig("COS", "傳回角度（弧度）的餘弦值", math.Cos, nil)
	trig("TAN", "傳回角度（弧度）的正切值", math.Tan, nil)
	trig("ASIN", "傳回反正弦值（弧度）", math.Asin, unit)
	trig("ACOS", "傳回反餘弦值（弧度）", math.Acos, unit)
	trig("ATAN", "傳回反正切值（弧度）", math.Atan, nil)
	trig("SINH", "傳回雙曲正弦值", math.Sinh, nil)
	trig("COSH", "傳回雙曲餘弦值", math.Cosh, nil)
	trig("TANH", "傳回雙曲正切值", math.Tanh, nil)
	trig("ASINH", "傳回反雙曲正弦值", math.Asinh, nil)
	trig("ACOSH", "傳回反雙曲餘弦值", math.Acosh, func(x float64) bool { return x >= 1 })
	trig("ATANH", "傳回反雙曲正切值", math.Atanh, func(x float64) bool { return x > -1 && x < 1 })
	recip := func(name, desc string, f func(float64) float64) {
		unary(catMath, name, desc, func(x float64) eval.Value {
			v := f(x)
			if v == 0 {
				return errDiv0
			}
			return eval.Number(1 / v)
		})
	}
	recip("COT", "傳回角度的餘切值", math.Tan)
	recip("CSC", "傳回角度的餘割值", math.Sin)
	recip("SEC", "傳回角度的正割值", math.Cos)
	trig("RADIANS", "將角度轉換為弧度", func(x float64) float64 { return x * math.Pi / 180 }, nil)
	trig("DEGREES", "將弧度轉換為角度", func(x float64) float64 { return x * 180 / math.Pi }, nil)
	trig("SQRTPI", "傳回 (數字 × π) 的平方根", func(x float64) float64 { return math.Sqrt(x * math.Pi) }, func(x float64) bool { return x >= 0 })
	reg(catMath, "ATAN2", 2, 2, "x_num, y_num", "傳回 x、y 座標的反正切值（弧度）", func(a *eval.Args) eval.Value {
		x, y, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		if x == 0 && y == 0 {
			return errDiv0
		}
		return eval.Number(math.Atan2(y, x))
	})

	unary(catMath, "EVEN", "將數字遠離 0 進位到最接近的偶數", func(x float64) eval.Value {
		return eval.Number(awayToMultiple(x, 2, 0))
	})
	unary(catMath, "ODD", "將數字遠離 0 進位到最接近的奇數", func(x float64) eval.Value {
		return eval.Number(awayToMultiple(x, 2, 1))
	})
	unary(catMath, "FACT", "傳回階乘", func(x float64) eval.Value {
		n := math.Floor(x)
		if n < 0 {
			return errNum
		}
		return eval.Number(math.Round(math.Gamma(n + 1)))
	})
	unary(catMath, "FACTDOUBLE", "傳回雙階乘（n!!）", func(x float64) eval.Value {
		n := math.Floor(x)
		if n < -1 {
			return errNum
		}
		p := 1.0
		for i := n; i > 1; i -= 2 {
			p *= i
		}
		return eval.Number(p)
	})
	combo := func(name, desc string, f func(n, k float64) float64) {
		reg(catMath, name, 2, 2, "number, number_chosen", desc, func(a *eval.Args) eval.Value {
			n, k, err := twoNumbers(a)
			if err.IsErr() {
				return err
			}
			n, k = math.Floor(n), math.Floor(k)
			if n < 0 || k < 0 || name != "COMBINA" && name != "PERMUTATIONA" && k > n {
				return errNum
			}
			return eval.Number(math.Round(f(n, k)))
		})
	}
	combo("COMBIN", "傳回組合數", func(n, k float64) float64 { return math.Exp(lnComb(n, k)) })
	combo("COMBINA", "傳回可重複的組合數", func(n, k float64) float64 {
		if n == 0 && k == 0 {
			return 1
		}
		return math.Exp(lnComb(n+k-1, k))
	})
	combo("PERMUT", "傳回排列數", func(n, k float64) float64 {
		a, _ := math.Lgamma(n + 1)
		b, _ := math.Lgamma(n - k + 1)
		return math.Exp(a - b)
	})
	combo("PERMUTATIONA", "傳回可重複的排列數", func(n, k float64) float64 { return math.Pow(n, k) })
	intList := func(name, desc string, f func(a, b float64) float64, start float64) {
		reg(catMath, name, 1, variadic, "number1, [number2], ...", desc, func(a *eval.Args) eval.Value {
			xs, err := numbersOf(a, 0, a.Len())
			if err.IsErr() {
				return err
			}
			acc := start
			for _, x := range xs {
				x = math.Floor(x)
				if x < 0 {
					return errNum
				}
				acc = f(acc, x)
			}
			return eval.Number(acc)
		})
	}
	gcd := func(a, b float64) float64 {
		for b != 0 {
			a, b = b, math.Mod(a, b)
		}
		return a
	}
	intList("GCD", "傳回最大公因數", gcd, 0)
	intList("LCM", "傳回最小公倍數", func(a, b float64) float64 {
		if a == 0 || b == 0 {
			return 0
		}
		return a / gcd(a, b) * b
	}, 1)
	reg(catMath, "MULTINOMIAL", 1, variadic, "number1, [number2], ...", "傳回多項式係數", func(a *eval.Args) eval.Value {
		xs, err := numbersOf(a, 0, a.Len())
		if err.IsErr() {
			return err
		}
		total, lg := 0.0, 0.0
		for _, x := range xs {
			x = math.Floor(x)
			if x < 0 {
				return errNum
			}
			total += x
			l, _ := math.Lgamma(x + 1)
			lg += l
		}
		t, _ := math.Lgamma(total + 1)
		return eval.Number(math.Round(math.Exp(t - lg)))
	})
	reg(catMath, "QUOTIENT", 2, 2, "numerator, denominator", "傳回除法的整數部分", func(a *eval.Args) eval.Value {
		n, d, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		if d == 0 {
			return errDiv0
		}
		return eval.Number(math.Trunc(numfmt.Normalize(n / d)))
	})
	reg(catMath, "MROUND", 2, 2, "number, multiple", "傳回四捨五入到指定倍數的數字", func(a *eval.Args) eval.Value {
		n, m, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		if m == 0 {
			return eval.Number(0)
		}
		if n*m < 0 {
			return errNum
		}
		return eval.Number(numfmt.Normalize(math.Round(numfmt.Normalize(n/m)) * m))
	})
	reg(catMath, "CEILING.MATH", 1, 3, "number, [significance], [mode]", "將數字進位到最接近的整數或基數倍數", func(a *eval.Args) eval.Value {
		return ceilFloorMath(a, true)
	})
	reg(catMath, "FLOOR.MATH", 1, 3, "number, [significance], [mode]", "將數字捨去到最接近的整數或基數倍數", func(a *eval.Args) eval.Value {
		return ceilFloorMath(a, false)
	})
	precise := func(name, desc string, up bool) {
		reg(catMath, name, 1, 2, "number, [significance]", desc, func(a *eval.Args) eval.Value {
			n, err := a.Number(0, 0)
			if err.IsErr() {
				return err
			}
			s, err := a.Number(1, 1)
			if err.IsErr() {
				return err
			}
			s = math.Abs(s)
			if s == 0 {
				return eval.Number(0)
			}
			f := math.Floor
			if up {
				f = math.Ceil
			}
			return eval.Number(numfmt.Normalize(f(numfmt.Normalize(n/s)) * s))
		})
	}
	precise("CEILING.PRECISE", "將數字進位（不論正負號）到最接近的基數倍數", true)
	precise("FLOOR.PRECISE", "將數字捨去（不論正負號）到最接近的基數倍數", false)
	alias("ISO.CEILING", "CEILING.PRECISE")

	sumPairs := func(name, desc string, f func(x, y float64) float64) {
		reg(catMath, name, 2, 2, "array_x, array_y", desc, func(a *eval.Args) eval.Value {
			gx, err := a.Grid(0)
			if err.IsErr() {
				return err
			}
			gy, err := a.Grid(1)
			if err.IsErr() {
				return err
			}
			if gx.Rows()*gx.Cols() != gy.Rows()*gy.Cols() {
				return errNA
			}
			t := 0.0
			for i := 0; i < gx.Rows()*gx.Cols(); i++ {
				x, y := gx.At(i/gx.Cols(), i%gx.Cols()), gy.At(i/gy.Cols(), i%gy.Cols())
				if x.IsErr() {
					return x
				}
				if y.IsErr() {
					return y
				}
				if x.Kind == eval.KNumber && y.Kind == eval.KNumber {
					t += f(x.Num, y.Num)
				}
			}
			return eval.Number(t)
		})
	}
	sumPairs("SUMX2MY2", "傳回兩陣列對應值平方差的總和", func(x, y float64) float64 { return x*x - y*y })
	sumPairs("SUMX2PY2", "傳回兩陣列對應值平方和的總和", func(x, y float64) float64 { return x*x + y*y })
	sumPairs("SUMXMY2", "傳回兩陣列對應值差的平方和", func(x, y float64) float64 { return (x - y) * (x - y) })

	reg(catMath, "SUBTOTAL", 2, variadic, "function_num, ref1, [ref2], ...", "傳回清單或資料庫的小計", func(a *eval.Args) eval.Value {
		fn, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		if fn > 100 {
			fn -= 100
		}
		if fn == 2 || fn == 3 {
			n := 0
			a.EachValue(1, func(v eval.Value, fromRef bool) bool {
				if fn == 2 && v.Kind == eval.KNumber || fn == 3 && (v.Kind != eval.KEmpty || !fromRef) {
					n++
				}
				return true
			})
			return eval.Number(float64(n))
		}
		xs, err := numbersOf(a, 1, a.Len())
		if err.IsErr() {
			return err
		}
		switch fn {
		case 1:
			return average(xs)
		case 4:
			return extremeOf(xs, true)
		case 5:
			return extremeOf(xs, false)
		case 6:
			p := 1.0
			for _, x := range xs {
				p *= x
			}
			return eval.Number(p)
		case 7, 8, 10, 11:
			v, ok := varianceOf(xs, fn == 7 || fn == 10)
			if !ok {
				return errDiv0
			}
			if fn <= 8 {
				v = math.Sqrt(v)
			}
			return eval.Number(v)
		case 9:
			return eval.Number(sum(xs))
		}
		return errValue
	})

	// ---- Number bases ----
	reg(catMath, "BASE", 2, 3, "number, radix, [min_length]", "將數字轉換為指定基數的文字", func(a *eval.Args) eval.Value {
		n, radix, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		width, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		n, radix = math.Floor(n), math.Floor(radix)
		if n < 0 || n >= 1<<53 || radix < 2 || radix > 36 || width < 0 {
			return errNum
		}
		return eval.Text(pad(strings.ToUpper(strconv.FormatInt(int64(n), int(radix))), width))
	})
	reg(catMath, "DECIMAL", 2, 2, "text, radix", "將指定基數的文字轉換為十進位數字", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		radix, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		if radix < 2 || radix > 36 {
			return errNum
		}
		v, perr := strconv.ParseInt(strings.TrimSpace(s), radix, 64)
		if perr != nil {
			return errNum
		}
		return eval.Number(float64(v))
	})
	baseConv := func(from, to int, name, desc string, bits int) {
		reg(catMath, name, 1, 2, "number, [places]", desc, func(a *eval.Args) eval.Value {
			places, err := a.Int(1, 0)
			if err.IsErr() {
				return err
			}
			var n int64
			if from == 10 {
				x, err := a.Number(0, 0)
				if err.IsErr() {
					return err
				}
				n = int64(math.Trunc(x))
			} else {
				s, err := a.Text(0, "")
				if err.IsErr() {
					return err
				}
				if len(s) > 10 {
					return errNum
				}
				u, perr := strconv.ParseUint(s, from, 64)
				if perr != nil {
					return errNum
				}
				// Ten digits in the source base use two's complement.
				full := uint64(1) << (bitsPer(from) * 10)
				if len(s) == 10 && u >= full/2 {
					n = int64(u) - int64(full)
				} else {
					n = int64(u)
				}
			}
			if to == 10 {
				return eval.Number(float64(n))
			}
			limit := int64(1) << (bitsPer(to)*10 - 1)
			if n < -limit || n >= limit {
				return errNum
			}
			if n < 0 {
				u := uint64(n) & (uint64(1)<<(bitsPer(to)*10) - 1)
				return eval.Text(strings.ToUpper(strconv.FormatUint(u, to)))
			}
			s := strings.ToUpper(strconv.FormatInt(n, to))
			if a.Has(1) {
				if places < len(s) || places > 10 {
					return errNum
				}
				s = pad(s, places)
			}
			return eval.Text(s)
		})
	}
	baseConv(10, 2, "DEC2BIN", "將十進位數字轉換為二進位", 0)
	baseConv(10, 8, "DEC2OCT", "將十進位數字轉換為八進位", 0)
	baseConv(10, 16, "DEC2HEX", "將十進位數字轉換為十六進位", 0)
	baseConv(2, 10, "BIN2DEC", "將二進位數字轉換為十進位", 0)
	baseConv(8, 10, "OCT2DEC", "將八進位數字轉換為十進位", 0)
	baseConv(16, 10, "HEX2DEC", "將十六進位數字轉換為十進位", 0)
	baseConv(2, 16, "BIN2HEX", "將二進位數字轉換為十六進位", 0)
	baseConv(16, 2, "HEX2BIN", "將十六進位數字轉換為二進位", 0)

	reg(catMath, "ROMAN", 1, 2, "number, [form]", "將阿拉伯數字轉換為羅馬數字", func(a *eval.Args) eval.Value {
		n, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		v := int(math.Trunc(n))
		if v < 0 || v > 3999 {
			return errValue
		}
		return eval.Text(roman(v))
	})
	reg(catMath, "ARABIC", 1, 1, "text", "將羅馬數字轉換為阿拉伯數字", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		v, ok := arabic(strings.ToUpper(strings.TrimSpace(s)))
		if !ok {
			return errValue
		}
		return eval.Number(float64(v))
	})
}

func bitsPer(base int) uint {
	switch base {
	case 2:
		return 1
	case 8:
		return 3
	}
	return 4
}

func pad(s string, width int) string {
	if len(s) < width {
		return strings.Repeat("0", width-len(s)) + s
	}
	return s
}

// awayToMultiple rounds x away from zero to the next value v with
// v mod m == r (EVEN: m=2 r=0; ODD: m=2 r=1).
func awayToMultiple(x, m, r float64) float64 {
	sign := 1.0
	if x < 0 {
		sign, x = -1, -x
	}
	v := math.Ceil(x)
	for math.Mod(v, m) != r {
		v++
	}
	return sign * v
}

func ceilFloorMath(a *eval.Args, up bool) eval.Value {
	n, err := a.Number(0, 0)
	if err.IsErr() {
		return err
	}
	s, err := a.Number(1, 1)
	if err.IsErr() {
		return err
	}
	mode, err := a.Number(2, 0)
	if err.IsErr() {
		return err
	}
	s = math.Abs(s)
	if s == 0 {
		return eval.Number(0)
	}
	q := numfmt.Normalize(n / s)
	// For negative numbers a non-zero mode rounds away from zero.
	toward := up
	if n < 0 && mode != 0 {
		toward = !up
	}
	if toward {
		q = math.Ceil(q)
	} else {
		q = math.Floor(q)
	}
	return eval.Number(numfmt.Normalize(q * s))
}

var romanTable = []struct {
	v int
	s string
}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}}

func roman(n int) string {
	var b strings.Builder
	for _, r := range romanTable {
		for n >= r.v {
			b.WriteString(r.s)
			n -= r.v
		}
	}
	return b.String()
}

func arabic(s string) (int, bool) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	vals := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	total, prev := 0, 0
	rs := []rune(s)
	for i := len(rs) - 1; i >= 0; i-- {
		v, ok := vals[rs[i]]
		if !ok {
			return 0, false
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	if neg {
		total = -total
	}
	return total, true
}
