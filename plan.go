// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
)

// Plan computes deterministic atlas placements without rendering image data.
func Plan(items []Item, opts Options) (*Layout, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	if err := validateItems(items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &Layout{
			Width:      opts.MinSize,
			Height:     opts.MinSize,
			Placements: nil,
		}, nil
	}

	work := make([]Item, len(items))
	copy(work, items)
	sortItemsForPacking(work, opts)

	width, height := findOptimalSize(work, opts)
	if width > opts.MaxSize || height > opts.MaxSize {
		return nil, fmt.Errorf(
			"%w: required atlas size %dx%d exceeds MaxSize=%d",
			ErrInvalidOptions,
			width,
			height,
			opts.MaxSize,
		)
	}

	bin := newMaxRects(width, height, opts.AllowRotate)
	placements := make([]Placement, 0, len(work))
	heuristic := normalizeHeuristic(opts.Heuristic)

	for _, item := range work {
		pw := item.Width + 2*opts.Padding
		ph := item.Height + 2*opts.Padding

		rect, ok := bin.Insert(pw, ph, heuristic)
		if !ok {
			return nil, fmt.Errorf(
				"%w: item %q cannot fit into %dx%d",
				ErrPlacementFailed,
				item.ID,
				width,
				height,
			)
		}

		x := rect.X + opts.Padding
		y := rect.Y + opts.Padding
		placements = append(placements, Placement{
			ID:      item.ID,
			X:       x,
			Y:       y,
			Width:   item.Width,
			Height:  item.Height,
			Rotated: rect.Rotated,
		})
	}

	return &Layout{
		Width:      width,
		Height:     height,
		Placements: placements,
	}, nil
}

// validateOptions validates packing options.
func validateOptions(opts Options) error {
	if opts.MinSize <= 0 || opts.MaxSize <= 0 || opts.MinSize > opts.MaxSize {
		return fmt.Errorf(
			"%w: MinSize=%d MaxSize=%d",
			ErrInvalidOptions,
			opts.MinSize,
			opts.MaxSize,
		)
	}
	if opts.Padding < 0 {
		return fmt.Errorf("%w: Padding=%d", ErrInvalidOptions, opts.Padding)
	}

	return nil
}

// validateItems validates item IDs and dimensions.
func validateItems(items []Item) error {
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		item := items[i]
		if item.ID == "" {
			return fmt.Errorf("%w: empty item ID", ErrInvalidItem)
		}
		if item.Width <= 0 || item.Height <= 0 {
			return fmt.Errorf(
				"%w: item %q has invalid size %dx%d",
				ErrInvalidItem,
				item.ID,
				item.Width,
				item.Height,
			)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("%w: duplicate item ID %q", ErrInvalidItem, item.ID)
		}

		seen[item.ID] = struct{}{}
	}

	return nil
}

// normalizeHeuristic normalizes out-of-range heuristic values.
func normalizeHeuristic(heuristic Heuristic) Heuristic {
	if heuristic < HeuristicBestShortSideFit || heuristic > HeuristicFirstFit {
		return HeuristicBestShortSideFit
	}

	return heuristic
}
