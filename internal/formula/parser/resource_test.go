package parser

import (
	"strings"
	"testing"
)

func TestFormulaResourceBounds(t *testing.T) {
	for _, src := range []string{strings.Repeat("-", 129) + "1", strings.Repeat("(", 129) + "1" + strings.Repeat(")", 129), strings.Repeat("1+", 2100) + "1", `"` + strings.Repeat("x", 65536) + `"`} {
		if _, err := Parse(src); err == nil {
			t.Fatal("accepted excessive formula")
		}
	}
	if _, err := Parse(`REPT("` + strings.Repeat("a", 32768) + `",2)`); err != nil {
		t.Fatal("legitimate long literal rejected", err)
	}
}
