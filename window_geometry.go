package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"ww-excel/internal/model"
)

// GetScreen can return pre-layout Retina coordinates. Use it only for the ID;
// the screen manager's list has canonical logical desktop coordinates.
func windowWorkAreas(current *application.Screen, screens []*application.Screen) []model.WindowBounds {
	var preferred *application.Screen
	for _, screen := range screens {
		if current != nil && screen.ID == current.ID {
			preferred = screen
			break
		}
		if screen.IsPrimary {
			preferred = screen
		}
	}
	if preferred == nil && len(screens) > 0 {
		preferred = screens[0]
	}
	if preferred == nil {
		return nil
	}
	areaOf := func(screen *application.Screen) model.WindowBounds {
		area := screen.WorkArea
		return model.WindowBounds{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height}
	}
	areas := []model.WindowBounds{areaOf(preferred)}
	for _, screen := range screens {
		if screen.ID != preferred.ID {
			areas = append(areas, areaOf(screen))
		}
	}
	return areas
}
