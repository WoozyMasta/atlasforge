// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "sort"

// sortItemsForPacking sorts bigger items first after Plan validates padded sizes and area.
func sortItemsForPacking(items []Item, opts Options) {
	sort.Slice(items, func(i, j int) bool {
		wi := items[i].Width + 2*opts.Padding
		hi := items[i].Height + 2*opts.Padding
		wj := items[j].Width + 2*opts.Padding
		hj := items[j].Height + 2*opts.Padding

		mi := max(wi, hi)
		mj := max(wj, hj)
		if mi != mj {
			return mi > mj
		}

		ai := wi * hi
		aj := wj * hj
		if ai != aj {
			return ai > aj
		}

		if hi != hj {
			return hi > hj
		}

		return wi > wj
	})
}
