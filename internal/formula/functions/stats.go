package functions

import (
	"math"
	"sort"

	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

func init() {
	// ---- Order statistics ----
	kth := func(name, desc string, largest bool) {
		reg(catStat, name, 2, 2, "array, k", desc, func(a *eval.Args) eval.Value {
			xs, err := numbersOf(a, 0, 1)
			if err.IsErr() {
				return err
			}
			k, err := a.Number(1, 0)
			if err.IsErr() {
				return err
			}
			i := int(math.Ceil(k))
			if len(xs) == 0 || i < 1 || i > len(xs) {
				return errNum
			}
			sort.Float64s(xs)
			if largest {
				return eval.Number(xs[len(xs)-i])
			}
			return eval.Number(xs[i-1])
		})
	}
	kth("LARGE", "傳回資料中第 k 大的值", true)
	kth("SMALL", "傳回資料中第 k 小的值", false)

	rank := func(name, desc string, avg bool) {
		reg(catStat, name, 2, 3, "number, ref, [order]", desc, func(a *eval.Args) eval.Value {
			x, err := a.Number(0, 0)
			if err.IsErr() {
				return err
			}
			if a.Raw(1).Kind != eval.KRef {
				return errValue
			}
			xs, err := numbersOf(a, 1, 2)
			if err.IsErr() {
				return err
			}
			order, err := a.Int(2, 0)
			if err.IsErr() {
				return err
			}
			better, ties := 0, 0
			for _, v := range xs {
				switch {
				case v == x:
					ties++
				case order == 0 && v > x, order != 0 && v < x:
					better++
				}
			}
			if ties == 0 {
				return errNA
			}
			if avg {
				return eval.Number(float64(better) + float64(ties+1)/2)
			}
			return eval.Number(float64(better + 1))
		})
	}
	rank("RANK.EQ", "傳回數字在清單中的排名（同分取最高名次）", false)
	rank("RANK.AVG", "傳回數字在清單中的排名（同分取平均名次）", true)
	alias("RANK", "RANK.EQ")

	percentile := func(name, desc string, exclusive bool, quart bool) {
		sig := "array, k"
		if quart {
			sig = "array, quart"
		}
		reg(catStat, name, 2, 2, sig, desc, func(a *eval.Args) eval.Value {
			xs, err := numbersOf(a, 0, 1)
			if err.IsErr() {
				return err
			}
			p, err := a.Number(1, 0)
			if err.IsErr() {
				return err
			}
			if quart {
				q := math.Floor(p)
				if q < 0 || q > 4 || exclusive && (q < 1 || q > 3) {
					return errNum
				}
				p = q / 4
			}
			v, ok := percentileOf(xs, p, exclusive)
			if !ok {
				return errNum
			}
			return eval.Number(v)
		})
	}
	percentile("PERCENTILE.INC", "傳回第 k 個百分位數（k 介於 0 到 1，含）", false, false)
	percentile("PERCENTILE.EXC", "傳回第 k 個百分位數（k 介於 0 到 1，不含）", true, false)
	percentile("QUARTILE.INC", "傳回四分位數（0 到 4）", false, true)
	percentile("QUARTILE.EXC", "傳回四分位數（1 到 3，不含端點）", true, true)
	alias("PERCENTILE", "PERCENTILE.INC")
	alias("QUARTILE", "QUARTILE.INC")

	percentRank := func(name, desc string, exclusive bool) {
		reg(catStat, name, 2, 3, "array, x, [significance]", desc, func(a *eval.Args) eval.Value {
			xs, err := numbersOf(a, 0, 1)
			if err.IsErr() {
				return err
			}
			x, err := a.Number(1, 0)
			if err.IsErr() {
				return err
			}
			digits, err := a.Int(2, 3)
			if err.IsErr() {
				return err
			}
			if len(xs) == 0 || digits < 1 {
				return errNum
			}
			sort.Float64s(xs)
			if x < xs[0] || x > xs[len(xs)-1] {
				return errNA
			}
			n := float64(len(xs))
			rankAt := func(i int) float64 { // rank of xs[i]
				less := float64(sort.SearchFloat64s(xs, xs[i]))
				if exclusive {
					return (less + 1) / (n + 1)
				}
				if n == 1 {
					return 1
				}
				return less / (n - 1)
			}
			i := sort.SearchFloat64s(xs, x)
			var r float64
			if xs[i] == x {
				r = rankAt(i)
			} else {
				lo, hi := i-1, i
				r = rankAt(lo) + (x-xs[lo])/(xs[hi]-xs[lo])*(rankAt(hi)-rankAt(lo))
			}
			scale := math.Pow(10, float64(digits))
			return eval.Number(math.Floor(r*scale+1e-9) / scale)
		})
	}
	percentRank("PERCENTRANK.INC", "傳回值在資料中的百分比排名（0 到 1，含）", false)
	percentRank("PERCENTRANK.EXC", "傳回值在資料中的百分比排名（0 到 1，不含）", true)
	alias("PERCENTRANK", "PERCENTRANK.INC")

	reg(catStat, "MODE.SNGL", 1, variadic, "number1, [number2], ...", "傳回出現最多次的值", func(a *eval.Args) eval.Value {
		xs, err := numbersOf(a, 0, a.Len())
		if err.IsErr() {
			return err
		}
		counts := map[float64]int{}
		best, bestN := 0.0, 1
		for _, x := range xs {
			counts[x]++
			if n := counts[x]; n > bestN || n == bestN && n > 1 && firstIndex(xs, x) < firstIndex(xs, best) {
				best, bestN = x, n
			}
		}
		if bestN < 2 {
			return errNA
		}
		return eval.Number(best)
	})
	alias("MODE", "MODE.SNGL")

	// ---- Dispersion ----
	variance := func(name, desc string, sample, root, includeText bool) {
		reg(catStat, name, 1, variadic, "number1, [number2], ...", desc, func(a *eval.Args) eval.Value {
			var xs []float64
			var err eval.Value
			if includeText {
				xs, err = valuesA(a)
			} else {
				xs, err = numbersOf(a, 0, a.Len())
			}
			if err.IsErr() {
				return err
			}
			v, ok := varianceOf(xs, sample)
			if !ok {
				return errDiv0
			}
			if root {
				v = math.Sqrt(v)
			}
			return eval.Number(v)
		})
	}
	variance("STDEV.S", "依樣本估算標準差", true, true, false)
	variance("STDEV.P", "依整個母體計算標準差", false, true, false)
	variance("VAR.S", "依樣本估算變異數", true, false, false)
	variance("VAR.P", "依整個母體計算變異數", false, false, false)
	variance("STDEVA", "依樣本估算標準差（文字與邏輯值也計入）", true, true, true)
	variance("STDEVPA", "依整個母體計算標準差（文字與邏輯值也計入）", false, true, true)
	variance("VARA", "依樣本估算變異數（文字與邏輯值也計入）", true, false, true)
	variance("VARPA", "依整個母體計算變異數（文字與邏輯值也計入）", false, false, true)
	alias("STDEV", "STDEV.S")
	alias("STDEVP", "STDEV.P")
	alias("VAR", "VAR.S")
	alias("VARP", "VAR.P")

	listStat := func(name, desc string, f func(xs []float64) eval.Value) {
		reg(catStat, name, 1, variadic, "number1, [number2], ...", desc, func(a *eval.Args) eval.Value {
			xs, err := numbersOf(a, 0, a.Len())
			if err.IsErr() {
				return err
			}
			return f(xs)
		})
	}
	listStat("SUMSQ", "傳回數字平方的總和", func(xs []float64) eval.Value {
		t := 0.0
		for _, x := range xs {
			t += x * x
		}
		return eval.Number(t)
	})
	listStat("GEOMEAN", "傳回幾何平均數", func(xs []float64) eval.Value {
		if len(xs) == 0 {
			return errNum
		}
		t := 0.0
		for _, x := range xs {
			if x <= 0 {
				return errNum
			}
			t += math.Log(x)
		}
		return eval.Number(math.Exp(t / float64(len(xs))))
	})
	listStat("HARMEAN", "傳回調和平均數", func(xs []float64) eval.Value {
		if len(xs) == 0 {
			return errNum
		}
		t := 0.0
		for _, x := range xs {
			if x <= 0 {
				return errNum
			}
			t += 1 / x
		}
		return eval.Number(float64(len(xs)) / t)
	})
	listStat("AVEDEV", "傳回資料與平均值的平均絕對差", func(xs []float64) eval.Value {
		if len(xs) == 0 {
			return errNum
		}
		m := sum(xs) / float64(len(xs))
		t := 0.0
		for _, x := range xs {
			t += math.Abs(x - m)
		}
		return eval.Number(t / float64(len(xs)))
	})
	listStat("DEVSQ", "傳回資料與平均值差的平方和", func(xs []float64) eval.Value {
		if len(xs) == 0 {
			return errNum
		}
		return eval.Number(devsq(xs))
	})
	listStat("KURT", "傳回峰度", func(xs []float64) eval.Value {
		n := float64(len(xs))
		s, ok := varianceOf(xs, true)
		if n < 4 || !ok || s == 0 {
			return errDiv0
		}
		m, sd := sum(xs)/n, math.Sqrt(s)
		t := 0.0
		for _, x := range xs {
			t += math.Pow((x-m)/sd, 4)
		}
		return eval.Number(n*(n+1)/((n-1)*(n-2)*(n-3))*t - 3*(n-1)*(n-1)/((n-2)*(n-3)))
	})
	listStat("SKEW", "傳回分配的偏態（樣本）", func(xs []float64) eval.Value {
		n := float64(len(xs))
		s, ok := varianceOf(xs, true)
		if n < 3 || !ok || s == 0 {
			return errDiv0
		}
		m, sd := sum(xs)/n, math.Sqrt(s)
		t := 0.0
		for _, x := range xs {
			t += math.Pow((x-m)/sd, 3)
		}
		return eval.Number(n / ((n - 1) * (n - 2)) * t)
	})
	listStat("SKEW.P", "傳回分配的偏態（母體）", func(xs []float64) eval.Value {
		n := float64(len(xs))
		s, ok := varianceOf(xs, false)
		if n < 1 || !ok || s == 0 {
			return errDiv0
		}
		m, sd := sum(xs)/n, math.Sqrt(s)
		t := 0.0
		for _, x := range xs {
			t += math.Pow((x-m)/sd, 3)
		}
		return eval.Number(t / n)
	})
	reg(catStat, "TRIMMEAN", 2, 2, "array, percent", "排除頭尾一定比例的資料後求平均", func(a *eval.Args) eval.Value {
		xs, err := numbersOf(a, 0, 1)
		if err.IsErr() {
			return err
		}
		p, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		if p < 0 || p >= 1 || len(xs) == 0 {
			return errNum
		}
		cut := int(math.Floor(float64(len(xs))*p)) / 2
		sort.Float64s(xs)
		return average(xs[cut : len(xs)-cut])
	})
	reg(catStat, "STANDARDIZE", 3, 3, "x, mean, standard_dev", "傳回標準化的值", func(a *eval.Args) eval.Value {
		x, m, sd, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if sd <= 0 {
			return errNum
		}
		return eval.Number((x - m) / sd)
	})

	// ---- A variants ----
	reg(catStat, "AVERAGEA", 1, variadic, "value1, [value2], ...", "傳回平均值（文字算 0、TRUE 算 1）", func(a *eval.Args) eval.Value {
		xs, err := valuesA(a)
		if err.IsErr() {
			return err
		}
		return average(xs)
	})
	extremeA := func(name, desc string, better func(x, y float64) bool) {
		reg(catStat, name, 1, variadic, "value1, [value2], ...", desc, func(a *eval.Args) eval.Value {
			xs, err := valuesA(a)
			if err.IsErr() {
				return err
			}
			if len(xs) == 0 {
				return eval.Number(0)
			}
			best := xs[0]
			for _, x := range xs[1:] {
				if better(x, best) {
					best = x
				}
			}
			return eval.Number(best)
		})
	}
	extremeA("MAXA", "傳回最大值（文字算 0、TRUE 算 1）", func(x, y float64) bool { return x > y })
	extremeA("MINA", "傳回最小值（文字算 0、TRUE 算 1）", func(x, y float64) bool { return x < y })
	reg(catStat, "MAXIFS", 3, variadic, "max_range, criteria_range1, criteria1, ...", "傳回符合條件的最大值", func(a *eval.Args) eval.Value {
		return multiConditional(a, 0, 1, func(vals []float64) eval.Value { return extremeOf(vals, true) })
	})
	reg(catStat, "MINIFS", 3, variadic, "min_range, criteria_range1, criteria1, ...", "傳回符合條件的最小值", func(a *eval.Args) eval.Value {
		return multiConditional(a, 0, 1, func(vals []float64) eval.Value { return extremeOf(vals, false) })
	})

	// ---- Two variables ----
	paired := func(name, sig, desc string, f func(xs, ys []float64) eval.Value) {
		reg(catStat, name, 2, 2, sig, desc, func(a *eval.Args) eval.Value {
			ys, xs, err := pairs(a, 0, 1)
			if err.IsErr() {
				return err
			}
			return f(xs, ys)
		})
	}
	// Argument order: CORREL(array1, array2) uses (array1, array2); the
	// regression functions take (known_y's, known_x's).
	paired("CORREL", "array1, array2", "傳回兩組資料的相關係數", func(ys, xs []float64) eval.Value { return correl(xs, ys) })
	paired("PEARSON", "array1, array2", "傳回皮爾森積差相關係數", func(ys, xs []float64) eval.Value { return correl(xs, ys) })
	paired("RSQ", "known_y's, known_x's", "傳回迴歸線的判定係數 R²", func(xs, ys []float64) eval.Value {
		r := correl(xs, ys)
		if r.IsErr() {
			return r
		}
		return eval.Number(r.Num * r.Num)
	})
	paired("COVARIANCE.P", "array1, array2", "傳回母體共變數", func(ys, xs []float64) eval.Value { return covariance(xs, ys, false) })
	paired("COVARIANCE.S", "array1, array2", "傳回樣本共變數", func(ys, xs []float64) eval.Value { return covariance(xs, ys, true) })
	alias("COVAR", "COVARIANCE.P")
	paired("SLOPE", "known_y's, known_x's", "傳回線性迴歸線的斜率", func(xs, ys []float64) eval.Value {
		b, _, ok := linear(xs, ys)
		if !ok {
			return errDiv0
		}
		return eval.Number(b)
	})
	paired("INTERCEPT", "known_y's, known_x's", "傳回線性迴歸線的截距", func(xs, ys []float64) eval.Value {
		_, c, ok := linear(xs, ys)
		if !ok {
			return errDiv0
		}
		return eval.Number(c)
	})
	paired("STEYX", "known_y's, known_x's", "傳回迴歸預測 y 值的標準誤差", func(xs, ys []float64) eval.Value {
		n := float64(len(xs))
		if n < 3 {
			return errDiv0
		}
		sxx, syy, sxy := moments(xs, ys)
		if sxx == 0 {
			return errDiv0
		}
		return eval.Number(math.Sqrt((syy - sxy*sxy/sxx) / (n - 2)))
	})
	reg(catStat, "FORECAST.LINEAR", 3, 3, "x, known_y's, known_x's", "以線性迴歸預測 x 對應的 y 值", func(a *eval.Args) eval.Value {
		x, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		ys, xs, err := pairs(a, 1, 2)
		if err.IsErr() {
			return err
		}
		b, c, ok := linear(xs, ys)
		if !ok {
			return errDiv0
		}
		return eval.Number(c + b*x)
	})
	alias("FORECAST", "FORECAST.LINEAR")

	// ---- Distributions ----
	reg(catStat, "NORM.DIST", 4, 4, "x, mean, standard_dev, cumulative", "傳回常態分配", func(a *eval.Args) eval.Value {
		x, m, sd, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(3, false)
		if err.IsErr() {
			return err
		}
		if sd <= 0 {
			return errNum
		}
		z := (x - m) / sd
		if cum {
			return eval.Number(normCDF(z))
		}
		return eval.Number(normPDF(z) / sd)
	})
	alias("NORMDIST", "NORM.DIST")
	reg(catStat, "NORM.S.DIST", 2, 2, "z, cumulative", "傳回標準常態分配", func(a *eval.Args) eval.Value {
		z, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(1, false)
		if err.IsErr() {
			return err
		}
		if cum {
			return eval.Number(normCDF(z))
		}
		return eval.Number(normPDF(z))
	})
	unary(catStat, "NORMSDIST", "傳回標準常態累積分配", func(z float64) eval.Value { return eval.Number(normCDF(z)) })
	reg(catStat, "NORM.INV", 3, 3, "probability, mean, standard_dev", "傳回常態累積分配的反函數", func(a *eval.Args) eval.Value {
		p, m, sd, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if p <= 0 || p >= 1 || sd <= 0 {
			return errNum
		}
		return eval.Number(m + sd*normInv(p))
	})
	alias("NORMINV", "NORM.INV")
	unary(catStat, "NORM.S.INV", "傳回標準常態累積分配的反函數", func(p float64) eval.Value {
		if p <= 0 || p >= 1 {
			return errNum
		}
		return eval.Number(normInv(p))
	})
	alias("NORMSINV", "NORM.S.INV")
	reg(catStat, "LOGNORM.DIST", 4, 4, "x, mean, standard_dev, cumulative", "傳回對數常態分配", func(a *eval.Args) eval.Value {
		x, m, sd, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(3, false)
		if err.IsErr() {
			return err
		}
		if x <= 0 || sd <= 0 {
			return errNum
		}
		z := (math.Log(x) - m) / sd
		if cum {
			return eval.Number(normCDF(z))
		}
		return eval.Number(normPDF(z) / (x * sd))
	})
	reg(catStat, "LOGNORM.INV", 3, 3, "probability, mean, standard_dev", "傳回對數常態累積分配的反函數", func(a *eval.Args) eval.Value {
		p, m, sd, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if p <= 0 || p >= 1 || sd <= 0 {
			return errNum
		}
		return eval.Number(math.Exp(m + sd*normInv(p)))
	})
	reg(catStat, "EXPON.DIST", 3, 3, "x, lambda, cumulative", "傳回指數分配", func(a *eval.Args) eval.Value {
		x, l, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(2, false)
		if err.IsErr() {
			return err
		}
		if x < 0 || l <= 0 {
			return errNum
		}
		if cum {
			return eval.Number(1 - math.Exp(-l*x))
		}
		return eval.Number(l * math.Exp(-l*x))
	})
	alias("EXPONDIST", "EXPON.DIST")
	reg(catStat, "POISSON.DIST", 3, 3, "x, mean, cumulative", "傳回卜瓦松分配", func(a *eval.Args) eval.Value {
		x, m, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(2, false)
		if err.IsErr() {
			return err
		}
		k := math.Floor(x)
		if k < 0 || m < 0 {
			return errNum
		}
		pmf := func(i float64) float64 {
			lg, _ := math.Lgamma(i + 1)
			return math.Exp(i*math.Log(m) - m - lg)
		}
		if m == 0 {
			return eval.Number(1)
		}
		if !cum {
			return eval.Number(pmf(k))
		}
		t := 0.0
		for i := 0.0; i <= k; i++ {
			t += pmf(i)
		}
		return eval.Number(math.Min(t, 1))
	})
	alias("POISSON", "POISSON.DIST")
	reg(catStat, "BINOM.DIST", 4, 4, "number_s, trials, probability_s, cumulative", "傳回二項分配", func(a *eval.Args) eval.Value {
		k, n, p, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(3, false)
		if err.IsErr() {
			return err
		}
		k, n = math.Floor(k), math.Floor(n)
		if k < 0 || k > n || p < 0 || p > 1 {
			return errNum
		}
		pmf := func(i float64) float64 { return math.Exp(lnComb(n, i)) * math.Pow(p, i) * math.Pow(1-p, n-i) }
		if !cum {
			return eval.Number(pmf(k))
		}
		t := 0.0
		for i := 0.0; i <= k; i++ {
			t += pmf(i)
		}
		return eval.Number(math.Min(t, 1))
	})
	alias("BINOMDIST", "BINOM.DIST")

	reg(catStat, "T.DIST", 3, 3, "x, deg_freedom, cumulative", "傳回 t 分配（左尾）", func(a *eval.Args) eval.Value {
		x, df, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(2, false)
		if err.IsErr() {
			return err
		}
		df = math.Floor(df)
		if df < 1 {
			return errDiv0
		}
		if cum {
			return eval.Number(tCDF(x, df))
		}
		lg1, _ := math.Lgamma((df + 1) / 2)
		lg2, _ := math.Lgamma(df / 2)
		return eval.Number(math.Exp(lg1-lg2) / math.Sqrt(df*math.Pi) * math.Pow(1+x*x/df, -(df+1)/2))
	})
	reg(catStat, "T.DIST.2T", 2, 2, "x, deg_freedom", "傳回雙尾 t 分配", func(a *eval.Args) eval.Value {
		x, df, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		df = math.Floor(df)
		if x < 0 || df < 1 {
			return errNum
		}
		return eval.Number(2 * (1 - tCDF(x, df)))
	})
	reg(catStat, "T.DIST.RT", 2, 2, "x, deg_freedom", "傳回右尾 t 分配", func(a *eval.Args) eval.Value {
		x, df, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		df = math.Floor(df)
		if df < 1 {
			return errNum
		}
		return eval.Number(1 - tCDF(x, df))
	})
	reg(catStat, "TDIST", 3, 3, "x, deg_freedom, tails", "傳回 t 分配（1 = 單尾、2 = 雙尾）", func(a *eval.Args) eval.Value {
		x, df, tails, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		df, tails = math.Floor(df), math.Floor(tails)
		if x < 0 || df < 1 || tails != 1 && tails != 2 {
			return errNum
		}
		return eval.Number(tails * (1 - tCDF(x, df)))
	})
	reg(catStat, "T.INV", 2, 2, "probability, deg_freedom", "傳回 t 分配的左尾反函數", func(a *eval.Args) eval.Value {
		p, df, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		df = math.Floor(df)
		if p <= 0 || p >= 1 || df < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return tCDF(x, df) }, p, -1e3, 1e3))
	})
	reg(catStat, "T.INV.2T", 2, 2, "probability, deg_freedom", "傳回雙尾 t 分配的反函數", func(a *eval.Args) eval.Value {
		p, df, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		df = math.Floor(df)
		if p <= 0 || p > 1 || df < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return tCDF(x, df) }, 1-p/2, 0, 1e3))
	})
	alias("TINV", "T.INV.2T")

	reg(catStat, "CHISQ.DIST", 3, 3, "x, deg_freedom, cumulative", "傳回卡方分配（左尾）", func(a *eval.Args) eval.Value {
		x, k, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(2, false)
		if err.IsErr() {
			return err
		}
		k = math.Floor(k)
		if x < 0 || k < 1 {
			return errNum
		}
		if cum {
			return eval.Number(gammaP(k/2, x/2))
		}
		lg, _ := math.Lgamma(k / 2)
		return eval.Number(math.Exp((k/2-1)*math.Log(x) - x/2 - k/2*math.Ln2 - lg))
	})
	reg(catStat, "CHISQ.DIST.RT", 2, 2, "x, deg_freedom", "傳回卡方分配的右尾機率", func(a *eval.Args) eval.Value {
		x, k, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		k = math.Floor(k)
		if x < 0 || k < 1 {
			return errNum
		}
		return eval.Number(1 - gammaP(k/2, x/2))
	})
	alias("CHIDIST", "CHISQ.DIST.RT")
	reg(catStat, "CHISQ.INV", 2, 2, "probability, deg_freedom", "傳回卡方分配左尾機率的反函數", func(a *eval.Args) eval.Value {
		p, k, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		k = math.Floor(k)
		if p < 0 || p >= 1 || k < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return gammaP(k/2, x/2) }, p, 0, 1e5))
	})
	reg(catStat, "CHISQ.INV.RT", 2, 2, "probability, deg_freedom", "傳回卡方分配右尾機率的反函數", func(a *eval.Args) eval.Value {
		p, k, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		k = math.Floor(k)
		if p <= 0 || p > 1 || k < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return gammaP(k/2, x/2) }, 1-p, 0, 1e5))
	})
	alias("CHIINV", "CHISQ.INV.RT")

	fcdf := func(x, d1, d2 float64) float64 {
		if x <= 0 {
			return 0
		}
		return betaI(d1/2, d2/2, d1*x/(d1*x+d2))
	}
	reg(catStat, "F.DIST", 4, 4, "x, deg_freedom1, deg_freedom2, cumulative", "傳回 F 分配（左尾）", func(a *eval.Args) eval.Value {
		x, d1, d2, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		cum, err := a.Bool(3, false)
		if err.IsErr() {
			return err
		}
		d1, d2 = math.Floor(d1), math.Floor(d2)
		if x < 0 || d1 < 1 || d2 < 1 {
			return errNum
		}
		if cum {
			return eval.Number(fcdf(x, d1, d2))
		}
		lb := lnBeta(d1/2, d2/2)
		return eval.Number(math.Exp((d1/2)*math.Log(d1/d2) + (d1/2-1)*math.Log(x) - ((d1+d2)/2)*math.Log(1+d1*x/d2) - lb))
	})
	reg(catStat, "F.DIST.RT", 3, 3, "x, deg_freedom1, deg_freedom2", "傳回 F 分配的右尾機率", func(a *eval.Args) eval.Value {
		x, d1, d2, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		d1, d2 = math.Floor(d1), math.Floor(d2)
		if x < 0 || d1 < 1 || d2 < 1 {
			return errNum
		}
		return eval.Number(1 - fcdf(x, d1, d2))
	})
	alias("FDIST", "F.DIST.RT")
	reg(catStat, "F.INV", 3, 3, "probability, deg_freedom1, deg_freedom2", "傳回 F 分配左尾機率的反函數", func(a *eval.Args) eval.Value {
		p, d1, d2, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		d1, d2 = math.Floor(d1), math.Floor(d2)
		if p < 0 || p >= 1 || d1 < 1 || d2 < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return fcdf(x, d1, d2) }, p, 0, 1e6))
	})
	reg(catStat, "F.INV.RT", 3, 3, "probability, deg_freedom1, deg_freedom2", "傳回 F 分配右尾機率的反函數", func(a *eval.Args) eval.Value {
		p, d1, d2, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		d1, d2 = math.Floor(d1), math.Floor(d2)
		if p <= 0 || p > 1 || d1 < 1 || d2 < 1 {
			return errNum
		}
		return eval.Number(invert(func(x float64) float64 { return fcdf(x, d1, d2) }, 1-p, 0, 1e6))
	})
	alias("FINV", "F.INV.RT")

	reg(catStat, "CONFIDENCE.NORM", 3, 3, "alpha, standard_dev, size", "使用常態分配傳回母體平均數的信賴區間半寬", func(a *eval.Args) eval.Value {
		alpha, sd, n, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		n = math.Floor(n)
		if alpha <= 0 || alpha >= 1 || sd <= 0 || n < 1 {
			return errNum
		}
		return eval.Number(normInv(1-alpha/2) * sd / math.Sqrt(n))
	})
	alias("CONFIDENCE", "CONFIDENCE.NORM")
	reg(catStat, "CONFIDENCE.T", 3, 3, "alpha, standard_dev, size", "使用 t 分配傳回母體平均數的信賴區間半寬", func(a *eval.Args) eval.Value {
		alpha, sd, n, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		n = math.Floor(n)
		if alpha <= 0 || alpha >= 1 || sd <= 0 || n < 1 {
			return errNum
		}
		if n == 1 {
			return errDiv0
		}
		t := invert(func(x float64) float64 { return tCDF(x, n-1) }, 1-alpha/2, 0, 1e3)
		return eval.Number(t * sd / math.Sqrt(n))
	})
	unary(catStat, "GAMMA", "傳回 Gamma 函數值", func(x float64) eval.Value {
		if x <= 0 && x == math.Floor(x) {
			return errNum
		}
		return eval.Number(math.Gamma(x))
	})
	unary(catStat, "GAMMALN", "傳回 Gamma 函數的自然對數", func(x float64) eval.Value {
		if x <= 0 {
			return errNum
		}
		lg, _ := math.Lgamma(x)
		return eval.Number(lg)
	})
	alias("GAMMALN.PRECISE", "GAMMALN")
	unary(catStat, "FISHER", "傳回 Fisher 轉換值", func(x float64) eval.Value {
		if x <= -1 || x >= 1 {
			return errNum
		}
		return eval.Number(math.Atanh(x))
	})
	unary(catStat, "FISHERINV", "傳回 Fisher 轉換的反函數", func(y float64) eval.Value { return eval.Number(math.Tanh(y)) })
}

