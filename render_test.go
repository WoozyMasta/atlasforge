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
