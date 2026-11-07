package model

import (
	"math"
	"testing"
)

func TestPatternDensityAndGradientGeometry(t *testing.T) {
	for pattern, want := range map[string]int{"darkGray": 48, "mediumGray": 32, "lightGray": 16, "gray125": 8, "gray0625": 4} {
		got := 0
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				if PatternInk(pattern, x, y) {
					got++
				}
				if PatternInk(pattern, x+8, y+8) != PatternInk(pattern, x, y) {
					t.Fatal("pattern seam")
				}
			}
		}
		if got != want {
			t.Fatalf("%s: %d", pattern, got)
		}
	}
	f := FillSpec{Kind: "path", Left: .5, Right: .5, Top: .5, Bottom: .5}
	if FillPosition(f, .5, .5) != 0 || FillPosition(f, 1, .5) != 1 {
		t.Fatal("path geometry")
	}
	stops := []FillStop{{Position: 0, Color: "FF0000"}, {Position: .25, Color: "00FF00"}, {Position: 1, Color: "0000FF"}}
	c := interpolateFill(stops, .625)
	if c.R != 0 || c.G != 128 || c.B != 128 {
		t.Fatal(c)
	}
}
func TestNormalizeAdvancedFillRejectsInvalidAndInternsCanonical(t *testing.T) {
	spec := FillSpec{Kind: "linear", Degree: 37, Stops: `[{"position":1,"color":"abcdef"},{"position":0,"color":"123456"}]`}
	normalized, err := NormalizeFillSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.ColorStops()[0].Position != 0 || normalized.ColorStops()[1].Color != "ABCDEF" {
		t.Fatal(normalized)
	}
	table := NewStyleTable(Style{})
	if table.Intern(Style{FillSpec: normalized}) != table.Intern(Style{FillSpec: normalized}) {
		t.Fatal("style dedupe")
	}
	for _, f := range []FillSpec{{Kind: "linear", Degree: math.NaN(), Stops: spec.Stops}, {Kind: "linear", Degree: 361, Stops: spec.Stops}, {Kind: "linear", Stops: `[]`}, {Kind: "path", Left: .8, Right: .2, Stops: spec.Stops}, {Kind: "pattern", Pattern: `bad"`, Foreground: "FFFFFF", Background: "000000"}} {
		if _, err := NormalizeFillSpec(f); err == nil {
			t.Fatal("invalid fill accepted")
		}
	}
}
