package functions

import (
	"math"

	"ww-excel/internal/formula/eval"
)

func init() {
	// Time value of money. type 1 = payments at the start of a period.
	reg(catFin, "PMT", 3, 5, "rate, nper, pv, [fv], [type]", "傳回貸款的每期付款金額", func(a *eval.Args) eval.Value {
		r, n, pv, fv, typ, err := tvmArgs(a, 3)
		if err.IsErr() {
			return err
		}
		if n == 0 {
			return errNum
		}
		return eval.Number(pmt(r, n, pv, fv, typ))
	})
	reg(catFin, "FV", 3, 5, "rate, nper, pmt, [pv], [type]", "傳回投資的未來值", func(a *eval.Args) eval.Value {
		r, n, p, pv, typ, err := tvmArgs(a, 3)
		if err.IsErr() {
			return err
		}
		return eval.Number(fv(r, n, p, pv, typ))
	})
	reg(catFin, "PV", 3, 5, "rate, nper, pmt, [fv], [type]", "傳回投資的現值", func(a *eval.Args) eval.Value {
		r, n, p, f, typ, err := tvmArgs(a, 3)
		if err.IsErr() {
			return err
		}
		if r == 0 {
			return eval.Number(-f - p*n)
		}
		g := math.Pow(1+r, n)
		return eval.Number((-f - p*(1+r*typ)*(g-1)/r) / g)
	})
	reg(catFin, "NPER", 3, 5, "rate, pmt, pv, [fv], [type]", "傳回投資的期數", func(a *eval.Args) eval.Value {
		r, p, pv, f, typ, err := tvmArgs(a, 3)
		if err.IsErr() {
			return err
		}
		if r == 0 {
			if p == 0 {
				return errNum
			}
			return eval.Number(-(pv + f) / p)
		}
		num := p*(1+r*typ) - f*r
		den := pv*r + p*(1+r*typ)
		if num/den <= 0 {
			return errNum
		}
		return eval.Number(math.Log(num/den) / math.Log(1+r))
	})
	reg(catFin, "RATE", 3, 6, "nper, pmt, pv, [fv], [type], [guess]", "傳回年金每期的利率", func(a *eval.Args) eval.Value {
		n, p, pv, f, typ, err := tvmArgs(a, 3)
		if err.IsErr() {
			return err
		}
		guess, err := a.Number(5, 0.1)
		if err.IsErr() {
			return err
		}
		r, ok := solve(func(r float64) float64 {
			if math.Abs(r) < 1e-12 {
				return pv + p*n + f
			}
			g := math.Pow(1+r, n)
			return pv*g + p*(1+r*typ)*(g-1)/r + f
		}, guess)
		if !ok {
			return errNum
		}
		return eval.Number(r)
	})
	ipmtFn := func(a *eval.Args, principal bool) eval.Value {
		r, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		per, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		n, err := a.Number(2, 0)
		if err.IsErr() {
			return err
		}
		pv, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		f, err := a.Number(4, 0)
		if err.IsErr() {
			return err
		}
		typ, err := a.Number(5, 0)
		if err.IsErr() {
			return err
		}
		if typ != 0 {
			typ = 1
		}
		if per < 1 || per > n {
			return errNum
		}
		p := pmt(r, n, pv, f, typ)
		ip := interestPart(r, per, p, pv, typ)
		if principal {
			return eval.Number(p - ip)
		}
		return eval.Number(ip)
	}
	reg(catFin, "IPMT", 4, 6, "rate, per, nper, pv, [fv], [type]", "傳回某一期付款中的利息", func(a *eval.Args) eval.Value { return ipmtFn(a, false) })
	reg(catFin, "PPMT", 4, 6, "rate, per, nper, pv, [fv], [type]", "傳回某一期付款中的本金", func(a *eval.Args) eval.Value { return ipmtFn(a, true) })
	cumFn := func(a *eval.Args, principal bool) eval.Value {
		r, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		n, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		pv, err := a.Number(2, 0)
		if err.IsErr() {
			return err
		}
		start, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		end, err := a.Number(4, 0)
		if err.IsErr() {
			return err
		}
		typ, err := a.Number(5, 0)
		if err.IsErr() {
			return err
		}
		start, end = math.Ceil(start), math.Floor(end)
		if r <= 0 || n <= 0 || pv <= 0 || start < 1 || end < start || end > n || typ != 0 && typ != 1 {
			return errNum
		}
		p := pmt(r, n, pv, 0, typ)
		t := 0.0
		for per := start; per <= end; per++ {
			ip := interestPart(r, per, p, pv, typ)
			if principal {
				t += p - ip
			} else {
				t += ip
			}
		}
		return eval.Number(t)
	}
	reg(catFin, "CUMIPMT", 6, 6, "rate, nper, pv, start_period, end_period, type", "傳回兩期之間累計支付的利息", func(a *eval.Args) eval.Value { return cumFn(a, false) })
	reg(catFin, "CUMPRINC", 6, 6, "rate, nper, pv, start_period, end_period, type", "傳回兩期之間累計償還的本金", func(a *eval.Args) eval.Value { return cumFn(a, true) })
	reg(catFin, "ISPMT", 4, 4, "rate, per, nper, pv", "傳回直線償還本金時某一期的利息", func(a *eval.Args) eval.Value {
		r, per, n, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		pv, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		if n == 0 {
			return errDiv0
		}
		return eval.Number(pv * r * (per/n - 1))
	})

	reg(catFin, "NPV", 2, variadic, "rate, value1, [value2], ...", "傳回一系列現金流量的淨現值", func(a *eval.Args) eval.Value {
		r, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		xs, err := numbersOf(a, 1, a.Len())
		if err.IsErr() {
			return err
		}
		if r == -1 {
			return errDiv0
		}
		t := 0.0
		for i, x := range xs {
			t += x / math.Pow(1+r, float64(i+1))
		}
		return eval.Number(t)
	})
	reg(catFin, "IRR", 1, 2, "values, [guess]", "傳回一系列現金流量的內部報酬率", func(a *eval.Args) eval.Value {
		xs, err := numbersOf(a, 0, 1)
		if err.IsErr() {
			return err
		}
		guess, err := a.Number(1, 0.1)
		if err.IsErr() {
			return err
		}
		if !hasSignChange(xs) {
			return errNum
		}
		r, ok := solve(func(r float64) float64 {
			t := 0.0
			for i, x := range xs {
				t += x / math.Pow(1+r, float64(i))
			}
			return t
		}, guess)
		if !ok {
			return errNum
		}
		return eval.Number(r)
	})
	reg(catFin, "MIRR", 3, 3, "values, finance_rate, reinvest_rate", "傳回修正的內部報酬率", func(a *eval.Args) eval.Value {
		xs, err := numbersOf(a, 0, 1)
		if err.IsErr() {
			return err
		}
		fr, err := a.Number(1, 0)
		if err.IsErr() {
			return err
		}
		rr, err := a.Number(2, 0)
		if err.IsErr() {
			return err
		}
		n := float64(len(xs))
		var pos, neg float64
		for i, x := range xs {
			if x > 0 {
				pos += x * math.Pow(1+rr, n-1-float64(i))
			} else {
				neg += x / math.Pow(1+fr, float64(i))
			}
		}
		if pos == 0 || neg == 0 || n < 2 {
			return errDiv0
		}
		return eval.Number(math.Pow(-pos/neg, 1/(n-1)) - 1)
	})
	xflows := func(a *eval.Args, vi, di int) ([]float64, []float64, eval.Value) {
		vals, err := numbersOf(a, vi, vi+1)
		if err.IsErr() {
			return nil, nil, err
		}
		dates, err := numbersOf(a, di, di+1)
		if err.IsErr() {
			return nil, nil, err
		}
		if len(vals) != len(dates) || len(vals) == 0 {
			return nil, nil, errNum
		}
		for i := range dates {
			dates[i] = math.Floor(dates[i])
			if dates[i] < dates[0] {
				return nil, nil, errNum
			}
		}
		return vals, dates, eval.Empty
	}
	xnpv := func(r float64, vals, dates []float64) float64 {
		t := 0.0
		for i, v := range vals {
			t += v / math.Pow(1+r, (dates[i]-dates[0])/365)
		}
		return t
	}
	reg(catFin, "XNPV", 3, 3, "rate, values, dates", "傳回不定期現金流量的淨現值", func(a *eval.Args) eval.Value {
		r, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		vals, dates, err := xflows(a, 1, 2)
		if err.IsErr() {
			return err
		}
		return eval.Number(xnpv(r, vals, dates))
	})
	reg(catFin, "XIRR", 2, 3, "values, dates, [guess]", "傳回不定期現金流量的內部報酬率", func(a *eval.Args) eval.Value {
		vals, dates, err := xflows(a, 0, 1)
		if err.IsErr() {
			return err
		}
		guess, err := a.Number(2, 0.1)
		if err.IsErr() {
			return err
		}
		if !hasSignChange(vals) {
			return errNum
		}
		r, ok := solve(func(r float64) float64 { return xnpv(r, vals, dates) }, guess)
		if !ok {
			return errNum
		}
		return eval.Number(r)
	})

	// ---- Depreciation ----
	reg(catFin, "SLN", 3, 3, "cost, salvage, life", "傳回直線法的每期折舊", func(a *eval.Args) eval.Value {
		cost, salvage, life, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if life == 0 {
			return errDiv0
		}
		return eval.Number((cost - salvage) / life)
	})
	reg(catFin, "SYD", 4, 4, "cost, salvage, life, per", "傳回年數合計法的折舊", func(a *eval.Args) eval.Value {
		cost, salvage, life, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		per, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		if life <= 0 || per <= 0 || per > life {
			return errNum
		}
		return eval.Number((cost - salvage) * (life - per + 1) * 2 / (life * (life + 1)))
	})
	reg(catFin, "DDB", 4, 5, "cost, salvage, life, period, [factor]", "傳回倍數遞減法的折舊", func(a *eval.Args) eval.Value {
		cost, salvage, life, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		period, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		factor, err := a.Number(4, 2)
		if err.IsErr() {
			return err
		}
		if cost < 0 || salvage < 0 || life <= 0 || period <= 0 || period > life || factor <= 0 {
			return errNum
		}
		book, dep := cost, 0.0
		for p := 1.0; p <= math.Ceil(period); p++ {
			dep = math.Min(book*factor/life, math.Max(0, book-salvage))
			book -= dep
		}
		return eval.Number(dep)
	})
	reg(catFin, "DB", 4, 5, "cost, salvage, life, period, [month]", "傳回定率遞減法的折舊", func(a *eval.Args) eval.Value {
		cost, salvage, life, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		period, err := a.Number(3, 0)
		if err.IsErr() {
			return err
		}
		month, err := a.Number(4, 12)
		if err.IsErr() {
			return err
		}
		if cost < 0 || salvage < 0 || life <= 0 || period <= 0 || month < 1 || month > 12 || period > life+1 {
			return errNum
		}
		if cost == 0 {
			return eval.Number(0)
		}
		rate := math.Round((1-math.Pow(salvage/cost, 1/life))*1000) / 1000
		total, dep := 0.0, 0.0
		for p := 1.0; p <= period; p++ {
			switch {
			case p == 1:
				dep = cost * rate * month / 12
			case p == math.Floor(life)+1:
				dep = (cost - total) * rate * (12 - month) / 12
			default:
				dep = (cost - total) * rate
			}
			total += dep
		}
		return eval.Number(dep)
	})

	// ---- Rates ----
	reg(catFin, "EFFECT", 2, 2, "nominal_rate, npery", "傳回有效年利率", func(a *eval.Args) eval.Value {
		r, n, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		n = math.Floor(n)
		if r <= 0 || n < 1 {
			return errNum
		}
		return eval.Number(math.Pow(1+r/n, n) - 1)
	})
	reg(catFin, "NOMINAL", 2, 2, "effect_rate, npery", "傳回名目年利率", func(a *eval.Args) eval.Value {
		r, n, err := twoNumbers(a)
		if err.IsErr() {
			return err
		}
		n = math.Floor(n)
		if r <= 0 || n < 1 {
			return errNum
		}
		return eval.Number(n * (math.Pow(1+r, 1/n) - 1))
	})
	reg(catFin, "PDURATION", 3, 3, "rate, pv, fv", "傳回投資達到指定金額所需的期數", func(a *eval.Args) eval.Value {
		r, pv, f, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if r <= 0 || pv <= 0 || f <= 0 {
			return errNum
		}
		return eval.Number((math.Log(f) - math.Log(pv)) / math.Log(1+r))
	})
	reg(catFin, "RRI", 3, 3, "nper, pv, fv", "傳回投資成長的等值利率", func(a *eval.Args) eval.Value {
		n, pv, f, err := threeNumbers(a)
		if err.IsErr() {
			return err
		}
		if n <= 0 || pv == 0 {
			return errNum
		}
		return eval.Number(math.Pow(f/pv, 1/n) - 1)
	})
	reg(catFin, "FVSCHEDULE", 2, 2, "principal, schedule", "傳回套用一系列複利率後的未來值", func(a *eval.Args) eval.Value {
		p, err := a.Number(0, 0)
		if err.IsErr() {
			return err
		}
		rates, err := numbersOf(a, 1, 2)
		if err.IsErr() {
			return err
		}
		for _, r := range rates {
			p *= 1 + r
		}
		return eval.Number(p)
	})
}

