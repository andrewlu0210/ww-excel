package numfmt

import (
	"strconv"
	"strings"
)

// Excel's DBNum1 year zero uses the white circle (U+25CB). Convert date
// tokens individually so digits inside quoted literals remain unchanged.
func chineseDateDigits(value string) string {
	digits := []rune("○一二三四五六七八九")
	var out strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			out.WriteRune(digits[r-'0'])
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
func chineseDateInteger(value int) string {
	if value < 0 || value >= 100 {
		return chineseDateDigits(strconv.Itoa(value))
	}
	if value < 10 {
		return chineseDateDigits(strconv.Itoa(value))
	}
	tens, ones := value/10, value%10
	result := ""
	if tens > 1 {
		result = chineseDateDigits(strconv.Itoa(tens))
	}
	result += "十"
	if ones > 0 {
		result += chineseDateDigits(strconv.Itoa(ones))
	}
	return result
}
