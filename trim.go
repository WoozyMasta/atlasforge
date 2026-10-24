// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"image/color"
)

// transparentPixelImage is the canonical transparent source for empty-alpha sprites.
var transparentPixelImage = image.NewRGBA(image.Rect(0, 0, 1, 1))

// trimMetadata stores the reversible source geometry for one cropped sprite.
type trimMetadata struct {
	// originalWidth is the source width before trimming.
	originalWidth int
	// originalHeight is the source height before trimming.
	originalHeight int
	// offsetX is the crop's horizontal offset from source bounds.
	offsetX int
	// offsetY is the crop's vertical offset from source bounds.
	offsetY int
}

// sourceImageView bounds reads to a crop without allocating a cropped raster.
type sourceImageView struct {
	// source provides the original pixels.
	source image.Image
	// bounds is the visible rectangle in original source coordinates.
	bounds image.Rectangle
}

// imageSubImager exposes native bounded views for standard image types.
type imageSubImager interface {
	SubImage(r image.Rectangle) image.Image
}

// Bounds returns the cropped bounds in the original source coordinate space.
func (view sourceImageView) Bounds() image.Rectangle {
	return view.bounds
}

// ColorModel returns the source image's color model.
func (view sourceImageView) ColorModel() color.Model {
	return view.source.ColorModel()
}

// At reads a source pixel inside the crop and returns transparent outside it.
func (view sourceImageView) At(x, y int) color.Color {
	if !image.Pt(x, y).In(view.bounds) {
		return color.RGBA{}
	}

	return view.source.At(x, y)
}

// alphaBounds returns the smallest rectangle containing nontransparent pixels.
func alphaBounds(src image.Image) (image.Rectangle, bool) {
	bounds := src.Bounds()
	if bounds.Empty() {
		return image.Rectangle{}, false
	}

	switch imageSrc := src.(type) {
	case *image.RGBA:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			return imageSrc.Pix[imageSrc.PixOffset(x, y)+3] != 0
		})

	case *image.NRGBA:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			return imageSrc.Pix[imageSrc.PixOffset(x, y)+3] != 0
		})

	case *image.RGBA64:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			offset := imageSrc.PixOffset(x, y) + 6
			return imageSrc.Pix[offset] != 0 || imageSrc.Pix[offset+1] != 0
		})

	case *image.NRGBA64:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			offset := imageSrc.PixOffset(x, y) + 6
			return imageSrc.Pix[offset] != 0 || imageSrc.Pix[offset+1] != 0
		})

	case *image.Alpha:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			return imageSrc.Pix[imageSrc.PixOffset(x, y)] != 0
		})

	case *image.Alpha16:
		return alphaBoundsBy(bounds, func(x, y int) bool {
			offset := imageSrc.PixOffset(x, y)
			return imageSrc.Pix[offset] != 0 || imageSrc.Pix[offset+1] != 0
		})

	case *image.Paletted:
		return alphaBoundsPaletted(imageSrc)

	case *image.Uniform:
		_, _, _, alpha := imageSrc.C.RGBA()
		if alpha == 0 {
			return image.Rectangle{}, false
		}
		return bounds, true

	case *image.Gray, *image.Gray16, *image.YCbCr, *image.CMYK:
		return bounds, true
	}

	return alphaBoundsBy(bounds, func(x, y int) bool {
		_, _, _, alpha := src.At(x, y).RGBA()
		return alpha != 0
	})
}

// alphaBoundsBy scans alpha without converting a color for each pixel.
func alphaBoundsBy(bounds image.Rectangle, opaqueAt func(x, y int) bool) (image.Rectangle, bool) {
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !opaqueAt(x, y) {
				continue
			}

			found = true
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x+1)
			maxY = max(maxY, y+1)
		}
	}

	if !found {
		return image.Rectangle{}, false
	}

	return image.Rect(minX, minY, maxX, maxY), true
}

// alphaBoundsPaletted checks each palette alpha once before scanning indices.
func alphaBoundsPaletted(src *image.Paletted) (image.Rectangle, bool) {
	opaque := make([]bool, len(src.Palette))
	for i, entry := range src.Palette {
		_, _, _, alpha := entry.RGBA()
		opaque[i] = alpha != 0
	}

	return alphaBoundsBy(src.Bounds(), func(x, y int) bool {
		index := src.Pix[src.PixOffset(x, y)]
		return int(index) < len(opaque) && opaque[index]
	})
}