// numbersOf collects the numbers of arguments [from, to) with the
// aggregate rules of SUM (text and logicals in ranges are skipped).
func numbersOf(a *eval.Args, from, to int) ([]float64, eval.Value) {
	var xs []float64
	var errv eval.Value
	for i := from; i < to && i < a.Len(); i++ {
		v := a.Raw(i)
		if v.Kind != eval.KRef && v.Kind != eval.KArray {
			v = a.Scalar(i)
		}
		walk(a, v, func(v eval.Value, fromRef bool) bool {
			switch {
			case v.Kind == eval.KError:
				errv = v
				return false
			case v.Kind == eval.KNumber:
				xs = append(xs, v.Num)
			case fromRef || v.Kind == eval.KEmpty:
			default:
				n, err := eval.ToNumber(v)
				if err.IsErr() {
					errv = err
					return false
				}
				xs = append(xs, n)
			}
			return true
		})
		if errv.IsErr() {
			return nil, errv
		}
	}
	return xs, eval.Empty
}

// walk visits a reference's non-empty cells, an array's elements or a
// scalar.
func walk(a *eval.Args, v eval.Value, fn func(v eval.Value, fromRef bool) bool) {
	switch v.Kind {
	case eval.KRef:
		cont := true
		a.Ev().R.Each(v.Ref.Sheet, v.Ref.Range(), func(_, _ int, c model.Cell) {
			if cont {
				cont = fn(eval.FromCell(c), true)
			}
		})
	case eval.KArray:
		for _, el := range v.Arr.V {
			if !fn(el, true) {
				return
			}
		}
	default:
		fn(v, false)
	}
}

