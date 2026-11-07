package model

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
)

// FillSpec is comparable, including its canonical JSON stop list, so full
// styles remain safe map keys. The zero value keeps legacy solid fills intact.
type FillSpec struct {
	Kind       string  `json:"kind,omitempty"`
	Pattern    string  `json:"pattern,omitempty"`
	Foreground string  `json:"foreground,omitempty"`
	Background string  `json:"background,omitempty"`
	Degree     float64 `json:"degree,omitempty"`
	Left       float64 `json:"left,omitempty"`
	Right      float64 `json:"right,omitempty"`
	Top        float64 `json:"top,omitempty"`
	Bottom     float64 `json:"bottom,omitempty"`
	Stops      string  `json:"stops,omitempty"`
}
type FillStop struct {
	Position float64 `json:"position"`
	Color    string  `json:"color"`
}

var FillPatterns = []string{"darkGray", "mediumGray", "lightGray", "gray125", "gray0625", "darkHorizontal", "darkVertical", "darkDown", "darkUp", "darkGrid", "darkTrellis", "lightHorizontal", "lightVertical", "lightDown", "lightUp", "lightGrid", "lightTrellis"}
var fillHex = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

func (f FillSpec) ColorStops() []FillStop {
	if len(f.Stops) > 32768 {
		return nil
	}
	var stops []FillStop
	if json.Unmarshal([]byte(f.Stops), &stops) != nil || len(stops) > 256 {
		return nil
	}
	return stops
}
func EncodeFillStops(stops []FillStop) string { b, _ := json.Marshal(stops); return string(b) }
func NormalizeFillSpec(f FillSpec) (FillSpec, error) {
	bad := func() (FillSpec, error) { return FillSpec{}, errors.New("填滿參數不正確") }
	color := func(s string) bool { return fillHex.MatchString(s) }
	switch f.Kind {
	case "":
		return FillSpec{}, nil
	case "pattern":
		known := false
		for _, p := range FillPatterns {
			known = known || f.Pattern == p
		}
		if !known || !color(f.Foreground) || !color(f.Background) {
			return bad()
		}
		return FillSpec{Kind: "pattern", Pattern: f.Pattern, Foreground: strings.ToUpper(f.Foreground), Background: strings.ToUpper(f.Background)}, nil
	case "linear", "path":
		for _, n := range []float64{f.Degree, f.Left, f.Right, f.Top, f.Bottom} {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return bad()
			}
		}
		if f.Degree < 0 || f.Degree > 360 || f.Left < 0 || f.Left > 1 || f.Right < 0 || f.Right > 1 || f.Top < 0 || f.Top > 1 || f.Bottom < 0 || f.Bottom > 1 || f.Left > f.Right || f.Top > f.Bottom {
			return bad()
		}
		stops := f.ColorStops()
		if len(stops) < 2 {
			return bad()
		}
		for i, s := range stops {
			if !color(s.Color) || math.IsNaN(s.Position) || math.IsInf(s.Position, 0) || s.Position < 0 || s.Position > 1 {
				return bad()
			}
			stops[i].Color = strings.ToUpper(s.Color)
		}
		sort.SliceStable(stops, func(i, j int) bool { return stops[i].Position < stops[j].Position })
		f = FillSpec{Kind: f.Kind, Degree: f.Degree, Left: f.Left, Right: f.Right, Top: f.Top, Bottom: f.Bottom, Stops: EncodeFillStops(stops)}
		if f.Kind == "linear" {
			f.Left = 0
			f.Right = 0
			f.Top = 0
			f.Bottom = 0
		} else {
			f.Degree = 0
		}
		return f, nil
	}
	return bad()
}
func (s Style) HasFill() bool { return s.Fill != "" || s.FillSpec.Kind != "" }
