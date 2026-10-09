// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
)

// Pack plans and renders atlas image in a single call.
func Pack(sprites []Sprite, opts Options) (*Atlas, error) {
	if opts.TrimTransparent {
		if err := validateOptions(opts); err != nil {
			return nil, err
		}
	}

	items, sourceMap, trimByID, err := prepareSprites(sprites, opts.TrimTransparent)
	if err != nil {
		return nil, err
	}

	layout, err := Plan(items, opts)
	if err != nil {
		return nil, err
	}

	if opts.TrimTransparent {
		applyTrimMetadata(layout, trimByID, opts)
	}

	atlasImage, err := renderWithSourceMap(layout, sourceMap)
	if err != nil {
		return nil, err
	}

	return &Atlas{
		Image:  atlasImage,
		Layout: *layout,
	}, nil
}

// prepareSprites validates sprite input and builds planner items/source map.
func prepareSprites(sprites []Sprite, trim bool) ([]Item, map[string]image.Image, map[string]trimMetadata, error) {
	items := make([]Item, 0, len(sprites))
	sourceMap := make(map[string]image.Image, len(sprites))
	var trimByID map[string]trimMetadata
	if trim {
		trimByID = make(map[string]trimMetadata, len(sprites))
	}

	for i := range sprites {
		sprite := sprites[i]
		if sprite.ID == "" {
			return nil, nil, nil, fmt.Errorf("%w: empty sprite ID", ErrInvalidItem)
		}
		if isNilImage(sprite.Image) {
			return nil, nil, nil, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, sprite.ID)
		}

		imageWidth := sprite.Image.Bounds().Dx()
		imageHeight := sprite.Image.Bounds().Dy()
		if sprite.Width > 0 && sprite.Width != imageWidth {
			return nil, nil, nil, fmt.Errorf(
				"%w: sprite %q width override=%d image=%d",
				ErrSourceSizeMismatch,
				sprite.ID,
				sprite.Width,
				imageWidth,
			)
		}
		if sprite.Height > 0 && sprite.Height != imageHeight {
			return nil, nil, nil, fmt.Errorf(
				"%w: sprite %q height override=%d image=%d",
				ErrSourceSizeMismatch,
				sprite.ID,
				sprite.Height,
				imageHeight,
			)
		}

		width := sprite.Width
		height := sprite.Height
		if width <= 0 {
			width = imageWidth
		}
		if height <= 0 {
			height = imageHeight
		}
		if width <= 0 || height <= 0 {
			return nil, nil, nil, fmt.Errorf(
				"%w: sprite %q has invalid size %dx%d",
				ErrInvalidItem,
				sprite.ID,
				width,
				height,
			)
		}

		if _, exists := sourceMap[sprite.ID]; exists {
			return nil, nil, nil, fmt.Errorf("%w: duplicate sprite ID %q", ErrInvalidItem, sprite.ID)
		}

		if trim {
			var metadata trimMetadata
			var hasMetadata bool
			width, height, metadata, hasMetadata = trimmedSourceGeometry(sprite.Image)
			if hasMetadata {
				trimByID[sprite.ID] = metadata
			}
		}

		items = append(items, Item{
			ID:     sprite.ID,
			Width:  width,
			Height: height,
		})
		sourceMap[sprite.ID] = sprite.Image
	}

	return items, sourceMap, trimByID, nil
}

// renderSettingsForImageOptions records settings needed by image-based packing.
func renderSettingsForImageOptions(opts Options) *RenderSettings {
	settings := renderSettingsForOptions(opts)
	if !opts.TrimTransparent {
		return settings
	}

	if settings == nil {
		settings = &RenderSettings{Padding: opts.Padding}
	}
	settings.TrimTransparent = true

	return settings
}
