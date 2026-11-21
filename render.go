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
	return RenderWithOptions(layout, sources, RenderOptions{})
}

// RenderWithOptions renders a layout with caller-supplied resource limits.
// MaxBytes limits only the RGBA output buffer and is unlimited when zero.
func RenderWithOptions(layout *Layout, sources []Source, options RenderOptions) (image.Image, error) {
	if err := validateLayout(layout); err != nil {
		return nil, err
	}
	if err := validateRenderBufferLimit(layout, options.MaxBytes); err != nil {
		return nil, err
	}

	sourceMap, err := buildSourceMap(sources)
	if err != nil {
		return nil, err
	}

	if err := validatePlacementSources(layout, sourceMap); err != nil {
		return nil, err
	}

	return renderPrepared(layout, sourceMap, nil)
}

// RenderWithLoader renders a layout by loading each source immediately before drawing it.
// Images are not cached; the loader should return a valid image on every call.
func RenderWithLoader(layout *Layout, load ImageLoader) (image.Image, error) {
	return RenderWithLoaderOptions(layout, load, RenderOptions{})
}

// RenderWithLoaderOptions renders with lazy sources and caller-supplied resource limits.
// MaxBytes limits only the RGBA output buffer and is unlimited when zero.
func RenderWithLoaderOptions(
	layout *Layout,
	load ImageLoader,
	options RenderOptions,
) (image.Image, error) {
	if load == nil {
		return nil, ErrImageLoaderRequired
	}
	if err := validateLayout(layout); err != nil {
		return nil, err
	}
	if err := validateRenderBufferLimit(layout, options.MaxBytes); err != nil {
		return nil, err
	}

	return renderPrepared(layout, nil, load)
}

// validateRenderBufferLimit rejects invalid limits and oversized RGBA outputs.
func validateRenderBufferLimit(layout *Layout, maxBytes int64) error {
	if maxBytes < 0 {
		return fmt.Errorf("%w: render MaxBytes must be non-negative: %d", ErrInvalidOptions, maxBytes)
	}
	if maxBytes == 0 {
		return nil
	}
	if layout == nil {
		return ErrLayoutRequired
	}
	if layout.Width <= 0 || layout.Height <= 0 {
		return fmt.Errorf("%w: Width=%d Height=%d", ErrInvalidLayout, layout.Width, layout.Height)
	}

	area, err := checkedArea(int64(layout.Width), int64(layout.Height))
	if err != nil {
		return fmt.Errorf("%w: RGBA dimensions overflow", ErrInvalidLayout)
	}

	maxInt64 := int64(^uint64(0) >> 1)
	if area > maxInt64/4 {
		return fmt.Errorf("%w: RGBA buffer size overflows", ErrInvalidLayout)
	}

	bufferBytes := area * 4
	if bufferBytes > maxBytes {
		return fmt.Errorf(
			"%w: required %d bytes, limit is %d",
			ErrRenderLimitExceeded,
			bufferBytes,
			maxBytes,
		)
	}

	return nil
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
	if err := validatePlacementSources(layout, sourceMap); err != nil {
		return nil, err
	}

	return renderPrepared(layout, sourceMap, nil)
}