// valuesA collects values for the *A functions: text in ranges counts
// as 0, logicals as 1 or 0; empty cells are skipped.
func valuesA(a *eval.Args) ([]float64, eval.Value) {
	var xs []float64
	var errv eval.Value
	a.EachValue(0, func(v eval.Value, fromRef bool) bool {
		switch v.Kind {
		case eval.KError:
			errv = v
			return false
		case eval.KNumber, eval.KBool:
			xs = append(xs, v.Num)
		case eval.KString:
			if fromRef {
				xs = append(xs, 0)
				return true
			}
			n, err := eval.ToNumber(v)
			if err.IsErr() {
				errv = err
				return false
			}
			xs = append(xs, n)
		}
		return true
	})
	return xs, errv
}

// pairs collects aligned numeric pairs from two ranges (ys from arg yi,
// xs from arg xi); positions where either is not a number are skipped.
func pairs(a *eval.Args, yi, xi int) ([]float64, []float64, eval.Value) {
	gy, err := a.Grid(yi)
	if err.IsErr() {
		return nil, nil, err
	}
	gx, err := a.Grid(xi)
	if err.IsErr() {
		return nil, nil, err
	}
	// Whole columns are clamped to the used range; compare cell counts
	// of the original references.
	if gy.Rows()*gy.Cols() != gx.Rows()*gx.Cols() {
		if !(a.Raw(yi).Kind == eval.KRef && a.Raw(xi).Kind == eval.KRef &&
			a.Raw(yi).Ref.Rows()*a.Raw(yi).Ref.Cols() == a.Raw(xi).Ref.Rows()*a.Raw(xi).Ref.Cols()) {
			return nil, nil, errNA
		}
	}
	var ys, xs []float64
	n := min(gy.Rows()*gy.Cols(), gx.Rows()*gx.Cols())
	for i := 0; i < n; i++ {
		y := gy.At(i/gy.Cols(), i%gy.Cols())
		x := gx.At(i/gx.Cols(), i%gx.Cols())
		if y.IsErr() {
			return nil, nil, y
		}
		if x.IsErr() {
			return nil, nil, x
		}
		if y.Kind == eval.KNumber && x.Kind == eval.KNumber {
			ys = append(ys, y.Num)
			xs = append(xs, x.Num)
		}
	}
	return ys, xs, eval.Empty
}

