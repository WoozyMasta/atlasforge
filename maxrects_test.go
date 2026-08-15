// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "testing"

func TestMaxRectsBestAreaFitLargeFreeRectangle(t *testing.T) {
	t.Parallel()

	bin := newMaxRects(32769, 32769, false)
	if rect, ok := bin.Insert(1, 1, HeuristicBestAreaFit); !ok || rect.W != 1 || rect.H != 1 {
		t.Fatal("BAF failed to insert a rectangle that fits")
	}
}

func TestMaxRectsAllHeuristicsInsertFittingRectangle(t *testing.T) {
	t.Parallel()

	heuristics := []Heuristic{
		HeuristicBestShortSideFit,
		HeuristicBestLongSideFit,
		HeuristicBestAreaFit,
		HeuristicBottomLeft,
		HeuristicContactPoint,
		HeuristicFirstFit,
	}
	for _, heuristic := range heuristics {
		t.Run(heuristicName(heuristic), func(t *testing.T) {
			t.Parallel()

			bin := newMaxRects(8, 8, false)
			rect, ok := bin.Insert(3, 2, heuristic)
			if !ok {
				t.Fatalf("Insert(%s) failed for fitting rectangle", heuristicName(heuristic))
			}
			if rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 8 || rect.Y+rect.H > 8 {
				t.Fatalf("placement is outside bin: %+v", rect)
			}
		})
	}
}

func TestMaxRectsAreaScoreAvoidsInt32Overflow(t *testing.T) {
	t.Parallel()

	bin := newMaxRects(50000, 50000, false)
	free := mrRect{W: 50000, H: 50000}
	primary, _ := bin.score(HeuristicBestAreaFit, free, 1, 1)
	want := int64(50000)*int64(50000) - 1
	if primary != want {
		t.Fatalf("BAF primary score = %d, want %d", primary, want)
	}
}

func TestMaxRectsEqualScoresKeepFreeRectangleOrder(t *testing.T) {
	t.Parallel()

	bin := newMaxRects(12, 4, false)
	bin.free = []mrRect{
		{X: 0, Y: 0, W: 4, H: 4},
		{X: 8, Y: 0, W: 4, H: 4},
	}

	rect, ok := bin.Insert(2, 2, HeuristicBestShortSideFit)
	if !ok {
		t.Fatal("Insert failed for fitting rectangle")
	}
	if rect.X != 0 {
		t.Fatalf("placement X = %d, want first free rectangle at X=0", rect.X)
	}
}
