package model

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestImageSourceRectangle(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	src.SetNRGBA(2, 3, color.NRGBA{R: 255, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, src); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		crop       ImageCrop
		w, h, x, y int
	}{
		{"crop", ImageCrop{Left: 20000, Top: 30000, Right: 10000, Bottom: 20000}, 7, 5, 0, 0},
		{"padding", ImageCrop{Left: -10000, Top: -20000}, 11, 12, 3, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &Image{Data: data.Bytes(), Ext: ".png", Crop: tc.crop}
			rendered := original.Rendered()
			got, _, err := image.Decode(bytes.NewReader(rendered.Data))
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds().Dx() != tc.w || got.Bounds().Dy() != tc.h || color.NRGBAModel.Convert(got.At(tc.x, tc.y)) != (color.NRGBA{R: 255, A: 255}) {
				t.Fatalf("incorrect crop: bounds=%v pixel=%v", got.Bounds(), got.At(tc.x, tc.y))
			}
			if !bytes.Equal(original.Data, data.Bytes()) || original.Crop != tc.crop || rendered.Crop != (ImageCrop{}) {
				t.Fatal("source changed or rendered image still has crop metadata")
			}
		})
	}
	for _, crop := range []ImageCrop{{}, {Left: 100000}, {Top: 90000, Bottom: 90000}, {Left: -1000000000000}} {
		img := &Image{Data: data.Bytes(), Ext: ".png", Crop: crop}
		if img.Rendered() != img {
			t.Fatalf("expected unchanged image for %v", crop)
		}
	}
}