// tvmArgs reads (x0, x1, x2, [x3=0], [type=0]) for PMT-style functions;
// required is how many leading arguments are mandatory.
func tvmArgs(a *eval.Args, required int) (x0, x1, x2, x3, typ float64, err eval.Value) {
	if x0, x1, x2, err = threeNumbers(a); err.IsErr() {
		return
	}
	if x3, err = a.Number(3, 0); err.IsErr() {
		return
	}
	if typ, err = a.Number(4, 0); err.IsErr() {
		return
	}
	if typ != 0 {
		typ = 1
	}
	return
}

func pmt(r, n, pv, fv, typ float64) float64 {
	if r == 0 {
		return -(pv + fv) / n
	}
	g := math.Pow(1+r, n)
	return -(pv*g + fv) * r / ((1 + r*typ) * (g - 1))
}

func fv(r, n, p, pv, typ float64) float64 {
	if r == 0 {
		return -pv - p*n
	}
	g := math.Pow(1+r, n)
	return -pv*g - p*(1+r*typ)*(g-1)/r
}

// interestPart is the interest in period per of an annuity paying p.
func interestPart(r, per, p, pv, typ float64) float64 {
	if typ == 1 && per == 1 {
		return 0
	}
	// Balance at the start of the period.
	k := per - 1
	if typ == 1 {
		k = per - 2
		return fv(r, k, p, pv, 1)*r - p*r
	}
	return fv(r, k, p, pv, 0) * r
}