// renderPrepared draws validated layout geometry using eager or lazy source access.
func renderPrepared(layout *Layout, sourceMap map[string]image.Image, load ImageLoader) (image.Image, error) {
	atlas := image.NewRGBA(image.Rect(0, 0, layout.Width, layout.Height))
	extrude := 0
	if layout.Render != nil {
		extrude = layout.Render.Extrude
	}

	normalizeTransparent := layout.Render != nil && layout.Render.TrimTransparent
	var colorKey *ColorKeyOptions
	if layout.Render != nil {
		colorKey = layout.Render.ColorKey
	}

	groups := make(map[int][]Placement)
	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.AliasOf == "" && placement.OverlapGroup > 0 {
			groups[placement.OverlapGroup] = append(groups[placement.OverlapGroup], placement)
		}
	}

	blocks := make(map[int]OverlapBlock, len(layout.OverlapBlocks))
	for _, block := range layout.OverlapBlocks {
		blocks[block.ID] = block
	}

	renderedGroups := make(map[int]struct{}, len(groups))

	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.AliasOf != "" {
			continue
		}

		if placement.OverlapGroup > 0 {
			if _, rendered := renderedGroups[placement.OverlapGroup]; rendered {
				continue
			}

			members := groups[placement.OverlapGroup]
			block := blocks[placement.OverlapGroup]
			if err := renderOverlapBlock(layout, atlas, block, members, sourceMap, load, extrude); err != nil {
				return nil, err
			}

			renderedGroups[placement.OverlapGroup] = struct{}{}
			continue
		}

		src := sourceMap[placement.ID]
		if load != nil {
			var err error
			src, err = load(placement.ID)
			if err != nil {
				return nil, fmt.Errorf("load source %q: %w", placement.ID, err)
			}
			if isNilImage(src) {
				return nil, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, placement.ID)
			}
			if err := validatePlacementSource(placement, src); err != nil {
				return nil, err
			}
		}

		if placement.OriginalWidth > 0 || normalizeTransparent || colorKey != nil && colorKey.ToAlpha {
			src = sourceForPlacementWithColorKey(placement, src, normalizeTransparent, colorKey)
		}

		x := placement.X
		y := placement.Y

		if placement.Rotated {
			drawRotated90Into(atlas, x, y, src, placement.Width, placement.Height)
		} else if colorKeyView, ok := src.(colorKeyImageView); ok {
			drawColorKeyViewInto(atlas, x, y, colorKeyView, placement.Width, placement.Height)
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

// renderOverlapBlock fills, validates, draws, and extrudes one N-member zone.
func renderOverlapBlock(
	layout *Layout,
	atlas *image.RGBA,
	block OverlapBlock,
	members []Placement,
	sourceMap map[string]image.Image,
	load ImageLoader,
	extrude int,
) error {
	blockBounds := image.Rect(block.X, block.Y, block.X+block.Width, block.Y+block.Height)
	draw.Draw(atlas, blockBounds, &image.Uniform{C: unpackRGBA8(block.Color)}, image.Point{}, draw.Src)
	area, err := checkedArea(int64(block.Width), int64(block.Height))

	if err != nil || area > int64(int(^uint(0)>>1))-7 {
		return fmt.Errorf("%w: overlap block %d coverage size overflows", ErrInvalidLayout, block.ID)
	}

	coverage := make([]byte, int((area+7)/8))
	padding := 0
	var colorKey *ColorKeyOptions
	trimTransparent := false
	if layout.Render != nil {
		padding = layout.Render.Padding
		trimTransparent = layout.Render.TrimTransparent
		colorKey = layout.Render.ColorKey
	}

	for _, placement := range members {
		source, loadErr := loadRenderSource(placement, sourceMap, load)
		if loadErr != nil {
			return loadErr
		}

		view := sourceForPlacementWithColorKey(placement, source, trimTransparent, colorKey)
		if view.Bounds().Dx() != placement.Width || view.Bounds().Dy() != placement.Height {
			return fmt.Errorf(
				"%w: overlap block %d source %q has wrong prepared size",
				ErrSourceSizeMismatch, block.ID, placement.ID,
			)
		}

		for side := overlapLeft; side <= overlapBottom; side++ {
			depth, edgeColor := scanUniformBand(view, side)
			minimumDepth := 1
			if side == overlapLeft || side == overlapRight {
				if block.ShareX > 0 {
					minimumDepth = block.ShareX + padding
				}
			} else if block.ShareY > 0 {
				minimumDepth = block.ShareY + padding
			}
			if depth < minimumDepth || edgeColor != block.Color {
				return fmt.Errorf(
					"%w: overlap block %d source %q edge is incompatible",
					ErrInvalidLayout, block.ID, placement.ID,
				)
			}
		}

		bounds := view.Bounds()
		for sourceY := range placement.Height {
			for sourceX := range placement.Width {
				destinationX, destinationY := placement.X+sourceX, placement.Y+sourceY
				if placement.Rotated {
					destinationX = placement.X + placement.Height - 1 - sourceY
					destinationY = placement.Y + sourceX
				}

				pixel := canonicalImageRGBA8(view, bounds.Min.X+sourceX, bounds.Min.Y+sourceY)
				localX, localY := destinationX-block.X, destinationY-block.Y
				index := localY*block.Width + localX
				maskIndex, maskBit := index>>3, byte(1<<uint(index&7))
				pixelOffset := (destinationY-atlas.Rect.Min.Y)*atlas.Stride + (destinationX-atlas.Rect.Min.X)*4

				if coverage[maskIndex]&maskBit != 0 {
					if atlas.Pix[pixelOffset] != pixel.R ||
						atlas.Pix[pixelOffset+1] != pixel.G ||
						atlas.Pix[pixelOffset+2] != pixel.B ||
						atlas.Pix[pixelOffset+3] != pixel.A {
						return fmt.Errorf(
							"%w: overlap block %d pixels differ at (%d,%d)",
							ErrInvalidLayout, block.ID, destinationX, destinationY,
						)
					}
				} else {
					atlas.Pix[pixelOffset] = pixel.R
					atlas.Pix[pixelOffset+1] = pixel.G
					atlas.Pix[pixelOffset+2] = pixel.B
					atlas.Pix[pixelOffset+3] = pixel.A
					coverage[maskIndex] |= maskBit
				}
			}
		}
	}
	if extrude > 0 {
		extrudeRGBA(
			atlas,
			block.X,
			block.Y,
			block.Width,
			block.Height,
			extrude,
		)
	}

	return nil
}

// loadRenderSource obtains one placement image and checks its source dimensions.
func loadRenderSource(
	placement Placement,
	sourceMap map[string]image.Image,
	load ImageLoader,
) (image.Image, error) {
	source := sourceMap[placement.ID]
	if load != nil {
		var err error
		source, err = load(placement.ID)
		if err != nil {
			return nil, fmt.Errorf("load source %q: %w", placement.ID, err)
		}
	}

	if isNilImage(source) {
		return nil, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, placement.ID)
	}
	if err := validatePlacementSource(placement, source); err != nil {
		return nil, err
	}

	return source, nil
}

