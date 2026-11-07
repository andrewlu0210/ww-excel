package model

import (
	"image"
	"image/color"
	"math"
	"strconv"
)

func fillRGB(s string) color.RGBA {
	n, _ := strconv.ParseUint(s, 16, 32)
	return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
}

// PatternInk uses an eight-pixel tile. Keep the frontend's tile sampler in sync.
func PatternInk(pattern string, x, y int) bool {
	x = ((x % 8) + 8) % 8
	y = ((y % 8) + 8) % 8
	switch pattern {
	case "darkGray":
		return !(x%2 == 0 && y%2 == 0)
	case "mediumGray":
		return (x+y)%2 == 0
	case "lightGray":
		return x%2 == 0 && y%2 == 0
	case "gray125":
		return y%2 == 0 && x%4 == y%4
	case "gray0625":
		return y%4 == 0 && x%4 == 0
	case "darkHorizontal":
		return y%4 < 2
	case "darkVertical":
		return x%4 < 2
	case "darkDown":
		return (x-y+8)%4 < 2
	case "darkUp":
		return (x+y)%4 < 2
	case "darkGrid":
		return x%4 < 2 || y%4 < 2
	case "darkTrellis":
		return (x+y)%4 < 2 || (x-y+8)%4 < 2
	case "lightHorizontal":
		return y == 0
	case "lightVertical":
		return x == 0
	case "lightDown":
		return (x-y+8)%8 == 0
	case "lightUp":
		return (x+y)%8 == 0
	case "lightGrid":
		return x == 0 || y == 0
	case "lightTrellis":
		return (x+y)%8 == 0 || (x-y+8)%8 == 0
	}
	return false
}
func FillPosition(f FillSpec, x, y float64) float64 { return FillPositionInRect(f, x, y, 1, 1) }
func FillPositionInRect(f FillSpec, x, y, width, height float64) float64 {
	if f.Kind == "path" {
		axis := func(p, a, b float64) float64 {
			if p < a && a > 0 {
				return (a - p) / a
			}
			if p > b && b < 1 {
				return (p - b) / (1 - b)
			}
			return 0
		}
		return min(1, max(axis(x, f.Left, f.Right), axis(y, f.Top, f.Bottom)))
	}
	angle := f.Degree * math.Pi / 180
	c, s := math.Cos(angle)*width, math.Sin(angle)*height
	return min(1, max(0, .5+((x-.5)*c+(y-.5)*s)/(math.Abs(c)+math.Abs(s))))
}
func interpolateFill(stops []FillStop, t float64) color.RGBA {
	if len(stops) == 0 {
		return color.RGBA{255, 255, 255, 255}
	}
	if t <= stops[0].Position {
		return fillRGB(stops[0].Color)
	}
	for i := 1; i < len(stops); i++ {
		b := stops[i]
		if t > b.Position {
			continue
		}
		a := stops[i-1]
		ratio := 1.0
		if b.Position > a.Position {
			ratio = (t - a.Position) / (b.Position - a.Position)
		}
		ca, cb := fillRGB(a.Color), fillRGB(b.Color)
		mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*ratio)) }
		return color.RGBA{mix(ca.R, cb.R), mix(ca.G, cb.G), mix(ca.B, cb.B), 255}
	}
	return fillRGB(stops[len(stops)-1].Color)
}

// PaintFill makes a bounded raster for PDF; cached by the caller per geometry.
func PaintFill(f FillSpec, width, height int) *image.RGBA {
	width = max(1, min(width, 512))
	height = max(1, min(height, 512))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	stops := f.ColorStops()
	fg, bg := fillRGB(f.Foreground), fillRGB(f.Background)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var c color.RGBA
			if f.Kind == "pattern" {
				c = bg
				if PatternInk(f.Pattern, x, y) {
					c = fg
				}
			} else {
				c = interpolateFill(stops, FillPositionInRect(f, (float64(x)+.5)/float64(width), (float64(y)+.5)/float64(height), float64(width), float64(height)))
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}
