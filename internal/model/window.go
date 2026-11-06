package model

// WindowView describes an opening window, independent of worksheet zoom.
type WindowView struct {
	Width, Height int
	Position      *WindowPosition
}

type WindowPosition struct{ X, Y int }

// WindowBounds uses desktop coordinates with a top-left origin.
type WindowBounds struct{ X, Y, Width, Height int }

func (s WindowView) HasSize() bool { return s.Width > 0 && s.Height > 0 }
func (s WindowView) Valid() bool   { return s.HasSize() || s.Position != nil }

// Place selects the screen with the largest overlap with the saved window.
// The first work area is the current screen and is the off-screen fallback.
func (s WindowView) Place(current WindowBounds, areas []WindowBounds) WindowBounds {
	desired := current
	if s.HasSize() {
		desired.Width, desired.Height = s.Width, s.Height
	}
	if s.Position != nil {
		desired.X, desired.Y = s.Position.X, s.Position.Y
	}
	if len(areas) == 0 {
		return desired
	}
	target := areas[0]
	if s.Position != nil {
		best := int64(0)
		for _, area := range areas {
			width := max(0, min(desired.X+desired.Width, area.X+area.Width)-max(desired.X, area.X))
			height := max(0, min(desired.Y+desired.Height, area.Y+area.Height)-max(desired.Y, area.Y))
			overlap := int64(width) * int64(height)
			if overlap > best {
				target, best = area, overlap
			}
		}
	}
	size := (WindowView{Width: desired.Width, Height: desired.Height}).Fit(target.Width, target.Height)
	return WindowBounds{
		X:     max(target.X, min(desired.X, target.X+target.Width-size.Width)),
		Y:     max(target.Y, min(desired.Y, target.Y+target.Height-size.Height)),
		Width: size.Width, Height: size.Height,
	}
}

// Fit keeps the window usable and within the current screen work area.
func (s WindowView) Fit(width, height int) WindowView {
	s.Width = max(640, s.Width)
	s.Height = max(480, s.Height)
	if width > 0 {
		s.Width = min(s.Width, width)
	}
	if height > 0 {
		s.Height = min(s.Height, height)
	}
	return s
}
