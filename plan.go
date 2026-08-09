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
	if err := validateFitBounds(items, opts); err != nil {
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

	width, height, found := findOptimalSize(work, opts)
	if !found {
		return nil, fmt.Errorf(
			"%w: no atlas candidate fits within MinSize=%d MaxSize=%d",
			ErrPlacementFailed,
			opts.MinSize,
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

	maxInt := int(^uint(0) >> 1)
	if opts.MaxSize > maxInt/opts.MaxSize {
		return fmt.Errorf(
			"%w: MaxSize squared exceeds representable int area: MaxSize=%d",
			ErrInvalidOptions,
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

// validateFitBounds performs cheap pre-checks before heuristic search.
func validateFitBounds(items []Item, opts Options) error {
	maxAtlasArea := opts.MaxSize * opts.MaxSize
	totalPaddedArea := 0

	for i := range items {
		item := items[i]
		if item.ID == "" || item.Width <= 0 || item.Height <= 0 {
			continue
		}

		paddedW, ok := paddedDimension(item.Width, opts.Padding)
		if !ok {
			return fmt.Errorf("%w: padded width overflows int for item %q", ErrInvalidOptions, item.ID)
		}
		paddedH, ok := paddedDimension(item.Height, opts.Padding)
		if !ok {
			return fmt.Errorf("%w: padded height overflows int for item %q", ErrInvalidOptions, item.ID)
		}
		if paddedW > opts.MaxSize || paddedH > opts.MaxSize {
			return fmt.Errorf(
				"%w: item %q padded size %dx%d exceeds MaxSize=%d",
				ErrInvalidOptions,
				item.ID,
				paddedW,
				paddedH,
				opts.MaxSize,
			)
		}

		itemArea := paddedW * paddedH
		if itemArea > maxAtlasArea-totalPaddedArea {
			return fmt.Errorf(
				"%w: total padded area exceeds max atlas area %d",
				ErrInvalidOptions,
				maxAtlasArea,
			)
		}
		totalPaddedArea += itemArea
	}

	return nil
}

// paddedDimension returns size plus two padding margins when the result fits int.
func paddedDimension(size, padding int) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	if size <= 0 || padding < 0 || padding > (maxInt-size)/2 {
		return 0, false
	}

	return size + 2*padding, true
}

// normalizeHeuristic normalizes out-of-range heuristic values.
func normalizeHeuristic(heuristic Heuristic) Heuristic {
	if heuristic < HeuristicBestShortSideFit || heuristic > HeuristicFirstFit {
		return HeuristicBestShortSideFit
	}

	return heuristic
}
