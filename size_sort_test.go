// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"math"
	"reflect"
	"testing"
)

func TestPackIntoBinContinuesAfterAnItemDoesNotFit(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "too-wide", Width: 11, Height: 2},
		{ID: "fits", Width: 4, Height: 4},
		{ID: "too-tall", Width: 2, Height: 11},
	}
	wantItems := append([]Item(nil), items...)

	placements, placed, remaining := packIntoBin(
		items,
		10,
		10,
		Options{},
		HeuristicBestShortSideFit,
		true,
	)
	if len(placements) != 1 || placements[0] != (mrRect{X: 0, Y: 0, W: 4, H: 4}) {
		t.Fatalf("placements = %+v, want one 4x4 placement at origin", placements)
	}
	if !reflect.DeepEqual(remaining, []Item{items[0], items[2]}) {
		t.Fatalf("remaining = %+v, want [%q %q]", remaining, items[0].ID, items[2].ID)
	}
	if !reflect.DeepEqual(placed, []Item{items[1]}) {
		t.Fatalf("placed = %+v, want [%q]", placed, items[1].ID)
	}
	if !reflect.DeepEqual(items, wantItems) {
		t.Fatalf("input items mutated: got %+v, want %+v", items, wantItems)
	}
}

func TestPackIntoBinHandlesEmptySingleAndRotatedItems(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		placements, placed, remaining := packIntoBin(
			nil, 8, 8, Options{}, HeuristicBestShortSideFit, true,
		)
		if len(placements) != 0 || len(placed) != 0 || len(remaining) != 0 {
			t.Fatalf(
				"empty result = placements %v, placed %v, remaining %v",
				placements, placed, remaining,
			)
		}
	})

	t.Run("single rotated item", func(t *testing.T) {
		t.Parallel()

		placements, placed, remaining := packIntoBin(
			[]Item{{ID: "rotated", Width: 7, Height: 4}},
			4,
			8,
			Options{AllowRotate: true},
			HeuristicBestShortSideFit,
			true,
		)
		if len(remaining) != 0 || len(placed) != 1 || placed[0].ID != "rotated" {
			t.Fatalf("placed = %+v, remaining = %+v; want one placed item", placed, remaining)
		}
		if len(placements) != 1 || placements[0] != (mrRect{X: 0, Y: 0, W: 4, H: 7, Rotated: true}) {
			t.Fatalf("placements = %+v, want one rotated 4x7 placement", placements)
		}
	})
}

func TestPackIntoBinStopsAtFirstFailureWhenRequested(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "too-wide", Width: 11, Height: 2},
		{ID: "fits", Width: 4, Height: 4},
	}
	placements, placed, remaining := packIntoBin(
		items,
		10,
		10,
		Options{},
		HeuristicBestShortSideFit,
		false,
	)
	if len(placements) != 0 {
		t.Fatalf("placements = %+v, want none", placements)
	}
	if len(placed) != 0 {
		t.Fatalf("placed = %+v, want none", placed)
	}
	if len(remaining) != len(items) || &remaining[0] != &items[0] {
		t.Fatalf("remaining = %+v, want original input suffix", remaining)
	}
}

func TestPlanSearchesIndependentSizeCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		item  Item
		opts  Options
		wantW int
		wantH int
	}{
		{
			name:  "wide rectangle",
			item:  Item{ID: "a", Width: 40, Height: 20},
			opts:  Options{MinSize: 16, MaxSize: 128, AspectPenalty: 0.25},
			wantW: 64,
			wantH: 32,
		},
		{
			name:  "tall rectangle",
			item:  Item{ID: "a", Width: 20, Height: 40},
			opts:  Options{MinSize: 16, MaxSize: 128, AspectPenalty: 0.25},
			wantW: 32,
			wantH: 64,
		},
		{
			name:  "non-power-of-two maximum",
			item:  Item{ID: "a", Width: 33, Height: 33},
			opts:  Options{MinSize: 16, MaxSize: 48},
			wantW: 48,
			wantH: 48,
		},
		{
			name:  "non-power-of-two minimum and maximum",
			item:  Item{ID: "a", Width: 40, Height: 40},
			opts:  Options{MinSize: 24, MaxSize: 60},
			wantW: 48,
			wantH: 48,
		},
		{
			name:  "fixed minimum and maximum",
			item:  Item{ID: "a", Width: 12, Height: 8},
			opts:  Options{MinSize: 32, MaxSize: 32},
			wantW: 32,
			wantH: 32,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout, err := Plan([]Item{test.item}, test.opts)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if layout.Width != test.wantW || layout.Height != test.wantH {
				t.Fatalf(
					"layout size = %dx%d, want %dx%d",
					layout.Width, layout.Height, test.wantW, test.wantH,
				)
			}
		})
	}
}