// sourceForPlacementWithColorKey restores crop geometry and applies key-to-alpha lazily.
func sourceForPlacementWithColorKey(
	placement Placement,
	src image.Image,
	normalizeTransparent bool,
	colorKey *ColorKeyOptions,
) image.Image {
	bounds := src.Bounds()
	cropBounds := bounds
	if placement.OriginalWidth > 0 {
		minX := bounds.Min.X + placement.OffsetX
		minY := bounds.Min.Y + placement.OffsetY
		cropBounds = image.Rect(
			minX,
			minY,
			minX+placement.Width,
			minY+placement.Height,
		)
	}
	if cropBounds == bounds && !normalizeTransparent && (colorKey == nil || !colorKey.ToAlpha) {
		return src
	}

	view := sourceSubImage(src, cropBounds)
	view = colorKeyView(view, colorKey)
	if normalizeTransparent && placement.Width == 1 && placement.Height == 1 {
		if canonicalRGBA8(view.At(cropBounds.Min.X, cropBounds.Min.Y)).A == 0 {
			return transparentPixelImage
		}
	}

	return view
}

// sourceSubImage uses a native bounded view when the image type supports it.
func sourceSubImage(src image.Image, bounds image.Rectangle) image.Image {
	if subImager, ok := src.(imageSubImager); ok {
		view := subImager.SubImage(bounds)
		if view.Bounds() == bounds {
			return view
		}
	}

	return sourceImageView{source: src, bounds: bounds}
}

// trimmedSourceGeometryWithOptions finds content bounds after requested pixel filters.
func trimmedSourceGeometryWithOptions(src image.Image, opts Options) (int, int, trimMetadata, bool) {
	bounds := src.Bounds()
	contentBounds, hasContent := sourceContentBounds(src, opts)
	if !hasContent {
		contentBounds = image.Rect(
			bounds.Min.X,
			bounds.Min.Y,
			bounds.Min.X+1,
			bounds.Min.Y+1,
		)
	}

	if contentBounds == bounds {
		return bounds.Dx(), bounds.Dy(), trimMetadata{}, false
	}

	return contentBounds.Dx(), contentBounds.Dy(), trimMetadata{
		originalWidth:  bounds.Dx(),
		originalHeight: bounds.Dy(),
		offsetX:        contentBounds.Min.X - bounds.Min.X,
		offsetY:        contentBounds.Min.Y - bounds.Min.Y,
	}, true
}

// sourceContentBounds applies transparent and exact color-key trim predicates.
func sourceContentBounds(src image.Image, opts Options) (image.Rectangle, bool) {
	key := opts.ColorKey
	if !opts.TrimTransparent && (key == nil || !key.Trim) {
		return src.Bounds(), !src.Bounds().Empty()
	}

	needsKeyScan := key != nil && (key.Trim || (key.ToAlpha && opts.TrimTransparent))
	if !needsKeyScan {
		return alphaBounds(src)
	}

	keyPixel := canonicalColorKey(key.Color)
	return alphaBoundsBy(src.Bounds(), func(x, y int) bool {
		pixel := canonicalImageRGBA8(src, x, y)
		keyMatch := pixel == keyPixel
		transformedAlpha := pixel.A
		if key.ToAlpha && keyMatch {
			transformedAlpha = 0
		}

		return (!key.Trim || !keyMatch) && (!opts.TrimTransparent || transformedAlpha != 0)
	})
}

// prepareTrimmedItems scans sources once and retains only geometry metadata.
func prepareTrimmedItems(items []Item, opts Options, load ImageLoader) ([]Item, map[string]trimMetadata, error) {
	trimmedItems := append([]Item(nil), items...)
	metadataByID := make(map[string]trimMetadata, len(items))

	for i := range items {
		item := items[i]
		src, err := load(item.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("scan source %q for trimming: %w", item.ID, err)
		}

		if err := validatePlacementSource(Placement{
			ID:     item.ID,
			Width:  item.Width,
			Height: item.Height,
		}, src); err != nil {
			return nil, nil, fmt.Errorf("scan source %q for trimming: %w", item.ID, err)
		}

		width, height, metadata, hasMetadata := trimmedSourceGeometryWithOptions(src, opts)
		trimmedItems[i].Width = width
		trimmedItems[i].Height = height
		if hasMetadata {
			metadataByID[item.ID] = metadata
		}
	}

	return trimmedItems, metadataByID, nil
}

// applyTrimMetadata attaches source geometry and render settings to one layout.
func applyTrimMetadata(layout *Layout, metadataByID map[string]trimMetadata, opts Options) {
	if !opts.TrimTransparent &&
		(opts.ColorKey == nil || !opts.ColorKey.Trim && !opts.ColorKey.ToAlpha) {
		return
	}

	for i := range layout.Placements {
		if metadata, ok := metadataByID[layout.Placements[i].ID]; ok {
			layout.Placements[i].OriginalWidth = metadata.originalWidth
			layout.Placements[i].OriginalHeight = metadata.originalHeight
			layout.Placements[i].OffsetX = metadata.offsetX
			layout.Placements[i].OffsetY = metadata.offsetY
		}
	}

	layout.Render = renderSettingsForImageOptions(opts)
}
