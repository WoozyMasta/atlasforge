// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"math"
)

// findOptimalSize searches candidate atlas sizes after Plan validates arithmetic bounds.
func findOptimalSize(items []Item, opts Options) (width, height int, found bool) {
	width, height, _, found = findOptimalPlan(items, opts)

	return width, height, found
}

// findOptimalPlan returns the selected size and its validated MaxRects placements.
func findOptimalPlan(items []Item, opts Options) (width, height int, placements []mrRect, found bool) {
	minW, minH := opts.MinSize, opts.MinSize
	totalArea := 0
	for _, item := range items {
		w := item.Width + 2*opts.Padding
		h := item.Height + 2*opts.Padding
		if opts.AllowRotate {
			minSide := min(w, h)
			minW = max(minW, minSide)
			minH = max(minH, minSide)
		} else {
			minW = max(minW, w)
			minH = max(minH, h)
		}

		totalArea += w * h
	}

	sizes := candidateSizes(opts.MinSize, opts.MaxSize)
	bestW, bestH := 0, 0
	bestScore := math.MaxFloat64
	heuristic := normalizeHeuristic(opts.Heuristic)
	if heuristic == HeuristicFirstFit {
		return findFirstFitPlan(items, opts, sizes, minW, minH, totalArea)
	}

	var bestPlacements []mrRect
	try := func(w, h int) {
		if w < minW || h < minH {
			return
		}

		if opts.ForceSquare {
			if w != h {
				return
			}
		} else if absPowerDiff(w, h) > 1 {
			return
		}
		if totalArea > w*h {
			return
		}

		score := candidateScore(w, h, opts.AspectPenalty)
		if !isBetterCandidate(w, h, score, bestW, bestH, bestScore, opts.PreferHeight) {
			return
		}

		candidatePlacements, remaining := packIntoBin(items, w, h, opts, heuristic, false)
		if len(remaining) != 0 {
			return
		}

		bestScore = score
		bestW, bestH = w, h
		bestPlacements = candidatePlacements
	}

	if opts.ForceSquare {
		for _, s := range sizes {
			try(s, s)
		}
		return bestW, bestH, bestPlacements, bestW > 0
	}

	if opts.PreferHeight {
		for _, w := range sizes {
			for _, h := range sizes {
				try(w, h)
			}
		}
	} else {
		for _, h := range sizes {
			for _, w := range sizes {
				try(w, h)
			}
		}
	}

	return bestW, bestH, bestPlacements, bestW > 0
}

// findFirstFitPlan returns placements for the first fitting candidate in iteration order.
func findFirstFitPlan(
	items []Item,
	opts Options,
	sizes []int,
	minW, minH int,
	totalArea int,
) (int, int, []mrRect, bool) {
	try := func(w, h int) ([]mrRect, bool) {
		if w < minW || h < minH {
			return nil, false
		}

		if opts.ForceSquare {
			if w != h {
				return nil, false
			}
		} else if absPowerDiff(w, h) > 1 {
			return nil, false
		}
		if totalArea > w*h {
			return nil, false
		}

		placements, remaining := packIntoBin(items, w, h, opts, HeuristicFirstFit, false)
		if len(remaining) != 0 {
			return nil, false
		}

		return placements, true
	}

	// FirstFit deliberately ignores AspectPenalty and returns the first fitting size.
	if opts.ForceSquare {
		for _, s := range sizes {
			if placements, ok := try(s, s); ok {
				return s, s, placements, true
			}
		}

		return 0, 0, nil, false
	}

	if opts.PreferHeight {
		for _, w := range sizes {
			for _, h := range sizes {
				if placements, ok := try(w, h); ok {
					return w, h, placements, true
				}
			}
		}
	} else {
		for _, h := range sizes {
			for _, w := range sizes {
				if placements, ok := try(w, h); ok {
					return w, h, placements, true
				}
			}
		}
	}

	return 0, 0, nil, false
}

// candidateSizes builds the deterministic MinSize doubling ladder through MaxSize.
func candidateSizes(minSize, maxSize int) []int {
	sizes := make([]int, 0, 8)
	for size := minSize; ; {
		sizes = append(sizes, size)
		if size > maxSize/2 {
			break
		}

		size *= 2
	}

	if sizes[len(sizes)-1] != maxSize {
		sizes = append(sizes, maxSize)
	}

	return sizes
}

// candidateScore returns weighted area score used by optimal size search.
func candidateScore(w, h int, aspectPenalty float64) float64 {
	aspect := float64(max(w, h)) / float64(min(w, h))
	score := (float64(w) * float64(h)) * (1.0 + aspectPenalty*(aspect-1.0))
	if math.IsNaN(score) || math.IsInf(score, 0) {
		// Scores outside float range saturate; ties use the standard deterministic policy.
		return math.MaxFloat64
	}

	return score
}

// isBetterCandidate checks if candidate could replace current best by score/tie policy.
func isBetterCandidate(
	w, h int,
	score float64,
	bestW, bestH int,
	bestScore float64,
	preferHeight bool,
) bool {
	if score < bestScore {
		return true
	}

	if score > bestScore {
		return false
	}

	return isBetterTie(w, h, bestW, bestH, preferHeight)
}

// isBetterTie checks tie preference when scores are equal.
func isBetterTie(w, h, otherW, otherH int, preferHeight bool) bool {
	if preferHeight {
		if h > otherH {
			return true
		}

		if h < otherH {
			return false
		}

		return w > otherW
	}

	if w > otherW {
		return true
	}

	if w < otherW {
		return false
	}

	return h > otherH
}

// packIntoBin inserts sorted items into one fixed bin
// and returns placements plus items that did not fit.
//
// When continueAfterFailure is false,
// remaining contains the first failed item and the untouched suffix.
func packIntoBin(
	items []Item,
	w, h int,
	opts Options,
	heuristic Heuristic,
	continueAfterFailure bool,
) (placements []mrRect, remaining []Item) {
	bin := newMaxRects(w, h, opts.AllowRotate)
	for i, item := range items {
		pw := item.Width + 2*opts.Padding
		ph := item.Height + 2*opts.Padding

		if _, ok := bin.Insert(pw, ph, heuristic); !ok {
			if !continueAfterFailure {
				return bin.used, items[i:]
			}

			remaining = append(remaining, item)
		}
	}

	return bin.used, remaining
}