func threeNumbers(a *eval.Args) (float64, float64, float64, eval.Value) {
	x, y, err := twoNumbers(a)
	if err.IsErr() {
		return 0, 0, 0, err
	}
	z, err := a.Number(2, 0)
	return x, y, z, err
}

func firstIndex(xs []float64, x float64) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

func extremeOf(xs []float64, largest bool) eval.Value {
	if len(xs) == 0 {
		return eval.Number(0)
	}
	best := xs[0]
	for _, x := range xs[1:] {
		if largest && x > best || !largest && x < best {
			best = x
		}
	}
	return eval.Number(best)
}

func percentileOf(xs []float64, p float64, exclusive bool) (float64, bool) {
	n := len(xs)
	if n == 0 || p < 0 || p > 1 {
		return 0, false
	}
	sort.Float64s(xs)
	var rank float64 // 0-based
	if exclusive {
		rank = p*float64(n+1) - 1
		if rank < 0 || rank > float64(n-1) {
			return 0, false
		}
	} else {
		rank = p * float64(n-1)
	}
	lo := math.Floor(rank)
	i := int(lo)
	if i >= n-1 {
		return xs[n-1], true
	}
	return xs[i] + (rank-lo)*(xs[i+1]-xs[i]), true
}

func devsq(xs []float64) float64 {
	m := sum(xs) / float64(len(xs))
	t := 0.0
	for _, x := range xs {
		t += (x - m) * (x - m)
	}
	return t
}

