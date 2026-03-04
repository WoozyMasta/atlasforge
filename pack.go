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
	items, sourceMap, err := prepareSprites(sprites)
	if err != nil {
		return nil, err
	}

	layout, err := Plan(items, opts)
	if err != nil {
		return nil, err
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
func prepareSprites(sprites []Sprite) ([]Item, map[string]image.Image, error) {
	items := make([]Item, 0, len(sprites))
	sourceMap := make(map[string]image.Image, len(sprites))
	for i := range sprites {
		sprite := sprites[i]
		if sprite.ID == "" {
			return nil, nil, fmt.Errorf("%w: empty sprite ID", ErrInvalidItem)
		}
		if sprite.Image == nil {
			return nil, nil, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, sprite.ID)
		}

		width := sprite.Width
		height := sprite.Height
		if width <= 0 {
			width = sprite.Image.Bounds().Dx()
		}
		if height <= 0 {
			height = sprite.Image.Bounds().Dy()
		}
		if width <= 0 || height <= 0 {
			return nil, nil, fmt.Errorf(
				"%w: sprite %q has invalid size %dx%d",
				ErrInvalidItem,
				sprite.ID,
				width,
				height,
			)
		}
		if _, exists := sourceMap[sprite.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate sprite ID %q", ErrInvalidItem, sprite.ID)
		}

		items = append(items, Item{
			ID:     sprite.ID,
			Width:  width,
			Height: height,
		})
		sourceMap[sprite.ID] = sprite.Image
	}

	return items, sourceMap, nil
}
