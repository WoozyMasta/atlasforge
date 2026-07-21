// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "testing"

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

	w, h := findOptimalSize(items, opts)
	if w != h {
		t.Fatalf("force square size = %dx%d, want square", w, h)
	}
	if w < 40 {
		t.Fatalf("force square width = %d, want >= 40", w)
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

	w1, h1 := findOptimalSize(items, base)
	if w1 != 64 || h1 != 32 {
		t.Fatalf("prefer width default size = %dx%d, want 64x32", w1, h1)
	}

	base.PreferHeight = true
	w2, h2 := findOptimalSize(items, base)
	if w2 != 32 || h2 != 64 {
		t.Fatalf("prefer height size = %dx%d, want 32x64", w2, h2)
	}
}
