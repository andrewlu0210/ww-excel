package functions

import (
	"math"
	"strings"
	"time"

	"ww-excel/internal/formula/eval"
	"ww-excel/internal/numfmt"
)

func init() {
	reg(catDate, "DATE", 3, 3, "year, month, day", "傳回指定日期的序列值", func(a *eval.Args) eval.Value {
		y, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		m, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		d, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		if y < 0 || y > 9999 {
			return errNum
		}
		if y < 1900 {
			y += 1900
		}
		s := numfmt.DateSerial(y, m, d)
		if s < 0 || s > numfmt.DateSerial(9999, 12, 31) {
			return errNum
		}
		return eval.Number(s)
	})
	reg(catDate, "TIME", 3, 3, "hour, minute, second", "傳回指定時間的序列值（0 到 1 之間的小數）", func(a *eval.Args) eval.Value {
		h, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		m, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		s, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		secs := h*3600 + m*60 + s
		if secs < 0 {
			return errNum
		}
		return eval.Number(float64(secs%86400) / 86400)
	})
	regVolatile(catDate, "TODAY", 0, 0, "", "傳回今天的日期", func(a *eval.Args) eval.Value {
		return eval.Number(math.Floor(numfmt.TimeSerial(a.Ev().Now)))
	})
	regVolatile(catDate, "NOW", 0, 0, "", "傳回目前的日期與時間", func(a *eval.Args) eval.Value {
		return eval.Number(numfmt.TimeSerial(a.Ev().Now))
	})
	datePart := func(name, desc string, pick func(y, m, d int) int) {
		reg(catDate, name, 1, 1, "serial_number", desc, func(a *eval.Args) eval.Value {
			s, err := serialArg(a, 0)
			if err.IsErr() {
				return err
			}
			return eval.Number(float64(pick(numfmt.YMD(s))))
		})
	}
	datePart("YEAR", "傳回日期的年份", func(y, _, _ int) int { return y })
	datePart("MONTH", "傳回日期的月份（1 到 12）", func(_, m, _ int) int { return m })
	datePart("DAY", "傳回日期是當月的第幾天", func(_, _, d int) int { return d })
	timePart := func(name, desc string, pick func(h, m, s int) int) {
		reg(catDate, name, 1, 1, "serial_number", desc, func(a *eval.Args) eval.Value {
			s, err := serialArg(a, 0)
			if err.IsErr() {
				return err
			}
			return eval.Number(float64(pick(numfmt.HMS(s))))
		})
	}
	timePart("HOUR", "傳回時間的小時（0 到 23）", func(h, _, _ int) int { return h })
	timePart("MINUTE", "傳回時間的分鐘", func(_, m, _ int) int { return m })
	timePart("SECOND", "傳回時間的秒數", func(_, _, s int) int { return s })
	reg(catDate, "WEEKDAY", 1, 2, "serial_number, [return_type]", "傳回日期是星期幾", func(a *eval.Args) eval.Value {
		s, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		kind, err := a.Int(1, 1)
		if err.IsErr() {
			return err
		}
		wd := numfmt.Weekday(s) // 0 = Sunday
		switch kind {
		case 1:
			return eval.Number(float64(wd + 1))
		case 2:
			return eval.Number(float64((wd+6)%7 + 1))
		case 3:
			return eval.Number(float64((wd + 6) % 7))
		}
		return errNum
	})
	reg(catDate, "EDATE", 2, 2, "start_date, months", "傳回幾個月之前或之後的同一天", func(a *eval.Args) eval.Value {
		return monthShift(a, false)
	})
	reg(catDate, "EOMONTH", 2, 2, "start_date, months", "傳回幾個月之前或之後那個月的最後一天", func(a *eval.Args) eval.Value {
		return monthShift(a, true)
	})
	reg(catDate, "DAYS", 2, 2, "end_date, start_date", "傳回兩個日期之間的天數", func(a *eval.Args) eval.Value {
		end, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		start, err := serialArg(a, 1)
		if err.IsErr() {
			return err
		}
		return eval.Number(math.Floor(end) - math.Floor(start))
	})
	reg(catDate, "DATEDIF", 3, 3, "start_date, end_date, unit", "傳回兩個日期之間的年（\"Y\"）、月（\"M\"）或天（\"D\"）數", func(a *eval.Args) eval.Value {
		start, err := serialArg(a, 0)
		if err.IsErr() {
			return err
		}
		end, err := serialArg(a, 1)
		if err.IsErr() {
			return err
		}
		unit, err := a.Text(2, "")
		if err.IsErr() {
			return err
		}
		if start > end {
			return errNum
		}
		y0, m0, d0 := numfmt.YMD(start)
		y1, m1, d1 := numfmt.YMD(end)
		months := (y1-y0)*12 + m1 - m0
		if d1 < d0 {
			months--
		}
		switch strings.ToUpper(unit) {
		case "Y":
			return eval.Number(float64(months / 12))
		case "M":
			return eval.Number(float64(months))
		case "D":
			return eval.Number(math.Floor(end) - math.Floor(start))
		}
		return errNum
	})
}

// serialArg reads a date serial; negative serials are #NUM!.
func serialArg(a *eval.Args, i int) (float64, eval.Value) {
	s, err := a.Number(i, 0)
	if err.IsErr() {
		return 0, err
	}
	if s < 0 {
		return 0, errNum
	}
	return s, eval.Empty
}

func monthShift(a *eval.Args, endOfMonth bool) eval.Value {
	s, err := serialArg(a, 0)
	if err.IsErr() {
		return err
	}
	months, err := a.Int(1, 0)
	if err.IsErr() {
		return err
	}
	y, m, d := numfmt.YMD(s)
	target := time.Date(y, time.Month(m)+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := target.AddDate(0, 1, -1).Day()
	if endOfMonth || d > last {
		d = last
	}
	out := numfmt.DateSerial(target.Year(), int(target.Month()), d)
	if out < 0 {
		return errNum
	}
	return eval.Number(out)
}
