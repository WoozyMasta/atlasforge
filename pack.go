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
	if opts.TrimTransparent || opts.Deduplicate || opts.ColorKey != nil || opts.Overlap {
		if err := validateOptions(opts); err != nil {
			return nil, err
		}
	}

	items, sourceMap, trimByID, err := prepareSpritesWithOptions(sprites, opts)
	if err != nil {
		return nil, err
	}

	var aliases []aliasRelation
	if opts.Deduplicate {
		items, aliases = deduplicatePreparedSpritesWithOptions(
			items,
			sourceMap,
			trimByID,
			opts,
			nil,
		)
	}

	layout, err := planImageItems(items, trimByID, opts, func(id string) (image.Image, error) {
		source, exists := sourceMap[id]
		if !exists {
			return nil, fmt.Errorf("%w: ID=%q", ErrSourceNotFound, id)
		}

		return source, nil
	})
	if err != nil {
		return nil, err
	}

	applyTrimMetadata(layout, trimByID, opts)

	if len(aliases) > 0 {
		if err := appendAliases(layout, aliases, trimByID); err != nil {
			return nil, err
		}
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

// prepareSpritesWithOptions validates sources and optionally trims by alpha or color key.
func prepareSpritesWithOptions(
	sprites []Sprite,
	opts Options,
) ([]Item, map[string]image.Image, map[string]trimMetadata, error) {
	items := make([]Item, 0, len(sprites))
	sourceMap := make(map[string]image.Image, len(sprites))
	var trimByID map[string]trimMetadata
	if opts.TrimTransparent || opts.ColorKey != nil && opts.ColorKey.Trim {
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

		if opts.TrimTransparent || opts.ColorKey != nil && opts.ColorKey.Trim {
			var metadata trimMetadata
			var hasMetadata bool
			width, height, metadata, hasMetadata = trimmedSourceGeometryWithOptions(sprite.Image, opts)
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
	if !opts.TrimTransparent && opts.ColorKey == nil && !opts.Overlap {
		return settings
	}

	if settings == nil {
		settings = &RenderSettings{Padding: opts.Padding}
	}
	settings.TrimTransparent = opts.TrimTransparent
	if opts.ColorKey != nil {
		colorKey := *opts.ColorKey
		settings.ColorKey = &colorKey
	}

	return settings
}
