// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"
)

func TestPackTrimsTransparentBoundsAndRendersOriginalSource(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(-5, 7, 27, 31))
	for y := source.Bounds().Min.Y + 5; y < source.Bounds().Min.Y+11; y++ {
		for x := source.Bounds().Min.X + 3; x < source.Bounds().Min.X+11; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 70, G: 120, A: 255})
		}
	}
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			if source.NRGBAAt(x, y).A == 0 {
				source.SetNRGBA(x, y, color.NRGBA{R: 231, G: 119, B: 77})
			}
		}
	}
	source.SetNRGBA(source.Bounds().Min.X+6, source.Bounds().Min.Y+8,
		color.NRGBA{R: 255, G: 70, B: 150})

	opts := Options{
		MinSize:         16,
		MaxSize:         16,
		Padding:         2,
		Extrude:         1,
		TrimTransparent: true,
		ForceSquare:     true,
	}
	atlas, err := Pack([]Sprite{{ID: "trimmed", Image: source}}, opts)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 8 || placement.Height != 6 {
		t.Fatalf("trimmed size = %dx%d, want 8x6", placement.Width, placement.Height)
	}

	if placement.OriginalWidth != 32 || placement.OriginalHeight != 24 ||
		placement.OffsetX != 3 || placement.OffsetY != 5 {
		t.Fatalf("trim metadata = %+v, want original=32x24 offset=3,5", placement)
	}

	if atlas.Layout.Render == nil || *atlas.Layout.Render != (RenderSettings{
		Padding:         2,
		Extrude:         1,
		TrimTransparent: true,
	}) {
		t.Fatalf("render settings = %+v", atlas.Layout.Render)
	}

	encoded, err := json.Marshal(atlas.Layout)
	if err != nil {
		t.Fatalf("Marshal layout: %v", err)
	}

	var roundTrip Layout
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("Unmarshal layout: %v", err)
	}

	rendered, err := Render(&roundTrip, []Source{{ID: "trimmed", Image: source}})
	if err != nil {
		t.Fatalf("Render original source: %v", err)
	}
	if !bytes.Equal(rendered.(*image.RGBA).Pix, atlas.Image.(*image.RGBA).Pix) {
		t.Fatal("JSON round-trip render differs from Pack output")
	}
	if got := atlas.Image.(*image.RGBA).RGBAAt(placement.X+3, placement.Y+3); got != (color.RGBA{}) {
		t.Fatalf("transparent interior pixel = %+v, want zero RGBA", got)
	}
}

