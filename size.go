// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"math"
)

// findOptimalSize searches candidate atlas sizes after Plan validates arithmetic bounds.
func findOptimalSize(items []Item, opts Options) (width, height int, found bool) {
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
		return findFirstFitSize(items, opts, sizes, minW, minH, totalArea)
	}

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

		if !canFitMaxRects(items, w, h, opts, heuristic) {
			return
		}

		bestScore = score
		bestW, bestH = w, h
	}

	if opts.ForceSquare {
		for _, s := range sizes {
			try(s, s)
		}
		return bestW, bestH, bestW > 0
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

	return bestW, bestH, bestW > 0
}

// findFirstFitSize picks the first valid candidate in the established iteration order.
func findFirstFitSize(
	items []Item,
	opts Options,
	sizes []int,
	minW, minH int,
	totalArea int,
) (int, int, bool) {
	try := func(w, h int) bool {
		if w < minW || h < minH {
			return false
		}

		if opts.ForceSquare {
			if w != h {
				return false
			}
		} else if absPowerDiff(w, h) > 1 {
			return false
		}
		if totalArea > w*h {
			return false
		}

		return canFitMaxRects(items, w, h, opts, HeuristicFirstFit)
	}

	// FirstFit deliberately ignores AspectPenalty and returns the first fitting size.
	if opts.ForceSquare {
		for _, s := range sizes {
			if try(s, s) {
				return s, s, true
			}
		}

		return 0, 0, false
	}

	if opts.PreferHeight {
		for _, w := range sizes {
			for _, h := range sizes {
				if try(w, h) {
					return w, h, true
				}
			}
		}
	} else {
		for _, h := range sizes {
			for _, w := range sizes {
				if try(w, h) {
					return w, h, true
				}
			}
		}
	}

	return 0, 0, false
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

// canFitMaxRects dry-runs placement for all items.
func canFitMaxRects(items []Item, w, h int, opts Options, heuristic Heuristic) bool {
	bin := newMaxRects(w, h, opts.AllowRotate)
	for _, item := range items {
		pw := item.Width + 2*opts.Padding
		ph := item.Height + 2*opts.Padding

		if _, ok := bin.Insert(pw, ph, heuristic); !ok {
			return false
		}
	}

	return true
}
