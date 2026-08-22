// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func FuzzPlan(f *testing.F) {
	for _, seed := range planFuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		input := decodePlanFuzzInput(data)
		original := make([]Item, len(input.items))
		copy(original, input.items)

		layout, err := Plan(input.items, input.options)
		if !reflect.DeepEqual(input.items, original) {
			t.Fatal("Plan mutated the input items")
		}

		if err != nil {
			if input.wantErr != nil {
				if !errors.Is(err, input.wantErr) {
					t.Fatalf("Plan error = %v, want errors.Is(_, %v)", err, input.wantErr)
				}
				return
			}

			if !errors.Is(err, ErrInvalidOptions) && !errors.Is(err, ErrPlacementFailed) {
				t.Fatalf("unexpected Plan error for decoded input: %v", err)
			}

			return
		}
		if input.wantErr != nil {
			t.Fatalf("Plan succeeded, want errors.Is(_, %v)", input.wantErr)
		}

		assertPlanFuzzInvariants(t, input.items, input.options, layout)

		repeated, err := Plan(input.items, input.options)
		if err != nil {
			t.Fatalf("repeated Plan: %v", err)
		}
		if !reflect.DeepEqual(layout, repeated) {
			t.Fatalf("Plan is not deterministic:\nfirst:  %+v\nsecond: %+v", layout, repeated)
		}
	})
}

func TestPlanFuzzSeedsHaveBaselineSuccesses(t *testing.T) {
	t.Parallel()

	seeds := planFuzzSuccessSeeds()
	successes := 0
	for i, seed := range seeds {
		input := decodePlanFuzzInput(seed)
		if input.wantErr != nil {
			t.Fatalf("success seed %d decodes as invalid", i)
		}

		layout, err := Plan(input.items, input.options)
		if err != nil {
			t.Fatalf("success seed %d: Plan: %v", i, err)
		}
		assertPlanFuzzInvariants(t, input.items, input.options, layout)
		successes++
	}

	if successes*100/len(seeds) < 90 {
		t.Fatalf("successful baseline seeds = %d/%d, want at least 90%%", successes, len(seeds))
	}
}

// planFuzzInput contains bounded decoded data and an optional expected error category.
type planFuzzInput struct {
	items   []Item
	options Options
	wantErr error
}