func TestPlanAllowsExtremeRectangularAtlasCandidates(t *testing.T) {
	t.Parallel()

	base := Options{MinSize: 16, MaxSize: 128, AspectPenalty: 0}
	tests := []struct {
		name    string
		options Options
		width   int
		height  int
		rotated bool
	}{
		{name: "optimal", options: base, width: 128, height: 16},
		{
			name:    "first fit",
			options: Options{MinSize: 16, MaxSize: 128, Heuristic: HeuristicFirstFit},
			width:   128,
			height:  16,
		},
		{
			name:    "force square",
			options: Options{MinSize: 16, MaxSize: 128, ForceSquare: true},
			width:   128, height: 128,
		},
		{
			name: "prefer height with rotation",
			options: Options{
				MinSize:       16,
				MaxSize:       128,
				AllowRotate:   true,
				PreferHeight:  true,
				AspectPenalty: 0,
			},
			width: 16, height: 128, rotated: true,
		},
		{
			name:    "padding",
			options: Options{MinSize: 16, MaxSize: 128, Padding: 2},
			width:   128, height: 16,
		},
		{
			name:    "positive aspect penalty",
			options: Options{MinSize: 16, MaxSize: 128, AspectPenalty: 2},
			width:   128, height: 128,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout, err := Plan([]Item{{ID: "wide", Width: 120, Height: 1}}, test.options)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if layout.Width != test.width || layout.Height != test.height {
				t.Fatalf("layout size = %dx%d, want %dx%d", layout.Width, layout.Height, test.width, test.height)
			}
			if len(layout.Placements) != 1 || layout.Placements[0].ID != "wide" {
				t.Fatalf("placements = %+v, want the original item ID", layout.Placements)
			}
			if layout.Placements[0].Rotated != test.rotated {
				t.Fatalf("placement rotation = %t, want %t", layout.Placements[0].Rotated, test.rotated)
			}
			if test.options.Padding > 0 &&
				(layout.Placements[0].X != test.options.Padding || layout.Placements[0].Y != test.options.Padding) {
				t.Fatalf("padded placement origin = (%d,%d), want (%d,%d)",
					layout.Placements[0].X, layout.Placements[0].Y, test.options.Padding, test.options.Padding)
			}
		})
	}
}

func TestCandidateScoreUsesFloatProduct(t *testing.T) {
	t.Parallel()

	w, h := int(1<<16), int(1<<16)
	want := float64(w) * float64(h)
	if got := candidateScore(w, h, 0); got != want {
		t.Fatalf("candidateScore(%d,%d,0) = %g, want %g", w, h, got, want)
	}
}

func TestCandidateScoreAvoidsNonFiniteResults(t *testing.T) {
	t.Parallel()

	for _, penalty := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := candidateScore(8, 4, penalty); math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("candidateScore with penalty %g = %g, want finite score", penalty, got)
		}
	}
}

func TestCandidateScoreSaturatesOverflow(t *testing.T) {
	t.Parallel()

	if got := candidateScore(2, 1, math.MaxFloat64); got != math.MaxFloat64 {
		t.Fatalf("candidateScore with MaxFloat64 penalty = %g, want MaxFloat64", got)
	}
}

func TestCandidateSizesNearMaxIntTerminate(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	sizes := candidateSizes(maxInt/2, maxInt)
	if len(sizes) == 0 || sizes[len(sizes)-1] != maxInt {
		t.Fatalf("candidate sizes = %v, want final size %d", sizes, maxInt)
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] <= sizes[i-1] {
			t.Fatalf("candidate sizes are not increasing: %v", sizes)
		}
	}
}

func TestPlanSizeSearchSupportsRotation(t *testing.T) {
	t.Parallel()

	item := Item{ID: "a", Width: 40, Height: 20}
	opts := Options{
		MinSize:      16,
		MaxSize:      64,
		Padding:      2,
		AllowRotate:  true,
		PreferHeight: true,
		Heuristic:    HeuristicBestShortSideFit,
	}

	layout, err := Plan([]Item{item}, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if layout.Width != 32 || layout.Height != 64 {
		t.Fatalf("rotated layout size = %dx%d, want 32x64", layout.Width, layout.Height)
	}
	if !layout.Placements[0].Rotated {
		t.Fatal("placement is not rotated")
	}
	if layout.Render == nil || *layout.Render != (RenderSettings{Padding: 2}) {
		t.Fatalf("rotated layout render settings = %+v, want padding=2", layout.Render)
	}
}

func TestPlanFirstFitIgnoresAspectPenalty(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 30, Height: 20},
		{ID: "b", Width: 30, Height: 20},
	}
	penalties := []struct {
		name  string
		value float64
	}{
		{name: "zero", value: 0},
		{name: "high", value: 100},
	}
	for _, penalty := range penalties {
		t.Run(penalty.name, func(t *testing.T) {
			t.Parallel()

			layout, err := Plan(items, Options{
				MinSize:       16,
				MaxSize:       128,
				AspectPenalty: penalty.value,
				Heuristic:     HeuristicFirstFit,
			})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if layout.Width != 64 || layout.Height != 32 {
				t.Fatalf("FirstFit size = %dx%d, want 64x32", layout.Width, layout.Height)
			}
		})
	}
}