func TestPackTrimsOnlyRightAndBottomWithZeroOffsets(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(-7, 4, 13, 14)
	source := image.NewRGBA(bounds)
	draw.Draw(
		source,
		image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+10, bounds.Min.Y+5),
		image.NewUniform(color.RGBA{G: 255, A: 255}),
		image.Point{},
		draw.Src,
	)

	atlas, err := Pack([]Sprite{{ID: "sprite", Image: source}}, Options{
		MinSize:         16,
		MaxSize:         16,
		TrimTransparent: true,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 10 || placement.Height != 5 {
		t.Fatalf("trimmed size = %dx%d, want 10x5", placement.Width, placement.Height)
	}

	if placement.OriginalWidth != 20 || placement.OriginalHeight != 10 ||
		placement.OffsetX != 0 || placement.OffsetY != 0 {
		t.Fatalf("trim metadata = %+v, want original=20x10 and zero offsets", placement)
	}

	encoded, err := json.Marshal(placement)
	if err != nil {
		t.Fatalf("Marshal placement: %v", err)
	}
	if strings.Contains(string(encoded), "offset_x") ||
		strings.Contains(string(encoded), "offset_y") {
		t.Fatalf("zero offsets were serialized: %s", encoded)
	}
}

func TestPackFullyTransparentSourcesBecomeZeroPixel(t *testing.T) {
	t.Parallel()

	large := image.NewNRGBA(image.Rect(-3, 8, 5, 12))
	for y := large.Bounds().Min.Y; y < large.Bounds().Max.Y; y++ {
		for x := large.Bounds().Min.X; x < large.Bounds().Max.X; x++ {
			large.SetNRGBA(x, y, color.NRGBA{R: 255, G: 90, B: 40})
		}
	}
	single := image.NewNRGBA(image.Rect(4, -2, 5, -1))
	single.SetNRGBA(4, -2, color.NRGBA{R: 201, G: 33, B: 87})

	for _, test := range []struct {
		name      string
		id        string
		source    image.Image
		originalW int
		originalH int
	}{
		{name: "large", id: "large", source: large, originalW: 8, originalH: 4},
		{name: "single pixel", id: "single", source: single},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			atlas, err := Pack([]Sprite{{ID: test.id, Image: test.source}}, Options{
				MinSize:         4,
				MaxSize:         4,
				TrimTransparent: true,
			})
			if err != nil {
				t.Fatalf("Pack: %v", err)
			}

			placement := atlas.Layout.Placements[0]
			if placement.Width != 1 || placement.Height != 1 {
				t.Fatalf("trimmed size = %dx%d, want 1x1", placement.Width, placement.Height)
			}
			if placement.OriginalWidth != test.originalW || placement.OriginalHeight != test.originalH {
				t.Fatalf(
					"original size = %dx%d, want %dx%d",
					placement.OriginalWidth, placement.OriginalHeight, test.originalW, test.originalH,
				)
			}
			if got := atlas.Image.At(placement.X, placement.Y); got != (color.RGBA{}) {
				t.Fatalf("fully transparent pixel = %v, want zero RGBA", got)
			}

			rendered, err := Render(&atlas.Layout, []Source{{ID: test.id, Image: test.source}})
			if err != nil {
				t.Fatalf("Render original source: %v", err)
			}
			if got := rendered.At(placement.X, placement.Y); got != (color.RGBA{}) {
				t.Fatalf("re-rendered transparent pixel = %v, want zero RGBA", got)
			}
		})
	}
}

func TestPackTrimOptionDoesNotAffectPlanOrOpaquePlacementMetadata(t *testing.T) {
	t.Parallel()

	opts := Options{MinSize: 8, MaxSize: 8, TrimTransparent: true}
	planned, err := Plan([]Item{{ID: "opaque", Width: 3, Height: 2}}, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if planned.Render != nil || planned.Placements[0].OriginalWidth != 0 {
		t.Fatalf("Plan applied image trimming: %+v", planned)
	}

	source := image.NewRGBA(image.Rect(0, 0, 3, 2))
	draw.Draw(
		source,
		source.Bounds(),
		image.NewUniform(color.RGBA{B: 255, A: 255}),
		image.Point{},
		draw.Src,
	)
	atlas, err := Pack([]Sprite{{ID: "opaque", Image: source}}, opts)
	if err != nil {
		t.Fatalf("Pack opaque source: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 3 || placement.Height != 2 ||
		placement.OriginalWidth != 0 || placement.OriginalHeight != 0 ||
		placement.OffsetX != 0 || placement.OffsetY != 0 {
		t.Fatalf("opaque placement has unexpected trim metadata: %+v", placement)
	}
	if atlas.Layout.Render == nil || !atlas.Layout.Render.TrimTransparent {
		t.Fatalf("render settings do not record trim option: %+v", atlas.Layout.Render)
	}
}

func TestPackTrimsPalettedAndSubImageSources(t *testing.T) {
	t.Parallel()

	palettedBounds := image.Rect(-6, 4, 10, 16)
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 240, A: 255}}
	paletted := image.NewPaletted(palettedBounds, palette)
	for y := palettedBounds.Min.Y + 3; y < palettedBounds.Min.Y+9; y++ {
		for x := palettedBounds.Min.X + 2; x < palettedBounds.Min.X+10; x++ {
			paletted.SetColorIndex(x, y, 1)
		}
	}

	full := image.NewNRGBA(image.Rect(-10, 20, 20, 50))
	sub := full.SubImage(image.Rect(-5, 25, 15, 45)).(*image.NRGBA)
	for y := sub.Bounds().Min.Y + 4; y < sub.Bounds().Min.Y+11; y++ {
		for x := sub.Bounds().Min.X + 6; x < sub.Bounds().Min.X+13; x++ {
			sub.SetNRGBA(x, y, color.NRGBA{G: 210, A: 255})
		}
	}

	tests := []struct {
		name   string
		id     string
		source image.Image
		width  int
		height int
		offset int
	}{
		{name: "paletted", id: "paletted", source: paletted, width: 8, height: 6, offset: 2},
		{name: "subimage", id: "subimage", source: sub, width: 7, height: 7, offset: 6},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			atlas, err := Pack([]Sprite{{ID: test.id, Image: test.source}}, Options{
				MinSize:         16,
				MaxSize:         16,
				TrimTransparent: true,
			})
			if err != nil {
				t.Fatalf("Pack: %v", err)
			}

			placement := atlas.Layout.Placements[0]
			if placement.Width != test.width ||
				placement.Height != test.height ||
				placement.OffsetX != test.offset {
				t.Fatalf("placement = %+v, want size=%dx%d offsetX=%d",
					placement, test.width, test.height, test.offset)
			}
		})
	}
}

