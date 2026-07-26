// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"image"
	"testing"
)

func TestRenderRequiresLayout(t *testing.T) {
	t.Parallel()

	_, err := Render(nil, nil)
	if !errors.Is(err, ErrLayoutRequired) {
		t.Fatalf("error %v, want ErrLayoutRequired", err)
	}
}

func TestRenderRequiresSourcesForPlacements(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 8, Height: 8},
		},
	}
	_, err := Render(layout, nil)
	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("error %v, want ErrSourceNotFound", err)
	}
}

func TestRenderSizeMismatch(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 8, Height: 8},
		},
	}
	sources := []Source{
		{ID: "a", Image: image.NewRGBA(image.Rect(0, 0, 9, 8))},
	}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrSourceSizeMismatch) {
		t.Fatalf("error %v, want ErrSourceSizeMismatch", err)
	}
}

func TestRenderInvalidLayoutSize(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  0,
		Height: 32,
	}
	_, err := Render(layout, nil)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderValidatesLayoutBeforeSources(t *testing.T) {
	t.Parallel()

	layout := &Layout{Width: 0, Height: 1}
	sources := []Source{{ID: "", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))}}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderRejectsInvalidPlacementGeometry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		placement Placement
	}{
		{name: "rotated right edge", placement: Placement{ID: "a", X: 2, Width: 2, Height: 1, Rotated: true}},
		{name: "negative x", placement: Placement{ID: "a", X: -1, Width: 1, Height: 1}},
		{name: "negative y", placement: Placement{ID: "a", Y: -1, Width: 1, Height: 1}},
		{name: "zero width", placement: Placement{ID: "a", Width: 0, Height: 1}},
		{name: "zero height", placement: Placement{ID: "a", Width: 1, Height: 0}},
		{name: "rotated effective width", placement: Placement{ID: "a", X: 1, Width: 1, Height: 2, Rotated: true}},
		{name: "rotated effective height", placement: Placement{ID: "a", Y: 1, Width: 2, Height: 1, Rotated: true}},
		{name: "unrotated clipping", placement: Placement{ID: "a", X: 1, Width: 2, Height: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout := &Layout{Width: 2, Height: 2, Placements: []Placement{test.placement}}
			src := image.NewRGBA(image.Rect(0, 0, test.placement.Width, test.placement.Height))
			got, err := Render(layout, []Source{{ID: "a", Image: src}})
			if !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("error %v, want ErrInvalidLayout", err)
			}
			if got != nil {
				t.Fatalf("image %v, want nil", got)
			}
		})
	}
}

func TestRenderRejectsUnrepresentableAtlasSize(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	layout := &Layout{Width: maxInt, Height: 1}

	_, err := Render(layout, nil)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderValidRotatedPlacementAtBoundary(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  2,
		Height: 2,
		Placements: []Placement{
			{ID: "a", X: 1, Y: 0, Width: 2, Height: 1, Rotated: true},
		},
	}
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, image.White)
	src.Set(1, 0, image.Black)

	got, err := Render(layout, []Source{{ID: "a", Image: src}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rgba := got.(*image.RGBA)
	if pixel := rgba.RGBAAt(1, 0); pixel.R != 255 || pixel.A != 255 {
		t.Fatalf("top rotated pixel mismatch: %+v", pixel)
	}
	if pixel := rgba.RGBAAt(1, 1); pixel.R != 0 || pixel.A != 255 {
		t.Fatalf("bottom rotated pixel mismatch: %+v", pixel)
	}
}

func TestRenderValidatesAllPlacementsBeforeDrawing(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  2,
		Height: 2,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 1, Height: 1},
			{ID: "b", X: 2, Y: 0, Width: 1, Height: 1},
		},
	}
	sources := []Source{
		{ID: "a", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))},
		{ID: "b", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))},
	}

	got, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
	if got != nil {
		t.Fatalf("image %v, want nil", got)
	}
}

func TestRenderDuplicateSourceID(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
	}
	sources := []Source{
		{ID: "dup", Image: image.NewRGBA(image.Rect(0, 0, 8, 8))},
		{ID: "dup", Image: image.NewRGBA(image.Rect(0, 0, 8, 8))},
	}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("error %v, want ErrInvalidItem", err)
	}
}

func TestRenderRotatedPlacement(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  4,
		Height: 4,
		Placements: []Placement{
			{
				ID:      "a",
				X:       1,
				Y:       1,
				Width:   2,
				Height:  1,
				Rotated: true,
			},
		},
	}

	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, image.White)
	src.Set(1, 0, image.Black)

	atlas, err := Render(layout, []Source{{ID: "a", Image: src}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rgba, ok := atlas.(*image.RGBA)
	if !ok {
		t.Fatalf("atlas type %T, want *image.RGBA", atlas)
	}

	// Original row: [white, black], rotated clockwise into column:
	// top pixel becomes white at (1,1), bottom pixel becomes black at (1,2).
	white := rgba.RGBAAt(1, 1)
	black := rgba.RGBAAt(1, 2)
	if white.A == 0 || white.R == 0 {
		t.Fatalf("top rotated pixel mismatch: %+v", white)
	}
	if black.A == 0 || black.R != 0 {
		t.Fatalf("bottom rotated pixel mismatch: %+v", black)
	}
}