// drawColorKeyViewInto writes converted pixels directly to the atlas buffer.
func drawColorKeyViewInto(dst *image.RGBA, dstX, dstY int, src colorKeyImageView, width, height int) {
	bounds := src.Bounds()
	stride := dst.Stride
	pixels := dst.Pix
	for y := range height {
		destinationRow := (dstY+y)*stride + dstX*4
		sourceY := bounds.Min.Y + y
		for x := range width {
			pixel := src.RGBAAt(bounds.Min.X+x, sourceY)
			destination := destinationRow + x*4
			pixels[destination] = pixel.R
			pixels[destination+1] = pixel.G
			pixels[destination+2] = pixel.B
			pixels[destination+3] = pixel.A
		}
	}
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

	if layout.Render != nil &&
		layout.Render.ColorKey != nil &&
		!layout.Render.ColorKey.Trim &&
		!layout.Render.ColorKey.ToAlpha {
		return fmt.Errorf("%w: render ColorKey requires Trim or ToAlpha", ErrInvalidLayout)
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

		if placement.OriginalWidth != 0 || placement.OriginalHeight != 0 ||
			placement.OffsetX != 0 || placement.OffsetY != 0 {
			if err := validateTrimMetadata(placement); err != nil {
				return err
			}
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
	if err := validateAliases(layout); err != nil {
		return err
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
		if placement.AliasOf != "" {
			continue
		}

		src, ok := sourceMap[placement.ID]
		if !ok {
			return fmt.Errorf("%w: ID=%q", ErrSourceNotFound, placement.ID)
		}

		if err := validatePlacementSource(placement, src); err != nil {
			return err
		}
	}

	return nil
}

// validatePlacementSource checks one source image against its placement geometry.
func validatePlacementSource(placement Placement, src image.Image) error {
	if isNilImage(src) {
		return fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, placement.ID)
	}

	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	expectedWidth := placement.Width
	expectedHeight := placement.Height
	if placement.OriginalWidth > 0 {
		expectedWidth = placement.OriginalWidth
		expectedHeight = placement.OriginalHeight
	}
	if sw != expectedWidth || sh != expectedHeight {
		if placement.OriginalWidth > 0 {
			return fmt.Errorf(
				"%w: ID=%q source=%dx%d original=%dx%d",
				ErrSourceSizeMismatch,
				placement.ID,
				sw,
				sh,
				expectedWidth,
				expectedHeight,
			)
		}

		return fmt.Errorf(
			"%w: ID=%q source=%dx%d placement=%dx%d",
			ErrSourceSizeMismatch,
			placement.ID,
			sw,
			sh,
			expectedWidth,
			expectedHeight,
		)
	}

	return nil
}

// validateTrimMetadata rejects incomplete or out-of-bounds source crop metadata.
func validateTrimMetadata(placement Placement) error {
	originalWidth := placement.OriginalWidth
	originalHeight := placement.OriginalHeight
	offsetX := placement.OffsetX
	offsetY := placement.OffsetY
	if originalWidth == 0 && originalHeight == 0 {
		if offsetX != 0 || offsetY != 0 {
			return fmt.Errorf(
				"%w: crop offsets require original dimensions: ID=%q",
				ErrInvalidLayout,
				placement.ID,
			)
		}

		return nil
	}

	if originalWidth <= 0 || originalHeight <= 0 ||
		offsetX < 0 || offsetY < 0 ||
		placement.Width > originalWidth || placement.Height > originalHeight ||
		offsetX > originalWidth-placement.Width || offsetY > originalHeight-placement.Height {
		return fmt.Errorf(
			"%w: invalid crop metadata: ID=%q original=%dx%d offset=%d,%d crop=%dx%d",
			ErrInvalidLayout,
			placement.ID,
			originalWidth,
			originalHeight,
			offsetX,
			offsetY,
			placement.Width,
			placement.Height,
		)
	}

	return nil
}