func TestPlanCandidateCorpusDoesNotRegressAreaOrBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		items   []Item
		opts    Options
		maxArea int
	}{
		{
			name: "mixed sprites",
			items: []Item{
				{ID: "a", Width: 10, Height: 12},
				{ID: "b", Width: 8, Height: 8},
				{ID: "c", Width: 5, Height: 14},
				{ID: "d", Width: 11, Height: 7},
			},
			opts:    Options{MinSize: 32, MaxSize: 64, Padding: 1, Heuristic: HeuristicBottomLeft},
			maxArea: 1024,
		},
		{
			name: "two wide sprites",
			items: []Item{
				{ID: "a", Width: 30, Height: 20},
				{ID: "b", Width: 30, Height: 20},
			},
			opts:    Options{MinSize: 16, MaxSize: 64, AspectPenalty: 0.25, Heuristic: HeuristicBestShortSideFit},
			maxArea: 4096,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout, err := Plan(test.items, test.opts)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if area := layout.Width * layout.Height; area > test.maxArea {
				t.Fatalf("atlas area = %d, want <= %d", area, test.maxArea)
			}

			for _, placement := range layout.Placements {
				width, height := placement.Width, placement.Height
				if placement.Rotated {
					width, height = height, width
				}
				x := placement.X - test.opts.Padding
				y := placement.Y - test.opts.Padding
				width += 2 * test.opts.Padding
				height += 2 * test.opts.Padding
				if x < 0 || y < 0 || x+width > layout.Width || y+height > layout.Height {
					t.Fatalf("placement exceeds atlas: %+v atlas=%dx%d", placement, layout.Width, layout.Height)
				}
			}
		})
	}
}

func TestSortItemsForPacking(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "b", Width: 8, Height: 8},   // max=8
		{ID: "c", Width: 10, Height: 1},  // max=10 area=10
		{ID: "e", Width: 9, Height: 2},   // max=9 area=18
		{ID: "a", Width: 10, Height: 2},  // max=10 area=20
		{ID: "f", Width: 10, Height: 10}, // max=10 area=100
	}

	sortItemsForPacking(items, Options{Padding: 0})

	got := []string{items[0].ID, items[1].ID, items[2].ID, items[3].ID, items[4].ID}
	want := []string{"f", "a", "c", "e", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted order = %v, want %v", got, want)
		}
	}
}

func TestFindOptimalSizeForceSquare(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "one", Width: 40, Height: 20},
	}
	opts := Options{
		MinSize:     16,
		MaxSize:     128,
		Padding:     0,
		ForceSquare: true,
		Heuristic:   HeuristicBestShortSideFit,
	}

	w, h, found := findOptimalSize(items, opts)
	if !found {
		t.Fatal("findOptimalSize did not find a fit")
	}
	if w != h {
		t.Fatalf("force square size = %dx%d, want square", w, h)
	}
	if w < 40 {
		t.Fatalf("force square width = %d, want >= 40", w)
	}
	if w != 64 || h != 64 {
		t.Fatalf("force square size = %dx%d, want 64x64", w, h)
	}
}

func TestFindOptimalSizePreferHeightTieBreak(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 30, Height: 20},
		{ID: "b", Width: 30, Height: 20},
	}
	base := Options{
		MinSize:       16,
		MaxSize:       64,
		Padding:       0,
		AspectPenalty: 0.25,
		Heuristic:     HeuristicBestShortSideFit,
	}

	w1, h1, found := findOptimalSize(items, base)
	if !found {
		t.Fatal("findOptimalSize did not find a fit")
	}
	if w1 != 64 || h1 != 32 {
		t.Fatalf("prefer width default size = %dx%d, want 64x32", w1, h1)
	}

	base.PreferHeight = true
	w2, h2, found := findOptimalSize(items, base)
	if !found {
		t.Fatal("findOptimalSize did not find a fit")
	}
	if w2 != 32 || h2 != 64 {
		t.Fatalf("prefer height size = %dx%d, want 32x64", w2, h2)
	}
}