func varianceOf(xs []float64, sample bool) (float64, bool) {
	n := len(xs)
	if n == 0 || sample && n < 2 {
		return 0, false
	}
	d := float64(n)
	if sample {
		d--
	}
	return devsq(xs) / d, true
}

// moments returns Σ(x-x̄)², Σ(y-ȳ)², Σ(x-x̄)(y-ȳ).
func moments(xs, ys []float64) (sxx, syy, sxy float64) {
	n := float64(len(xs))
	mx, my := sum(xs)/n, sum(ys)/n
	for i := range xs {
		dx, dy := xs[i]-mx, ys[i]-my
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
	}
	return
}

func correl(xs, ys []float64) eval.Value {
	if len(xs) < 2 {
		return errDiv0
	}
	sxx, syy, sxy := moments(xs, ys)
	if sxx == 0 || syy == 0 {
		return errDiv0
	}
	return eval.Number(sxy / math.Sqrt(sxx*syy))
}

func covariance(xs, ys []float64, sample bool) eval.Value {
	n := float64(len(xs))
	if n == 0 || sample && n < 2 {
		return errDiv0
	}
	_, _, sxy := moments(xs, ys)
	if sample {
		n--
	}
	return eval.Number(sxy / n)
}

// linear fits y = c + b·x by least squares.
func linear(xs, ys []float64) (b, c float64, ok bool) {
	if len(xs) == 0 {
		return 0, 0, false
	}
	sxx, _, sxy := moments(xs, ys)
	if sxx == 0 {
		return 0, 0, false
	}
	n := float64(len(xs))
	b = sxy / sxx
	return b, sum(ys)/n - b*sum(xs)/n, true
}

