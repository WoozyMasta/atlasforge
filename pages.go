// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"fmt"
)

// PlanPages splits items across deterministic atlas layouts.
//
// Each page is compacted within MinSize and MaxSize when possible.
// An empty input returns a non-nil empty slice.
func PlanPages(items []Item, opts Options) ([]Layout, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	if err := validatePageFitBounds(items, opts); err != nil {
		return nil, err
	}
	if err := validateItems(items); err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return []Layout{}, nil
	}

	work := append([]Item(nil), items...)
	sortItemsForPacking(work, opts)
	pending := work
	pages := make([]Layout, 0)
	heuristic := normalizeHeuristic(opts.Heuristic)

	for len(pending) > 0 {
		used, selected, remaining := packIntoBin(
			pending,
			opts.MaxSize,
			opts.MaxSize,
			opts,
			heuristic,
			true,
		)
		if len(selected) == 0 {
			return nil, fmt.Errorf(
				"%w: no progress placing item %q on a %dpx page",
				ErrPlacementFailed, pending[0].ID, opts.MaxSize,
			)
		}

		if len(pages) == 0 && len(remaining) == 0 {
			layout, err := Plan(items, opts)
			if err == nil {
				return []Layout{*layout}, nil
			}

			if !errors.Is(err, ErrPlacementFailed) {
				return nil, err
			}
		}

		layout, err := Plan(selected, opts)
		if err != nil {
			if !errors.Is(err, ErrPlacementFailed) {
				return nil, err
			}

			layout = layoutFromMaxBin(selected, used, opts)
		}

		pages = append(pages, *layout)
		pending = remaining
	}

	return pages, nil
}

// PackPages plans and renders sprites across one or more atlases.
//
// It retains all source images and all rendered atlas images in memory.
// An empty input returns a non-nil empty slice.
func PackPages(sprites []Sprite, opts Options) ([]Atlas, error) {
	items, sourceMap, err := prepareSprites(sprites)
	if err != nil {
		return nil, err
	}

	layouts, err := PlanPages(items, opts)
	if err != nil {
		return nil, err
	}

	atlases := make([]Atlas, len(layouts))
	for i := range layouts {
		atlasImage, renderErr := renderWithSourceMap(&layouts[i], sourceMap)
		if renderErr != nil {
			return nil, fmt.Errorf("render page %d: %w", i, renderErr)
		}
		atlases[i] = Atlas{Image: atlasImage, Layout: layouts[i]}
	}

	return atlases, nil
}

// BuildPages plans items and renders each page before passing it to consume.
//
// It retains layout metadata but not rendered pages or loaded images between callbacks.
// A consumer that retains atlas images controls its own memory use.
//
// Errors do not roll back pages already passed to the consumer.
func BuildPages(items []Item, opts Options, load ImageLoader, consume AtlasConsumer) error {
	if load == nil {
		return ErrImageLoaderRequired
	}
	if consume == nil {
		return ErrAtlasConsumerRequired
	}

	layouts, err := PlanPages(items, opts)
	if err != nil {
		return err
	}

	for pageIndex := range layouts {
		img, renderErr := RenderWithLoader(&layouts[pageIndex], load)
		if renderErr != nil {
			return fmt.Errorf("render page %d: %w", pageIndex, renderErr)
		}

		atlas := &Atlas{Image: img, Layout: layouts[pageIndex]}
		if consumeErr := consume(pageIndex, atlas); consumeErr != nil {
			return fmt.Errorf("consume page %d: %w", pageIndex, consumeErr)
		}
	}

	return nil
}

// layoutFromMaxBin converts the partial fixed-bin placements to a full-size layout.
func layoutFromMaxBin(items []Item, used []mrRect, opts Options) *Layout {
	placements := make([]Placement, 0, len(items))
	for i, item := range items {
		rect := used[i]
		placements = append(placements, Placement{
			ID:      item.ID,
			X:       rect.X + opts.Padding,
			Y:       rect.Y + opts.Padding,
			Width:   item.Width,
			Height:  item.Height,
			Rotated: rect.Rotated,
		})
	}

	return &Layout{
		Width:      opts.MaxSize,
		Height:     opts.MaxSize,
		Placements: placements,
		Render:     renderSettingsForOptions(opts),
	}
}