// decodePlanFuzzInput decodes at most 256 bytes into small valid or invalid cases.
func decodePlanFuzzInput(data []byte) planFuzzInput {
	if len(data) > 256 {
		data = data[:256]
	}
	byteAt := func(index int) byte {
		if index < len(data) {
			return data[index]
		}

		return 0
	}

	mode := byteAt(0) % 5
	if mode == 3 {
		maxInt := int(^uint(0) >> 1)
		return planFuzzInput{
			items: []Item{{ID: "edge", Width: maxInt, Height: 1}},
			options: Options{
				MinSize: 1,
				MaxSize: maxInt,
				Padding: maxInt,
			},
			wantErr: ErrInvalidOptions,
		}
	}
	if mode == 4 {
		if byteAt(1)%2 == 0 {
			return planFuzzInput{
				items: []Item{{ID: "wide", Width: 40, Height: 20}},
				options: Options{
					MinSize:       16,
					MaxSize:       128,
					AspectPenalty: 0.25,
				},
			}
		}

		return planFuzzInput{
			items: []Item{{ID: "square", Width: 33, Height: 33}},
			options: Options{
				MinSize: 16,
				MaxSize: 48,
			},
		}
	}

	penalties := [...]float64{0, 0.25, 1, 4}
	flags := byteAt(2)
	input := planFuzzInput{
		options: Options{
			MinSize:       8 + int(byteAt(5)%57),
			MaxSize:       64 + int(byteAt(6)%193),
			Padding:       int(byteAt(4) % 5),
			AspectPenalty: penalties[byteAt(7)%byte(len(penalties))],
			Heuristic:     Heuristic(byteAt(3)),
			AllowRotate:   flags&1 != 0,
			ForceSquare:   flags&2 != 0,
			PreferHeight:  flags&4 != 0,
		},
	}
	if input.options.Heuristic > HeuristicFirstFit {
		input.options.Heuristic = Heuristic(999)
	}
	count := int(byteAt(1) % 17)
	input.items = make([]Item, count)
	for i := range input.items {
		input.items[i] = Item{
			ID:     string(rune('a' + i)),
			Width:  1 + int(byteAt(8+2*i)%64),
			Height: 1 + int(byteAt(9+2*i)%64),
		}
	}

	switch mode {
	case 1:
		input.items = []Item{{
			ID:     "a",
			Width:  1 + int(byteAt(8)%8),
			Height: 1 + int(byteAt(9)%8),
		}}

		switch byteAt(1) % 4 {
		case 0:
			input.items[0].ID = ""

		case 1:
			input.items[0].Width = 0

		case 2:
			input.items[0].Height = -1

		case 3:
			if len(input.items) == 1 {
				input.items = append(input.items, input.items[0])
			}
			input.items[1].ID = input.items[0].ID
		}

		input.wantErr = ErrInvalidItem

	case 2:
		switch byteAt(1) % 4 {
		case 0:
			input.options.Padding = -1

		case 1:
			input.options.MinSize = input.options.MaxSize + 1

		case 2:
			input.options.AspectPenalty = math.NaN()

		case 3:
			switch byteAt(2) % 3 {
			case 0:
				input.options.AspectPenalty = math.Inf(1)

			case 1:
				input.options.AspectPenalty = math.Inf(-1)

			case 2:
				input.options.AspectPenalty = -0.01
			}
		}

		input.wantErr = ErrInvalidOptions
	}

	return input
}

// planFuzzSeeds combines successful geometry and invalid-input seeds.
func planFuzzSeeds() [][]byte {
	seeds := planFuzzSuccessSeeds()
	seeds = append(seeds,
		[]byte{},
		planFuzzSeed(1, 0, 0, 0, 0, 0, 0),
		planFuzzSeed(1, 1, 0, 0, 0, 0, 0),
		planFuzzSeed(1, 2, 0, 0, 0, 0, 0),
		planFuzzSeed(1, 3, 0, 0, 0, 0, 0),
		planFuzzSeed(2, 0, 0, 0, 0, 0, 0),
		planFuzzSeed(2, 1, 0, 0, 0, 0, 0),
		planFuzzSeed(2, 2, 0, 0, 0, 0, 0),
		planFuzzSeed(2, 3, 0, 0, 0, 0, 0),
		planFuzzSeed(2, 3, 1, 0, 0, 0, 0),
		planFuzzSeed(2, 3, 2, 0, 0, 0, 0),
		planFuzzSeed(0, 1, 0, 255, 0, 0, 0, 7, 7),
		[]byte{3},
		[]byte{4, 0},
		[]byte{4, 1},
	)

	return seeds
}

// planFuzzSuccessSeeds contains cases expected to produce successful layouts.
func planFuzzSuccessSeeds() [][]byte {
	seeds := make([][]byte, 0, 18)
	seeds = append(seeds,
		planFuzzSeed(0, 0, 0, 0, 0, 48, 0),
		planFuzzSeed(0, 1, 0, 0, 0, 0, 1, 9, 11),
		planFuzzSeed(0, 2, 0, 0, 0, 0, 0, 7, 7, 7, 7),
		planFuzzSeed(0, 1, 0, 0, 0, 0, 63, 0),
		planFuzzSeed(0, 1, 0, 0, 0, 0, 0, 63),
		planFuzzSeed(0, 2, 1, 0, 4, 0, 39, 19, 19, 39),
		planFuzzSeed(0, 2, 2, 1, 0, 0, 1, 29, 19, 19, 29),
		planFuzzSeed(0, 1, 4, 0, 1, 0, 0, 7, 7),
		planFuzzSeed(0, 1, 0, 0, 4, 0, 0, 7, 7),
		[]byte{4, 0},
		[]byte{4, 1},
	)

	unitItems := make([]byte, 32)
	seeds = append(seeds, planFuzzSeed(0, 16, 0, 0, 0, 0, 0, unitItems...))
	for heuristic := range 6 {
		seeds = append(seeds, planFuzzSeed(0, 1, 0, byte(heuristic), 0, 0, 0, 11, 4))
	}

	return seeds
}

