package model

import "testing"

func TestWindowViewFit(t *testing.T) {
	for _, tc := range []struct {
		size          WindowView
		width, height int
		want          WindowView
	}{
		{WindowView{Width: 1362, Height: 883}, 1920, 1080, WindowView{Width: 1362, Height: 883}},
		{WindowView{Width: 1362, Height: 883}, 1200, 750, WindowView{Width: 1200, Height: 750}},
		{WindowView{Width: 1, Height: 1}, 1200, 750, WindowView{Width: 640, Height: 480}},
		{WindowView{Width: 1, Height: 1}, 500, 300, WindowView{Width: 500, Height: 300}},
	} {
		if got := tc.size.Fit(tc.width, tc.height); got != tc.want {
			t.Fatalf("%+v: got %+v, want %+v", tc, got, tc.want)
		}
	}
}

func TestWindowViewPlace(t *testing.T) {
	current := WindowBounds{X: 80, Y: 60, Width: 1280, Height: 800}
	primary := WindowBounds{X: 0, Y: 25, Width: 1440, Height: 850}
	left := WindowBounds{X: -1920, Y: 0, Width: 1920, Height: 1080}
	above := WindowBounds{X: 0, Y: -1080, Width: 1920, Height: 1080}
	for _, tc := range []struct {
		name  string
		view  WindowView
		areas []WindowBounds
		want  WindowBounds
	}{
		{"saved position", WindowView{Width: 800, Height: 600, Position: &WindowPosition{72, 33}}, []WindowBounds{primary}, WindowBounds{72, 33, 800, 600}},
		{"menu bar", WindowView{Width: 800, Height: 600, Position: &WindowPosition{0, 0}}, []WindowBounds{primary}, WindowBounds{0, 25, 800, 600}},
		{"left monitor", WindowView{Width: 800, Height: 600, Position: &WindowPosition{-1800, 100}}, []WindowBounds{primary, left}, WindowBounds{-1800, 100, 800, 600}},
		{"above monitor", WindowView{Width: 800, Height: 600, Position: &WindowPosition{100, -1000}}, []WindowBounds{primary, above}, WindowBounds{100, -1000, 800, 600}},
		{"disconnected monitor", WindowView{Width: 800, Height: 600, Position: &WindowPosition{9000, 9000}}, []WindowBounds{primary, left}, WindowBounds{640, 275, 800, 600}},
		{"oversized", WindowView{Width: 3000, Height: 2000, Position: &WindowPosition{100, 100}}, []WindowBounds{primary}, primary},
		{"size only", WindowView{Width: 800, Height: 600}, []WindowBounds{primary, left}, WindowBounds{80, 60, 800, 600}},
		{"position only", WindowView{Position: &WindowPosition{100, 50}}, []WindowBounds{primary}, WindowBounds{100, 50, 1280, 800}},
		{"no screens", WindowView{Width: 800, Height: 600, Position: &WindowPosition{72, 33}}, nil, WindowBounds{72, 33, 800, 600}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.view.Place(current, tc.areas); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
