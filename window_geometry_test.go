package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"testing"
	"ww-excel/internal/model"
)

func TestWindowWorkAreasUsesLogicalRetinaCoordinates(t *testing.T) {
	raw := &application.Screen{ID: "primary", ScaleFactor: 2, WorkArea: application.Rect{Y: 66, Width: 3024, Height: 1772}}
	logical := &application.Screen{ID: "primary", IsPrimary: true, ScaleFactor: 2, WorkArea: application.Rect{Y: 33, Width: 1512, Height: 886}}
	areas := windowWorkAreas(raw, []*application.Screen{logical})
	if len(areas) != 1 || areas[0] != (model.WindowBounds{X: 0, Y: 33, Width: 1512, Height: 886}) {
		t.Fatal(areas)
	}
	// test8.xlsx stores xWindow=0, yWindow=660, width=18600, height=17680 twips.
	view := model.WindowView{Width: 930, Height: 884, Position: &model.WindowPosition{X: 0, Y: 33}}
	got := view.Place(model.WindowBounds{X: 80, Y: 60, Width: 1280, Height: 800}, areas)
	if got != (model.WindowBounds{X: 0, Y: 33, Width: 930, Height: 884}) {
		t.Fatalf("test8 window shifted: %+v", got)
	}
}

func TestWindowWorkAreasPrefersCurrentScreenAndFallsBackToPrimary(t *testing.T) {
	primary := &application.Screen{ID: "primary", IsPrimary: true, WorkArea: application.Rect{Y: 33, Width: 1512, Height: 886}}
	other := &application.Screen{ID: "other", WorkArea: application.Rect{X: -1920, Width: 1920, Height: 1080}}
	screens := []*application.Screen{other, primary}
	for _, current := range []*application.Screen{nil, {ID: "disconnected"}} {
		if got := windowWorkAreas(current, screens); len(got) != 2 || got[0].Y != 33 {
			t.Fatal(got)
		}
	}
	if got := windowWorkAreas(&application.Screen{ID: "other"}, screens); len(got) != 2 || got[0].X != -1920 {
		t.Fatal(got)
	}
	if got := windowWorkAreas(nil, nil); len(got) != 0 {
		t.Fatal(got)
	}
}
