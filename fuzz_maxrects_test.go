// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "testing"

func FuzzMaxRects(f *testing.F) {
	for _, seed := range maxRectsFuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		input := decodeMaxRectsFuzzInput(data)
		bin := newMaxRects(input.width, input.height, input.allowRotate)
		assertMaxRectsFuzzState(t, bin, input.heuristic != HeuristicFirstFit)

		for i, requested := range input.rectangles {
			usedBefore := len(bin.used)
			rect, ok := bin.Insert(requested.width, requested.height, input.heuristic)
			if ok {
				assertMaxRectsFuzzPlacement(t, input, requested, rect)
				if len(bin.used) != usedBefore+1 || bin.used[len(bin.used)-1] != rect {
					t.Fatalf("insert %d did not append the returned used rectangle", i)
				}
			} else if len(bin.used) != usedBefore {
				t.Fatalf("failed insert %d changed the used rectangle count", i)
			}

			assertMaxRectsFuzzState(t, bin, input.heuristic != HeuristicFirstFit)
		}
	})
}

// maxRectsFuzzInput contains bounded canvas and rectangle requests.
type maxRectsFuzzInput struct {
	width       int
	height      int
	allowRotate bool
	heuristic   Heuristic
	rectangles  []maxRectsFuzzSize
}

// maxRectsFuzzSize is one requested unrotated rectangle dimension pair.
type maxRectsFuzzSize struct {
	width  int
	height int
}

// decodeMaxRectsFuzzInput restricts canvas and request sizes before allocation.
func decodeMaxRectsFuzzInput(data []byte) maxRectsFuzzInput {
	if len(data) > 256 {
		data = data[:256]
	}
	byteAt := func(index int) byte {
		if index < len(data) {
			return data[index]
		}

		return 0
	}

	count := int(byteAt(2) % 33)
	input := maxRectsFuzzInput{
		width:       2 + int(byteAt(0)%63),
		height:      2 + int(byteAt(1)%63),
		allowRotate: byteAt(3)&1 != 0,
		heuristic:   Heuristic(byteAt(4) % 6),
		rectangles:  make([]maxRectsFuzzSize, count),
	}
	for i := range input.rectangles {
		input.rectangles[i] = maxRectsFuzzSize{
			width:  1 + int(byteAt(5+2*i)%32),
			height: 1 + int(byteAt(6+2*i)%32),
		}
	}

	return input
}

// assertMaxRectsFuzzPlacement validates orientation and bounds of one insertion.
func assertMaxRectsFuzzPlacement(t *testing.T, input maxRectsFuzzInput, requested maxRectsFuzzSize, rect mrRect) {
	t.Helper()
	if rect.W <= 0 || rect.H <= 0 || rect.X < 0 || rect.Y < 0 ||
		rect.X+rect.W > input.width || rect.Y+rect.H > input.height {
		t.Fatalf("inserted rectangle outside %dx%d canvas: %+v", input.width, input.height, rect)
	}
	if rect.Rotated {
		if !input.allowRotate || rect.W != requested.height || rect.H != requested.width {
			t.Fatalf("invalid rotated result %+v for request %+v", rect, requested)
		}
	} else if rect.W != requested.width || rect.H != requested.height {
		t.Fatalf("unrotated result %+v does not match request %+v", rect, requested)
	}
}

// assertMaxRectsFuzzState checks used/free bounds and excludes free-used overlap.
func assertMaxRectsFuzzState(t *testing.T, bin *maxRects, checkContainment bool) {
	t.Helper()
	for i, used := range bin.used {
		if !maxRectsFuzzInside(used, bin.w, bin.h) {
			t.Fatalf("used rectangle %d is outside canvas: %+v", i, used)
		}
		for j := range i {
			if maxRectsFuzzOverlaps(used, bin.used[j]) {
				t.Fatalf("used rectangles %d and %d intersect: %+v / %+v", j, i, bin.used[j], used)
			}
		}
	}

	for i, free := range bin.free {
		if !maxRectsFuzzInside(free, bin.w, bin.h) {
			t.Fatalf("free rectangle %d is outside canvas: %+v", i, free)
		}
		for j, used := range bin.used {
			if maxRectsFuzzOverlaps(free, used) {
				t.Fatalf("free rectangle %d intersects used rectangle %d: %+v / %+v", i, j, free, used)
			}
		}
		if checkContainment {
			for j := range i {
				if maxRectsFuzzContains(free, bin.free[j]) || maxRectsFuzzContains(bin.free[j], free) {
					t.Fatalf("pruned free rectangles %d and %d contain one another: %+v / %+v", j, i, bin.free[j], free)
				}
			}
		}
	}
}

// maxRectsFuzzInside verifies positive dimensions and canvas containment.
func maxRectsFuzzInside(rect mrRect, width, height int) bool {
	return rect.W > 0 && rect.H > 0 && rect.X >= 0 && rect.Y >= 0 &&
		rect.X+rect.W <= width && rect.Y+rect.H <= height
}

