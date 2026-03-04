// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"math"
)

// findOptimalSize searches candidate atlas sizes and returns the best fit.
func findOptimalSize(items []Item, opts Options) (width, height int) {
	minW, minH := 0, 0
	totalArea := 0
	for _, item := range items {
		w := item.Width + 2*opts.Padding
		h := item.Height + 2*opts.Padding
		if w > minW {
			minW = w
		}
		if h > minH {
			minH = h
		}

		totalArea += w * h
	}

	size := opts.MinSize
	if minW > size {
		size = nextPowerOfTwo(minW)
	}
	if minH > size {
		size = nextPowerOfTwo(minH)
	}

	bestW, bestH := size, size
	bestScore := math.MaxFloat64
	heuristic := normalizeHeuristic(opts.Heuristic)
	if heuristic == HeuristicFirstFit {
		return findFirstFitSize(items, opts, size, totalArea)
	}

	try := func(w, h int) {
		if w > opts.MaxSize || h > opts.MaxSize {
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
		for s := size; s <= opts.MaxSize; s *= 2 {
			try(s, s)
		}
		return bestW, bestH
	}

	if opts.PreferHeight {
		for w := size; w <= opts.MaxSize; w *= 2 {
			for h := size; h <= opts.MaxSize; h *= 2 {
				try(w, h)
			}
		}
	} else {
		for h := size; h <= opts.MaxSize; h *= 2 {
			for w := size; w <= opts.MaxSize; w *= 2 {
				try(w, h)
			}
		}
	}

	return bestW, bestH
}

// findFirstFitSize picks the first valid candidate in iteration order.
func findFirstFitSize(items []Item, opts Options, startSize int, totalArea int) (int, int) {
	try := func(w, h int) bool {
		if w > opts.MaxSize || h > opts.MaxSize {
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

	if opts.ForceSquare {
		for s := startSize; s <= opts.MaxSize; s *= 2 {
			if try(s, s) {
				return s, s
			}
		}

		return startSize, startSize
	}

	if opts.PreferHeight {
		for w := startSize; w <= opts.MaxSize; w *= 2 {
			for h := startSize; h <= opts.MaxSize; h *= 2 {
				if try(w, h) {
					return w, h
				}
			}
		}
	} else {
		for h := startSize; h <= opts.MaxSize; h *= 2 {
			for w := startSize; w <= opts.MaxSize; w *= 2 {
				if try(w, h) {
					return w, h
				}
			}
		}
	}

	return startSize, startSize
}

// candidateScore returns weighted area score used by optimal size search.
func candidateScore(w, h int, aspectPenalty float64) float64 {
	aspect := float64(max(w, h)) / float64(min(w, h))

	return float64(w*h) * (1.0 + aspectPenalty*(aspect-1.0))
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