func normPDF(z float64) float64 { return math.Exp(-z*z/2) / math.Sqrt(2*math.Pi) }
func normCDF(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }
func normInv(p float64) float64 { return -math.Sqrt2 * math.Erfcinv(2*p) }

func lnBeta(a, b float64) float64 {
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	return la + lb - lab
}

func lnComb(n, k float64) float64 {
	a, _ := math.Lgamma(n + 1)
	b, _ := math.Lgamma(k + 1)
	c, _ := math.Lgamma(n - k + 1)
	return a - b - c
}

// betaI is the regularized incomplete beta function I_x(a, b).
func betaI(a, b, x float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	}
	front := math.Exp(a*math.Log(x) + b*math.Log(1-x) - lnBeta(a, b))
	if x < (a+1)/(a+b+2) {
		return front * betaCF(a, b, x) / a
	}
	return 1 - front*betaCF(b, a, 1-x)/b
}

// betaCF evaluates the continued fraction for betaI (Lentz's method).
func betaCF(a, b, x float64) float64 {
	const tiny = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c, d := 1.0, 1-qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 300; m++ {
		fm := float64(m)
		m2 := 2 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-15 {
			break
		}
	}
	return h
}

// gammaP is the regularized lower incomplete gamma function P(a, x).
func gammaP(a, x float64) float64 {
	if x <= 0 {
		return 0
	}
	lg, _ := math.Lgamma(a)
	if x < a+1 {
		// Series.
		sum, del, ap := 1/a, 1/a, a
		for n := 0; n < 500; n++ {
			ap++
			del *= x / ap
			sum += del
			if math.Abs(del) < math.Abs(sum)*1e-16 {
				break
			}
		}
		return sum * math.Exp(-x+a*math.Log(x)-lg)
	}
	// Continued fraction for Q, then P = 1 - Q.
	const tiny = 1e-300
	b := x + 1 - a
	c, d := 1/tiny, 1/b
	h := d
	for i := 1; i <= 500; i++ {
		an := -float64(i) * (float64(i) - a)
		b += 2
		d = an*d + b
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = b + an/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-16 {
			break
		}
	}
	return 1 - math.Exp(-x+a*math.Log(x)-lg)*h
}

func tCDF(t, df float64) float64 {
	x := df / (df + t*t)
	tail := 0.5 * betaI(df/2, 0.5, x)
	if t > 0 {
		return 1 - tail
	}
	return tail
}

// invert solves cdf(x) = p for an increasing cdf by bisection, widening
// the bracket [lo, hi] as needed.
func invert(cdf func(float64) float64, p, lo, hi float64) float64 {
	for cdf(lo) > p && lo > -1e12 {
		lo = lo*2 - 1
	}
	for cdf(hi) < p && hi < 1e12 {
		hi = hi*2 + 1
	}
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		if cdf(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
		if hi-lo < 1e-15*math.Max(1, math.Abs(mid)) {
			break
		}
	}
	return (lo + hi) / 2
}