func TestRenderTrimsAndRotatesCropWithExtrusion(t *testing.T) {
	t.Parallel()

	source := image.NewRGBA(image.Rect(-4, 5, 1, 11))
	colors := []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255},
		{B: 255, A: 255}, {R: 255, G: 255, A: 255},
		{R: 255, B: 255, A: 255}, {G: 255, B: 255, A: 255},
	}
	i := 0
	for y := 7; y < 10; y++ {
		for x := -3; x < -1; x++ {
			source.SetRGBA(x, y, colors[i])
			i++
		}
	}

	layout := &Layout{
		Width:  8,
		Height: 8,
		Render: &RenderSettings{Padding: 1, Extrude: 1, TrimTransparent: true},
		Placements: []Placement{{
			ID:             "crop",
			X:              2,
			Y:              2,
			Width:          2,
			Height:         3,
			Rotated:        true,
			OriginalWidth:  5,
			OriginalHeight: 6,
			OffsetX:        1,
			OffsetY:        2,
		}},
	}

	got, err := Render(layout, []Source{{ID: "crop", Image: source}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	wantPixels := [][]color.RGBA{
		{{R: 255, B: 255, A: 255}, {B: 255, A: 255}, {R: 255, A: 255}},
		{{G: 255, B: 255, A: 255}, {R: 255, G: 255, A: 255}, {G: 255, A: 255}},
	}
	for y := range wantPixels {
		for x := range wantPixels[y] {
			if pixel := got.(*image.RGBA).RGBAAt(2+x, 2+y); pixel != wantPixels[y][x] {
				t.Errorf("pixel at (%d,%d) = %+v, want %+v",
					2+x, 2+y, pixel, wantPixels[y][x])
			}
		}
	}
}

func TestRenderRejectsInvalidTrimMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		placement Placement
	}{
		{
			name:      "half original size",
			placement: Placement{ID: "x", Width: 1, Height: 1, OriginalWidth: 2},
		},
		{
			name:      "offset without original",
			placement: Placement{ID: "x", Width: 1, Height: 1, OffsetX: 1},
		},
		{
			name: "negative offset",
			placement: Placement{
				ID:             "x",
				Width:          1,
				Height:         1,
				OriginalWidth:  2,
				OriginalHeight: 2,
				OffsetX:        -1,
			},
		},
		{
			name: "offset beyond source",
			placement: Placement{
				ID:             "x",
				Width:          1,
				Height:         1,
				OriginalWidth:  2,
				OriginalHeight: 2,
				OffsetX:        2,
			},
		},
		{
			name: "crop larger than source",
			placement: Placement{
				ID:             "x",
				Width:          3,
				Height:         1,
				OriginalWidth:  2,
				OriginalHeight: 2,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Render(&Layout{
				Width: 4, Height: 4,
				Placements: []Placement{test.placement},
			}, nil)
			if !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("Render error = %v, want ErrInvalidLayout", err)
			}
		})
	}
}