// planFuzzSeed encodes the option prefix followed by width/height byte pairs.
func planFuzzSeed(mode, count, flags, heuristic, padding, maxSize, penalty byte, dimensions ...byte) []byte {
	seed := make([]byte, 0, 8+len(dimensions))
	seed = append(seed, mode, count, flags, heuristic, padding, 8, maxSize, penalty)
	return append(seed, dimensions...)
}

// assertPlanFuzzInvariants checks output geometry without using MaxRects helpers.
func assertPlanFuzzInvariants(t *testing.T, items []Item, opts Options, layout *Layout) {
	t.Helper()
	if layout == nil {
		t.Fatal("Plan returned a nil layout without an error")
	}

	if layout.Width < opts.MinSize || layout.Height < opts.MinSize ||
		layout.Width > opts.MaxSize || layout.Height > opts.MaxSize {
		t.Fatalf("layout dimensions %dx%d outside %d..%d", layout.Width, layout.Height, opts.MinSize, opts.MaxSize)
	}

	if opts.ForceSquare && layout.Width != layout.Height {
		t.Fatalf("ForceSquare layout is not square: %dx%d", layout.Width, layout.Height)
	}
	if len(layout.Placements) != len(items) {
		t.Fatalf("placements = %d, want %d", len(layout.Placements), len(items))
	}

	itemsByID := make(map[string]Item, len(items))
	for _, item := range items {
		if _, exists := itemsByID[item.ID]; exists {
			t.Fatalf("input has duplicate ID %q", item.ID)
		}
		itemsByID[item.ID] = item
	}
	seen := make(map[string]struct{}, len(items))
	rects := make([][4]int, len(layout.Placements))
	for i, placement := range layout.Placements {
		item, exists := itemsByID[placement.ID]
		if !exists {
			t.Fatalf("unexpected placement ID %q", placement.ID)
		}
		if _, exists := seen[placement.ID]; exists {
			t.Fatalf("duplicate placement ID %q", placement.ID)
		}
		seen[placement.ID] = struct{}{}
		if placement.Width != item.Width || placement.Height != item.Height {
			t.Fatalf(
				"placement %q dimensions %dx%d, want %dx%d",
				placement.ID, placement.Width, placement.Height, item.Width, item.Height,
			)
		}
		if placement.Rotated && !opts.AllowRotate {
			t.Fatalf("placement %q is rotated while rotation is disabled", placement.ID)
		}

		width, height := placement.Width, placement.Height
		if placement.Rotated {
			width, height = height, width
		}
		x := placement.X - opts.Padding
		y := placement.Y - opts.Padding
		width += 2 * opts.Padding
		height += 2 * opts.Padding
		if x < 0 || y < 0 || x+width > layout.Width || y+height > layout.Height {
			t.Fatalf("padded placement %q outside atlas: %+v", placement.ID, placement)
		}

		rects[i] = [4]int{x, y, width, height}
	}

	if len(seen) != len(itemsByID) {
		t.Fatalf("placement IDs = %d, want %d", len(seen), len(itemsByID))
	}

	for i := range len(rects) {
		for j := i + 1; j < len(rects); j++ {
			a, b := rects[i], rects[j]
			if a[0] < b[0]+b[2] && b[0] < a[0]+a[2] &&
				a[1] < b[1]+b[3] && b[1] < a[1]+a[3] {
				t.Fatalf("padded placements %q and %q intersect", layout.Placements[i].ID, layout.Placements[j].ID)
			}
		}
	}
}
