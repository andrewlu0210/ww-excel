package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestRuntimeIconIsVisibleTransparentPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		t.Fatalf("Dock/About icon must be a PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Fatalf("unexpected icon size: %v", bounds)
	}
	for _, p := range [][2]int{{0, 0}, {1023, 0}, {0, 1023}, {1023, 1023}} {
		if _, _, _, alpha := img.At(p[0], p[1]).RGBA(); alpha != 0 {
			t.Fatalf("outer corner must remain transparent: %v", p)
		}
	}
	if _, _, _, alpha := img.At(512, 512).RGBA(); alpha < 64000 {
		t.Fatal("the illustration must remain visible and nearly opaque")
	}
}
