package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestRuntimeIconHasRoundedCorners(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(appIcon))
	if err != nil {
		t.Fatalf("Dock/About icon must be a PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Fatalf("unexpected icon size: %v", bounds)
	}
	for _, p := range [][2]int{{0, 0}, {1023, 0}, {0, 1023}, {1023, 1023}, {88, 88}, {935, 88}, {88, 935}, {935, 935}} {
		if _, _, _, alpha := img.At(p[0], p[1]).RGBA(); alpha != 0 {
			t.Fatalf("rounded corner must be transparent: %v", p)
		}
	}
	for _, p := range [][2]int{{512, 90}, {90, 512}, {933, 512}, {512, 933}} {
		if _, _, _, alpha := img.At(p[0], p[1]).RGBA(); alpha < 64000 {
			t.Fatalf("the inset illustration must remain opaque: %v", p)
		}
	}
	// The v1.2.6 icon uses an 88-pixel transparent Dock margin.
	for _, offset := range []int{0, 87, 936, 1023} {
		for along := 0; along < 1024; along += 64 {
			for _, p := range [][2]int{{offset, along}, {along, offset}} {
				if _, _, _, alpha := img.At(p[0], p[1]).RGBA(); alpha != 0 {
					t.Fatalf("Dock margin must be transparent: %v", p)
				}
			}
		}
	}
	if _, _, _, alpha := img.At(512, 512).RGBA(); alpha < 64000 {
		t.Fatal("the illustration must remain visible and nearly opaque")
	}
}
