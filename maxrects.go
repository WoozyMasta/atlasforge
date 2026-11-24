// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

// maxScore is the upper bound for MaxRects primary and secondary scores.
const maxScore int64 = 1<<63 - 1

// mrRect is an internal MaxRects rectangle.
type mrRect struct {
	X       int  // Top-left X coordinate.
	Y       int  // Top-left Y coordinate.
	W       int  // Rectangle width.
	H       int  // Rectangle height.
	Rotated bool // Whether the item was rotated clockwise.
}

// maxRects implements MaxRects bin packing.
type maxRects struct {
	used []mrRect // Rectangles already placed in the bin.
	free []mrRect // Remaining candidate rectangles.

	pruneMarks       []bool    // Reusable marks for free lists larger than the embedded buffer.
	inlinePruneMarks [128]bool // Embedded scratch space for common small free lists.

	w           int  // Bin width.
	h           int  // Bin height.
	allowRotate bool // Whether placements may rotate by 90 degrees.
}

// newMaxRects creates a MaxRects planner for fixed atlas dimensions.
func newMaxRects(w, h int, allowRotate bool) *maxRects {
	m := &maxRects{
		w:           w,
		h:           h,
		allowRotate: allowRotate,
		used:        make([]mrRect, 0, 128),
		free:        make([]mrRect, 0, 128),
	}
	m.free = append(m.free, mrRect{X: 0, Y: 0, W: w, H: h})

	return m
}

// Insert inserts a rectangle into the planner.
func (m *maxRects) Insert(w, h int, heuristic Heuristic) (mrRect, bool) {
	if heuristic == HeuristicFirstFit {
		for i := 0; i < len(m.free); i++ {
			fr := m.free[i]
			if fr.W >= w && fr.H >= h {
				rect := mrRect{X: fr.X, Y: fr.Y, W: w, H: h, Rotated: false}
				m.placeFirstFit(rect)
				return rect, true
			}

			if m.allowRotate && fr.W >= h && fr.H >= w {
				rect := mrRect{X: fr.X, Y: fr.Y, W: h, H: w, Rotated: true}
				m.placeFirstFit(rect)
				return rect, true
			}
		}

		return mrRect{}, false
	}

	best := mrRect{}
	pri, sec := maxScore, maxScore
	found := false

	for i := 0; i < len(m.free); i++ {
		fr := m.free[i]

		if fr.W >= w && fr.H >= h {
			p1, s1 := m.score(heuristic, fr, w, h)
			if p1 < pri || (p1 == pri && s1 < sec) {
				pri, sec = p1, s1
				best = mrRect{X: fr.X, Y: fr.Y, W: w, H: h, Rotated: false}
				found = true
			}
		}

		if m.allowRotate && fr.W >= h && fr.H >= w {
			p2, s2 := m.score(heuristic, fr, h, w)
			if p2 < pri || (p2 == pri && s2 < sec) {
				pri, sec = p2, s2
				best = mrRect{X: fr.X, Y: fr.Y, W: h, H: w, Rotated: true}
				found = true
			}
		}
	}

	if !found {
		return mrRect{}, false
	}

	m.place(best)
	return best, true
}

// placeFirstFit splits affected free rectangles and defers pruning for speed.
func (m *maxRects) placeFirstFit(used mrRect) {
	for i := 0; i < len(m.free); {
		if m.splitFree(i, used) {
			m.free = removeAtSwap(m.free, i)
			continue
		}

		i++
	}

	// FirstFit defers quadratic pruning until the free list is large, then prunes every eight placements.
	shouldPrune := len(m.free) > 256 && (len(m.used)&7) == 7
	if shouldPrune {
		m.pruneFree()
	}

	m.used = append(m.used, used)
}

// place splits every free rectangle that intersects the placed rectangle.
func (m *maxRects) place(used mrRect) {
	for i := 0; i < len(m.free); {
		if m.splitFree(i, used) {
			m.free = removeAt(m.free, i)
			continue
		}
		i++
	}

	m.pruneFree()
	m.used = append(m.used, used)
}

// score calculates ranking values for the given heuristic.
func (m *maxRects) score(heuristic Heuristic, fr mrRect, rw, rh int) (pri, sec int64) {
	switch heuristic {
	case HeuristicBestShortSideFit:
		leftoverH := fr.W - rw
		if leftoverH < 0 {
			leftoverH = -leftoverH
		}

		leftoverV := fr.H - rh
		if leftoverV < 0 {
			leftoverV = -leftoverV
		}

		shortSide := leftoverH
		longSide := leftoverV
		if leftoverV < shortSide {
			shortSide = leftoverV
		}
		if leftoverH > longSide {
			longSide = leftoverH
		}

		return int64(shortSide), int64(longSide)

	case HeuristicBestLongSideFit:
		leftoverH := fr.W - rw
		if leftoverH < 0 {
			leftoverH = -leftoverH
		}

		leftoverV := fr.H - rh
		if leftoverV < 0 {
			leftoverV = -leftoverV
		}

		shortSide := leftoverH
		longSide := leftoverV
		if leftoverV < shortSide {
			shortSide = leftoverV
		}
		if leftoverH > longSide {
			longSide = leftoverH
		}

		return int64(longSide), int64(shortSide)

	case HeuristicBestAreaFit:
		areaFit := int64(fr.W)*int64(fr.H) - int64(rw)*int64(rh)
		leftoverH := fr.W - rw
		if leftoverH < 0 {
			leftoverH = -leftoverH
		}

		leftoverV := fr.H - rh
		if leftoverV < 0 {
			leftoverV = -leftoverV
		}

		shortSide := min(leftoverH, leftoverV)

		return areaFit, int64(shortSide)

	case HeuristicBottomLeft:
		return int64(fr.Y + rh), int64(fr.X)

	case HeuristicContactPoint:
		return -m.contactScore(fr.X, fr.Y, rw, rh), 0

	default:
		return maxScore, maxScore
	}
}

