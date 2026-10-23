// Package numfmt formats numbers with Excel format codes. v0.3 covers the
// subset needed by TEXT() and the General format; the full set (colors,
// fractions, locale) arrives with cell formats in v0.4.
package numfmt

import (
	"math"
	"strconv"
	"strings"
)

// Significant digits Excel keeps when displaying and comparing numbers.
const precision = 15

// Normalize rounds n to 15 significant digits, the precision Excel
// displays. 0.1+0.2 becomes 0.3.
func Normalize(n float64) float64 {
	if n == 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return n
	}
	v, err := strconv.ParseFloat(strconv.FormatFloat(n, 'g', precision, 64), 64)
	if err != nil {
		return n
	}
	return v
}

// General formats like Excel's General format with a wide column:
// up to 15 significant digits, scientific notation for very large or
// very small magnitudes.
func General(n float64) string {
	if n == 0 {
		return "0"
	}
	v := Normalize(n)
	abs := math.Abs(v)
	if abs >= 1e15 || abs < 1e-9 {
		return scientific(v, 5)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// scientific renders 1.23457E+17 style with at most maxDecimals digits,
// trailing zeros removed.
func scientific(v float64, maxDecimals int) string {
	s := strconv.FormatFloat(v, 'E', maxDecimals, 64) // 1.23457E+17
	mant, exp, _ := strings.Cut(s, "E")
	if strings.Contains(mant, ".") {
		mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
	}
	sign := exp[0]
	digits := strings.TrimLeft(exp[1:], "0")
	if len(digits) < 2 {
		digits = strings.Repeat("0", 2-len(digits)) + digits
	}
	return mant + "E" + string(sign) + digits
}

// Round rounds half away from zero to the given number of decimals (may
// be negative), after normalizing to 15 significant digits. This is
// what makes ROUND(2.675, 2) return 2.68 as in Excel, even though
// 2.675 is stored as 2.67499999...
func Round(n float64, decimals int) float64 {
	if n == 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return n
	}
	digits, exp, neg := decompose(Normalize(n))
	// The value is 0.d1d2d3... × 10^exp. Keep exp+decimals digits.
	keep := exp + decimals
	if keep < 0 {
		return 0
	}
	if keep >= len(digits) {
		return Normalize(n)
	}
	round := digits[keep] >= '5'
	kept := []byte(digits[:keep])
	if round {
		i := len(kept) - 1
		for ; i >= 0; i-- {
			if kept[i] == '9' {
				kept[i] = '0'
				continue
			}
			kept[i]++
			break
		}
		if i < 0 {
			kept = append([]byte{'1'}, kept...)
			exp++
		}
	}
	if len(kept) == 0 {
		return 0
	}
	s := "0." + string(kept) + "e" + strconv.Itoa(exp)
	v, _ := strconv.ParseFloat(s, 64)
	if neg {
		v = -v
	}
	return v
}

// decompose returns the significant digits of |n| and the decimal
// exponent such that |n| = 0.digits × 10^exp.
func decompose(n float64) (digits string, exp int, neg bool) {
	neg = n < 0
	s := strconv.FormatFloat(math.Abs(n), 'e', precision-1, 64) // d.dddde±xx
	mant, e, _ := strings.Cut(s, "e")
	x, _ := strconv.Atoi(e)
	digits = strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	if digits == "" {
		digits = "0"
	}
	return digits, x + 1, neg
}
