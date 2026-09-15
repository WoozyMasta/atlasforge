// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"image/draw"
	"reflect"
)

// Render renders a precomputed layout using provided image sources.
func Render(layout *Layout, sources []Source) (image.Image, error) {
	if err := validateLayout(layout); err != nil {
		return nil, err
	}

	sourceMap, err := buildSourceMap(sources)
	if err != nil {
		return nil, err
	}

	return renderValidatedWithSourceMap(layout, sourceMap)
}

// isNilImage detects nil interfaces and nil pointers stored in image.Image.
func isNilImage(img image.Image) bool {
	if img == nil {
		return true
	}

	value := reflect.ValueOf(img)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// buildSourceMap validates sources and builds ID->image map.
func buildSourceMap(sources []Source) (map[string]image.Image, error) {
	sourceMap := make(map[string]image.Image, len(sources))
	for i := range sources {
		source := sources[i]
		if source.ID == "" {
			return nil, fmt.Errorf("%w: empty source ID", ErrSourceNotFound)
		}
		if isNilImage(source.Image) {
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

	return renderValidatedWithSourceMap(layout, sourceMap)
}

// renderValidatedWithSourceMap renders a layout after its geometry is validated.
func renderValidatedWithSourceMap(layout *Layout, sourceMap map[string]image.Image) (image.Image, error) {
	if err := validatePlacementSources(layout, sourceMap); err != nil {
		return nil, err
	}

	atlas := image.NewRGBA(image.Rect(0, 0, layout.Width, layout.Height))
	extrude := 0
	if layout.Render != nil {
		extrude = layout.Render.Extrude
	}

	for i := range layout.Placements {
		placement := layout.Placements[i]
		src := sourceMap[placement.ID]

		x := placement.X
		y := placement.Y

		if placement.Rotated {
			drawRotated90Into(atlas, x, y, src, placement.Width, placement.Height)
		} else {
			draw.Draw(
				atlas,
				image.Rect(x, y, x+placement.Width, y+placement.Height),
				src,
				src.Bounds().Min,
				draw.Src,
			)
		}

		if extrude > 0 {
			width := placement.Width
			height := placement.Height
			if placement.Rotated {
				width, height = height, width
			}

			extrudeRGBA(atlas, x, y, width, height, extrude)
		}
	}

	return atlas, nil
}

// extrudeRGBA copies each edge pixel into a bounded RGBA margin around content.
func extrudeRGBA(atlas *image.RGBA, x, y, width, height, depth int) {
	stride := atlas.Stride
	pixels := atlas.Pix

	for dy := -depth; dy < height+depth; dy++ {
		for dx := -depth; dx < width+depth; dx++ {
			if dx >= 0 && dx < width && dy >= 0 && dy < height {
				continue
			}

			sourceX := dx
			if sourceX < 0 {
				sourceX = 0
			} else if sourceX >= width {
				sourceX = width - 1
			}

			sourceY := dy
			if sourceY < 0 {
				sourceY = 0
			} else if sourceY >= height {
				sourceY = height - 1
			}

			sourceOffset := (y+sourceY)*stride + (x+sourceX)*4
			destinationOffset := (y+dy)*stride + (x+dx)*4
			copy(pixels[destinationOffset:destinationOffset+4], pixels[sourceOffset:sourceOffset+4])
		}
	}
}

// validateLayout validates atlas dimensions and placement geometry for rendering.
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

	if layout.Render != nil &&
		(layout.Render.Padding < 0 ||
			layout.Render.Extrude < 0 ||
			layout.Render.Extrude > layout.Render.Padding) {
		return fmt.Errorf(
			"%w: render settings require 0 <= Extrude <= Padding, got Extrude=%d Padding=%d",
			ErrInvalidLayout,
			layout.Render.Extrude,
			layout.Render.Padding,
		)
	}

	maxInt := int(^uint(0) >> 1)
	if layout.Width > maxInt/4 || layout.Height > maxInt/(layout.Width*4) {
		return fmt.Errorf(
			"%w: RGBA dimensions are not representable: Width=%d Height=%d",
			ErrInvalidLayout,
			layout.Width,
			layout.Height,
		)
	}

	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.Width <= 0 || placement.Height <= 0 || placement.X < 0 || placement.Y < 0 {
			return fmt.Errorf(
				"%w: invalid placement geometry: ID=%q X=%d Y=%d Width=%d Height=%d",
				ErrInvalidLayout,
				placement.ID,
				placement.X,
				placement.Y,
				placement.Width,
				placement.Height,
			)
		}

		effectiveWidth := placement.Width
		effectiveHeight := placement.Height
		if placement.Rotated {
			effectiveWidth, effectiveHeight = effectiveHeight, effectiveWidth
		}

		// Check coordinates before subtraction so both bounds tests stay safe.
		if placement.X >= layout.Width || placement.Y >= layout.Height ||
			effectiveWidth > layout.Width-placement.X || effectiveHeight > layout.Height-placement.Y {
			return fmt.Errorf(
				"%w: placement exceeds atlas bounds: ID=%q X=%d Y=%d Width=%d Height=%d Rotated=%t",
				ErrInvalidLayout,
				placement.ID,
				placement.X,
				placement.Y,
				placement.Width,
				placement.Height,
				placement.Rotated,
			)
		}
	}

	if layout.Render != nil && layout.Render.Extrude > 0 {
		if err := validateExtrusionBounds(layout); err != nil {
			return err
		}
	}

	return nil
}

// validateExtrusionBounds ensures all reserved margins fit inside the atlas.
func validateExtrusionBounds(layout *Layout) error {
	padding := layout.Render.Padding
	for i := range layout.Placements {
		placement := layout.Placements[i]
		width, height := placement.Width, placement.Height
		if placement.Rotated {
			width, height = height, width
		}

		if padding > placement.X ||
			padding > placement.Y ||
			padding > layout.Width-placement.X-width ||
			padding > layout.Height-placement.Y-height {
			return fmt.Errorf(
				"%w: placement padding exceeds atlas bounds: ID=%q Padding=%d",
				ErrInvalidLayout,
				placement.ID,
				padding,
			)
		}
	}

	return nil
}

// validatePlacementSources checks source availability and original dimensions before rendering.
func validatePlacementSources(layout *Layout, sourceMap map[string]image.Image) error {
	for i := range layout.Placements {
		placement := layout.Placements[i]
		src, ok := sourceMap[placement.ID]
		if !ok {
			return fmt.Errorf("%w: ID=%q", ErrSourceNotFound, placement.ID)
		}

		sw := src.Bounds().Dx()
		sh := src.Bounds().Dy()
		if sw != placement.Width || sh != placement.Height {
			return fmt.Errorf(
				"%w: ID=%q source=%dx%d placement=%dx%d",
				ErrSourceSizeMismatch,
				placement.ID,
				sw,
				sh,
				placement.Width,
				placement.Height,
			)
		}
	}

	return nil
}
