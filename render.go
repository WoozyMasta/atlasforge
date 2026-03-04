// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"image/draw"
)

// Render renders a precomputed layout using provided image sources.
func Render(layout *Layout, sources []Source) (image.Image, error) {
	sourceMap, err := buildSourceMap(sources)
	if err != nil {
		return nil, err
	}

	return renderWithSourceMap(layout, sourceMap)
}

// buildSourceMap validates sources and builds ID->image map.
func buildSourceMap(sources []Source) (map[string]image.Image, error) {
	sourceMap := make(map[string]image.Image, len(sources))
	for i := range sources {
		source := sources[i]
		if source.ID == "" {
			return nil, fmt.Errorf("%w: empty source ID", ErrSourceNotFound)
		}
		if source.Image == nil {
			return nil, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, source.ID)
		}
		if _, exists := sourceMap[source.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate source ID %q", ErrInvalidItem, source.ID)
		}

		sourceMap[source.ID] = source.Image
	}

	return sourceMap, nil
}

// renderWithSourceMap renders layout from already prepared source map.
func renderWithSourceMap(layout *Layout, sourceMap map[string]image.Image) (image.Image, error) {
	if err := validateLayout(layout); err != nil {
		return nil, err
	}

	atlas := image.NewRGBA(image.Rect(0, 0, layout.Width, layout.Height))

	for i := range layout.Placements {
		placement := layout.Placements[i]
		src, ok := sourceMap[placement.ID]
		if !ok {
			return nil, fmt.Errorf("%w: ID=%q", ErrSourceNotFound, placement.ID)
		}

		sw := src.Bounds().Dx()
		sh := src.Bounds().Dy()
		if sw != placement.Width || sh != placement.Height {
			return nil, fmt.Errorf(
				"%w: ID=%q source=%dx%d placement=%dx%d",
				ErrSourceSizeMismatch,
				placement.ID,
				sw,
				sh,
				placement.Width,
				placement.Height,
			)
		}

		x := placement.X
		y := placement.Y

		if placement.Rotated {
			drawRotated90Into(atlas, x, y, src, placement.Width, placement.Height)
			continue
		}

		draw.Draw(
			atlas,
			image.Rect(x, y, x+placement.Width, y+placement.Height),
			src,
			image.Point{},
			draw.Src,
		)
	}

	return atlas, nil
}

// validateLayout validates render input layout dimensions.
func validateLayout(layout *Layout) error {
	if layout == nil {
		return ErrLayoutRequired
	}
	if layout.Width <= 0 || layout.Height <= 0 {
		return fmt.Errorf(
			"%w: Width=%d Height=%d",
			ErrInvalidLayout,
			layout.Width,
			layout.Height,
		)
	}

	return nil
}