// contactScore calculates contact surface score for ContactPoint heuristic.
func (m *maxRects) contactScore(x, y, w, h int) int64 {
	score := int64(0)
	if x == 0 || x+w == m.w {
		score += int64(h)
	}
	if y == 0 || y+h == m.h {
		score += int64(w)
	}

	for i := 0; i < len(m.used); i++ {
		u := m.used[i]
		if u.X == x+w || u.X+u.W == x {
			score += commonInterval(u.Y, u.Y+u.H, y, y+h)
		}
		if u.Y == y+h || u.Y+u.H == y {
			score += commonInterval(u.X, u.X+u.W, x, x+w)
		}
	}

	return score
}

// commonInterval returns overlap length between [a0,a1) and [b0,b1).
func commonInterval(a0, a1, b0, b1 int) int64 {
	if a1 <= b0 || b1 <= a0 {
		return 0
	}

	end := min(a1, b1)
	start := max(a0, b0)

	return int64(end) - int64(start)
}

// splitFree removes an overlapping occupied region by appending the remaining strips.
func (m *maxRects) splitFree(freeIdx int, used mrRect) bool {
	fr := m.free[freeIdx]

	if used.X >= fr.X+fr.W || used.X+used.W <= fr.X || used.Y >= fr.Y+fr.H || used.Y+used.H <= fr.Y {
		return false
	}

	if used.X < fr.X+fr.W && used.X+used.W > fr.X {
		if used.Y > fr.Y && used.Y < fr.Y+fr.H {
			m.free = append(m.free, mrRect{X: fr.X, Y: fr.Y, W: fr.W, H: used.Y - fr.Y})
		}
		if used.Y+used.H < fr.Y+fr.H {
			m.free = append(
				m.free,
				mrRect{
					X: fr.X, Y: used.Y + used.H,
					W: fr.W, H: fr.Y + fr.H - (used.Y + used.H),
				},
			)
		}
	}

	if used.Y < fr.Y+fr.H && used.Y+used.H > fr.Y {
		if used.X > fr.X && used.X < fr.X+fr.W {
			m.free = append(m.free, mrRect{X: fr.X, Y: fr.Y, W: used.X - fr.X, H: fr.H})
		}
		if used.X+used.W < fr.X+fr.W {
			m.free = append(
				m.free,
				mrRect{
					X: used.X + used.W, Y: fr.Y,
					W: fr.X + fr.W - (used.X + used.W), H: fr.H,
				},
			)
		}
	}

	return true
}

// pruneFree removes free rectangles fully contained by another free rectangle.
func (m *maxRects) pruneFree() {
	n := len(m.free)
	if n < 2 {
		return
	}

	var marks []bool
	if n <= len(m.inlinePruneMarks) {
		marks = m.inlinePruneMarks[:n]
	} else {
		if cap(m.pruneMarks) < n {
			m.pruneMarks = make([]bool, n)
		}

		marks = m.pruneMarks[:n]
	}

	clear(marks)
	anyMarked := false

	for i := range n {
		if marks[i] {
			continue
		}

		a := m.free[i]
		for j := i + 1; j < n; j++ {
			if marks[j] {
				continue
			}

			b := m.free[j]
			if a.W <= b.W && a.H <= b.H && containedIn(a, b) {
				marks[i] = true
				anyMarked = true
				break
			}

			if b.W <= a.W && b.H <= a.H && containedIn(b, a) {
				marks[j] = true
				anyMarked = true
			}
		}
	}

	if !anyMarked {
		return
	}

	dst := m.free[:0]
	for i := range n {
		if marks[i] {
			continue
		}

		dst = append(dst, m.free[i])
	}

	m.free = dst
}

// containedIn reports whether rectangle a is fully contained in rectangle b.
func containedIn(a, b mrRect) bool {
	return a.X >= b.X && a.Y >= b.Y && a.X+a.W <= b.X+b.W && a.Y+a.H <= b.Y+b.H
}

// removeAt removes a slice element while preserving the order of remaining elements.
func removeAt[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}

	copy(s[i:], s[i+1:])
	return s[:len(s)-1]
}

// removeAtSwap removes a slice element without preserving the order of remaining elements.
func removeAtSwap[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}

	last := len(s) - 1
	s[i] = s[last]

	return s[:last]
}