func hasSignChange(xs []float64) bool {
	pos, neg := false, false
	for _, x := range xs {
		pos = pos || x > 0
		neg = neg || x < 0
	}
	return pos && neg
}

// solve finds a root of f near guess with Newton's method (numerical
// derivative), falling back to bisection over a bracket.
func solve(f func(float64) float64, guess float64) (float64, bool) {
	x := guess
	for i := 0; i < 100; i++ {
		y := f(x)
		if math.Abs(y) < 1e-10 {
			return x, true
		}
		h := math.Max(1e-7, math.Abs(x)*1e-7)
		d := (f(x+h) - y) / h
		if d == 0 || math.IsNaN(d) {
			break
		}
		next := x - y/d
		if next <= -1 {
			next = (x - 1) / 2
		}
		if math.Abs(next-x) < 1e-12 {
			return next, !math.IsNaN(next)
		}
		x = next
	}
	// Bisection between -0.99 and a growing upper bound.
	lo, hi := -0.9999, 1.0
	for f(lo)*f(hi) > 0 && hi < 1e6 {
		hi *= 2
	}
	if f(lo)*f(hi) > 0 {
		return 0, false
	}
	for i := 0; i < 300; i++ {
		mid := (lo + hi) / 2
		if f(lo)*f(mid) <= 0 {
			hi = mid
		} else {
			lo = mid
		}
	}
	return (lo + hi) / 2, true
}
