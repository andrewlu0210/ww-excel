package model

import "strconv"

// IconSetSize covers the 20 presets in Microsoft's ST_IconSetType definition.
func IconSetSize(name string) int {
	switch name {
	case "3Arrows", "3ArrowsGray", "3Flags", "3TrafficLights1", "3TrafficLights2", "3Signs", "3Symbols", "3Symbols2", "3Stars", "3Triangles":
		return 3
	case "4Arrows", "4ArrowsGray", "4RedToBlack", "4Rating", "4TrafficLights":
		return 4
	case "5Arrows", "5ArrowsGray", "5Rating", "5Quarters", "5Boxes":
		return 5
	}
	return 0
}

func IconThresholds(count int) []CFThreshold {
	if count < 1 {
		return nil
	}
	out := make([]CFThreshold, count)
	for i := range out {
		out[i] = CFThreshold{Type: "percent", Value: strconv.Itoa((i*100 + count/2) / count)}
	}
	return out
}
