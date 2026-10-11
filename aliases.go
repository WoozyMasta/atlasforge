// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"slices"
)

// occupiedRect is a canonical placement's reserved rectangle during validation.
type occupiedRect struct {
	// placementID identifies the canonical sprite.
	placementID string
	// minX is the left edge including recorded padding.
	minX int
	// minY is the top edge including recorded padding.
	minY int
	// maxX is the right edge including recorded padding.
	maxX int
	// maxY is the bottom edge including recorded padding.
	maxY int
}

// aliasRelation maps one logical sprite ID to its canonical placement ID.
type aliasRelation struct {
	// id is the logical sprite represented by an alias placement.
	id string
	// canonicalID identifies the physical placement it shares.
	canonicalID string
}

// validateAliases checks ID references, shared geometry, and canonical overlap.
func validateAliases(layout *Layout) error {
	placementsByID := make(map[string]int, len(layout.Placements))
	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.ID == "" {
			return fmt.Errorf("%w: placement ID is empty", ErrInvalidLayout)
		}
		if _, exists := placementsByID[placement.ID]; exists {
			return fmt.Errorf("%w: duplicate placement ID %q", ErrInvalidLayout, placement.ID)
		}

		placementsByID[placement.ID] = i
	}

	padding := 0
	if layout.Render != nil {
		padding = layout.Render.Padding
	}

	canonicalRects := make([]occupiedRect, 0, len(layout.Placements))
	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.AliasOf != "" {
			canonicalIndex, exists := placementsByID[placement.AliasOf]
			if !exists {
				return fmt.Errorf(
					"%w: alias %q references missing canonical placement %q",
					ErrInvalidLayout,
					placement.ID,
					placement.AliasOf,
				)
			}

			canonical := layout.Placements[canonicalIndex]
			if canonical.AliasOf != "" {
				return fmt.Errorf(
					"%w: alias %q references alias %q instead of a canonical placement",
					ErrInvalidLayout,
					placement.ID,
					placement.AliasOf,
				)
			}

			if !samePlacementGeometry(placement, canonical) {
				return fmt.Errorf(
					"%w: alias %q geometry differs from canonical placement %q",
					ErrInvalidLayout,
					placement.ID,
					placement.AliasOf,
				)
			}

			continue
		}

		width, height := placement.Width, placement.Height
		if placement.Rotated {
			width, height = height, width
		}

		if padding > placement.X || padding > placement.Y ||
			padding > layout.Width-placement.X-width || padding > layout.Height-placement.Y-height {
			return fmt.Errorf(
				"%w: reserved padding exceeds atlas bounds: ID=%q Padding=%d",
				ErrInvalidLayout,
				placement.ID,
				padding,
			)
		}

		canonicalRects = append(canonicalRects, occupiedRect{
			placementID: placement.ID,
			minX:        placement.X - padding,
			minY:        placement.Y - padding,
			maxX:        placement.X + width + padding,
			maxY:        placement.Y + height + padding,
		})
	}

	spanX, spanY := 0, 0
	if len(canonicalRects) > 0 {
		minX, maxX := canonicalRects[0].minX, canonicalRects[0].minX
		minY, maxY := canonicalRects[0].minY, canonicalRects[0].minY
		for i := 1; i < len(canonicalRects); i++ {
			minX = min(minX, canonicalRects[i].minX)
			maxX = max(maxX, canonicalRects[i].minX)
			minY = min(minY, canonicalRects[i].minY)
			maxY = max(maxY, canonicalRects[i].minY)
		}
		spanX = maxX - minX
		spanY = maxY - minY
	}

	// Sweep where placement starts spread farther apart to prune candidates.
	byX := spanX >= spanY
	slices.SortFunc(canonicalRects, func(a, b occupiedRect) int {
		if byX {
			if a.minX == b.minX {
				return a.maxX - b.maxX
			}
			return a.minX - b.minX
		}

		if a.minY == b.minY {
			return a.maxY - b.maxY
		}
		return a.minY - b.minY
	})

	for i := range canonicalRects {
		for j := i + 1; j < len(canonicalRects); j++ {
			if byX && canonicalRects[j].minX >= canonicalRects[i].maxX {
				break
			}
			if !byX && canonicalRects[j].minY >= canonicalRects[i].maxY {
				break
			}

			if rectanglesOverlap(canonicalRects[i], canonicalRects[j]) {
				return fmt.Errorf(
					"%w: canonical placements %q and %q overlap",
					ErrInvalidLayout,
					canonicalRects[i].placementID,
					canonicalRects[j].placementID,
				)
			}
		}
	}

	return nil
}

// appendAliases adds logical placements beside their page's canonical source.
func appendAliases(layout *Layout, aliases []aliasRelation, trimByID map[string]trimMetadata) error {
	canonicalByID := make(map[string]Placement, len(layout.Placements))
	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.AliasOf == "" {
			canonicalByID[placement.ID] = placement
		}
	}

	for _, relation := range aliases {
		canonical, exists := canonicalByID[relation.canonicalID]
		if !exists {
			return fmt.Errorf(
				"%w: canonical placement %q for alias %q was not planned",
				ErrInvalidLayout,
				relation.canonicalID,
				relation.id,
			)
		}

		placement := canonical
		placement.ID = relation.id
		placement.AliasOf = relation.canonicalID
		placement.OriginalWidth = 0
		placement.OriginalHeight = 0
		placement.OffsetX = 0
		placement.OffsetY = 0
		if metadata, ok := trimByID[relation.id]; ok {
			placement.OriginalWidth = metadata.originalWidth
			placement.OriginalHeight = metadata.originalHeight
			placement.OffsetX = metadata.offsetX
			placement.OffsetY = metadata.offsetY
		}
		layout.Placements = append(layout.Placements, placement)
	}

	return nil
}

// appendAliasesAcrossPages keeps each alias beside its canonical page.
func appendAliasesAcrossPages(
	layouts []Layout,
	aliases []aliasRelation,
	trimByID map[string]trimMetadata,
) error {
	pageByCanonical := make(map[string]int)
	for pageIndex := range layouts {
		for i := range layouts[pageIndex].Placements {
			placement := layouts[pageIndex].Placements[i]
			if placement.AliasOf == "" {
				pageByCanonical[placement.ID] = pageIndex
			}
		}
	}

	aliasesByPage := make([][]aliasRelation, len(layouts))
	for _, relation := range aliases {
		pageIndex, exists := pageByCanonical[relation.canonicalID]
		if !exists {
			return fmt.Errorf(
				"%w: canonical placement %q for alias %q was not planned",
				ErrInvalidLayout,
				relation.canonicalID,
				relation.id,
			)
		}

		aliasesByPage[pageIndex] = append(aliasesByPage[pageIndex], relation)
	}

	for pageIndex := range layouts {
		if err := appendAliases(&layouts[pageIndex], aliasesByPage[pageIndex], trimByID); err != nil {
			return fmt.Errorf("append aliases to page %d: %w", pageIndex, err)
		}
	}

	return nil
}

// samePlacementGeometry compares the physical fields shared by an alias.
func samePlacementGeometry(alias, canonical Placement) bool {
	return alias.X == canonical.X && alias.Y == canonical.Y &&
		alias.Width == canonical.Width && alias.Height == canonical.Height &&
		alias.Rotated == canonical.Rotated
}

// rectanglesOverlap reports whether two reserved axis-aligned rectangles intersect.
func rectanglesOverlap(a, b occupiedRect) bool {
	return a.minX < b.maxX && b.minX < a.maxX &&
		a.minY < b.maxY && b.minY < a.maxY
}
