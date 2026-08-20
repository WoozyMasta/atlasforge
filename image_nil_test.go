// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestPackRejectsNilImageValues(t *testing.T) {
	t.Parallel()

	var nilRGBA *image.RGBA
	var nilNRGBA *image.NRGBA
	var nilPaletted *image.Paletted
	var nilCustom *nilPointerImage
	tests := []struct {
		name string
		img  image.Image
	}{
		{name: "nil interface"},
		{name: "RGBA", img: nilRGBA},
		{name: "NRGBA", img: nilNRGBA},
		{name: "Paletted", img: nilPaletted},
		{name: "custom pointer", img: nilCustom},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Pack([]Sprite{{ID: "a", Image: test.img}}, Options{MinSize: 16, MaxSize: 16})
			if !errors.Is(err, ErrSourceImageMissing) {
				t.Fatalf("error %v, want ErrSourceImageMissing", err)
			}
		})
	}
}

func TestRenderRejectsNilImageValues(t *testing.T) {
	t.Parallel()

	var nilRGBA *image.RGBA
	var nilNRGBA *image.NRGBA
	var nilPaletted *image.Paletted
	var nilCustom *nilPointerImage
	tests := []struct {
		name string
		img  image.Image
	}{
		{name: "nil interface"},
		{name: "RGBA", img: nilRGBA},
		{name: "NRGBA", img: nilNRGBA},
		{name: "Paletted", img: nilPaletted},
		{name: "custom pointer", img: nilCustom},
	}
	layout := &Layout{
		Width:  16,
		Height: 16,
		Placements: []Placement{
			{ID: "a", Width: 1, Height: 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Render(layout, []Source{{ID: "a", Image: test.img}})
			if !errors.Is(err, ErrSourceImageMissing) {
				t.Fatalf("error %v, want ErrSourceImageMissing", err)
			}
		})
	}
}

// nilPointerImage implements image.Image through a pointer that may be nil.
type nilPointerImage struct{}

// ColorModel returns the format used by nilPointerImage.
func (*nilPointerImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds returns a single-pixel rectangle.
func (*nilPointerImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, 1, 1)
}

// At returns an opaque black pixel.
func (*nilPointerImage) At(int, int) color.Color {
	return color.RGBA{A: 255}
}