// maxRectsFuzzOverlaps checks half-open rectangle intersection independently.
func maxRectsFuzzOverlaps(a, b mrRect) bool {
	return a.X < b.X+b.W &&
		b.X < a.X+a.W &&
		a.Y < b.Y+b.H &&
		b.Y < a.Y+a.H
}

// maxRectsFuzzContains checks rectangle containment without production helpers.
func maxRectsFuzzContains(a, b mrRect) bool {
	return a.X >= b.X && a.Y >= b.Y &&
		a.X+a.W <= b.X+b.W && a.Y+a.H <= b.Y+b.H
}

// maxRectsFuzzSeeds covers empty, skinny, repeated, rotated, and all heuristic cases.
func maxRectsFuzzSeeds() [][]byte {
	seeds := make([][]byte, 0, 12)
	seeds = append(seeds,
		maxRectsFuzzSeed(0, 0, 0, 0, 0),
		maxRectsFuzzSeed(62, 2, 1, 1, 0, 0, 31),
		maxRectsFuzzSeed(62, 2, 1, 0, 0, 31, 0),
		maxRectsFuzzSeed(62, 62, 1, 1, 2, 0, 31),
		maxRectsFuzzSeed(62, 62, 1, 0, 3, 31, 0),
		maxRectsFuzzSeed(62, 62, 12, 0, 5,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0),
	)
	for _, heuristic := range [...]byte{0, 1, 2, 3, 4, 5} {
		seeds = append(seeds, maxRectsFuzzSeed(62, 62, 3, 1, heuristic, 1, 2, 2, 3, 3, 4))
	}

	return seeds
}

// maxRectsFuzzSeed encodes canvas, flags, heuristic, and requested dimensions.
func maxRectsFuzzSeed(canvasWidth, canvasHeight, count, flags, heuristic byte, dimensions ...byte) []byte {
	seed := make([]byte, 0, 5+len(dimensions))
	seed = append(seed, canvasWidth, canvasHeight, count, flags, heuristic)

	return append(seed, dimensions...)
}

// TestMaxRectsFuzzSplitFreeTouchingAndDisjoint covers non-overlapping edge cases.
func TestMaxRectsFuzzSplitFreeTouchingAndDisjoint(t *testing.T) {
	t.Parallel()

	free := mrRect{X: 0, Y: 0, W: 4, H: 4}
	tests := []struct {
		name string
		used mrRect
	}{
		{name: "touching right edge", used: mrRect{X: 4, Y: 0, W: 2, H: 4}},
		{name: "touching bottom edge", used: mrRect{X: 0, Y: 4, W: 4, H: 2}},
		{name: "separated", used: mrRect{X: 5, Y: 5, W: 1, H: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bin := newMaxRects(8, 8, false)
			bin.free = []mrRect{free}

			if bin.splitFree(0, test.used) {
				t.Fatalf("splitFree(%+v) reported non-overlapping geometry", test.used)
			}
			if len(bin.free) != 1 || bin.free[0] != free {
				t.Fatalf("free list changed after non-overlap: %+v", bin.free)
			}
		})
	}
}

// TestMaxRectsFuzzContainedInCoversEqualAndNestedRectangles fixes containment semantics.
func TestMaxRectsFuzzContainedInCoversEqualAndNestedRectangles(t *testing.T) {
	t.Parallel()

	outer := mrRect{X: 1, Y: 2, W: 8, H: 7}
	inner := mrRect{X: 3, Y: 4, W: 2, H: 3}
	equal := outer
	if !containedIn(equal, outer) || !containedIn(outer, equal) {
		t.Fatal("equal rectangles should contain one another")
	}
	if !containedIn(inner, outer) || containedIn(outer, inner) {
		t.Fatal("nested rectangles have incorrect containment")
	}
	if containedIn(mrRect{X: 9, Y: 2, W: 1, H: 1}, outer) {
		t.Fatal("partially outside rectangle should not be contained")
	}
}

// TestMaxRectsFuzzPruneFreeRemovesNestedRectangles verifies post-prune containment.
func TestMaxRectsFuzzPruneFreeRemovesNestedRectangles(t *testing.T) {
	t.Parallel()

	outer := mrRect{X: 0, Y: 0, W: 8, H: 8}
	disjoint := mrRect{X: 10, Y: 10, W: 2, H: 2}
	bin := newMaxRects(16, 16, false)
	bin.free = []mrRect{
		outer,
		outer,
		{X: 2, Y: 2, W: 2, H: 2},
		disjoint,
	}
	bin.pruneFree()

	if len(bin.free) != 2 || bin.free[0] != outer || bin.free[1] != disjoint {
		t.Fatalf("free list after prune = %+v, want outer and disjoint rectangles", bin.free)
	}
}
