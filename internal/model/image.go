package model

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
)

// Rendered applies the source rectangle before fitting the picture to its
// object frame. Keep the original bytes and crop metadata for native saves.
func (img *Image) Rendered() *Image {
	if img == nil || img.Crop == (ImageCrop{}) {
		return img
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 64_000_000 {
		return img
	}
	c := img.Crop
	x0 := math.Round(float64(cfg.Width) * float64(c.Left) / 100000)
	y0 := math.Round(float64(cfg.Height) * float64(c.Top) / 100000)
	x1 := math.Round(float64(cfg.Width) * (1 - float64(c.Right)/100000))
	y1 := math.Round(float64(cfg.Height) * (1 - float64(c.Bottom)/100000))
	w, h := x1-x0, y1-y0
	if w <= 0 || h <= 0 || w > 64_000_000 || h > 64_000_000 || w*h > 64_000_000 {
		return img
	}
	src, _, err := image.Decode(bytes.NewReader(img.Data))
	if err != nil {
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min.Add(image.Pt(int(x0), int(y0))), draw.Src)
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return img
	}
	return &Image{Data: out.Bytes(), Ext: ".png"}
}
