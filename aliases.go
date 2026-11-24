// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"slices"
)

// occupiedRect is a canonical placement's reserved rectangle during validation.
type occupiedRect struct {
	placementID string // canonical sprite represented by these bounds.
	minX        int    // reserved left edge, including padding.
	minY        int    // reserved top edge, including padding.
	maxX        int    // reserved right edge, including padding.
	maxY        int    // reserved bottom edge, including padding.
}

// aliasRelation maps one logical sprite ID to its canonical placement ID.
type aliasRelation struct {
	id          string // logical sprite represented by the alias placement.
	canonicalID string // physical placement shared by the alias.
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
	groups := make(map[int][]Placement)
	blocks := make(map[int]OverlapBlock, len(layout.OverlapBlocks))
	for _, block := range layout.OverlapBlocks {
		if block.ID <= 0 {
			return fmt.Errorf("%w: overlap block ID must be positive", ErrInvalidLayout)
		}
		if _, exists := blocks[block.ID]; exists {
			return fmt.Errorf("%w: duplicate overlap block ID %d", ErrInvalidLayout, block.ID)
		}

		blocks[block.ID] = block
	}

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

		if placement.OverlapGroup < 0 {
			return fmt.Errorf("%w: negative overlap group for %q", ErrInvalidLayout, placement.ID)
		}

		if placement.OverlapGroup > 0 {
			groups[placement.OverlapGroup] = append(groups[placement.OverlapGroup], placement)
			continue
		}

		reserved, err := paddedPlacementBounds(layout, placement, padding)
		if err != nil {
			return err
		}

		canonicalRects = append(canonicalRects, occupiedRect{
			placementID: placement.ID,
			minX:        reserved.Min.X,
			minY:        reserved.Min.Y,
			maxX:        reserved.Max.X,
			maxY:        reserved.Max.Y,
		})
	}

	if len(groups) != len(blocks) {
		return fmt.Errorf("%w: overlap block metadata/member count mismatch", ErrInvalidLayout)
	}
	if len(groups) > 0 && layout.Render == nil {
		return fmt.Errorf("%w: overlap blocks require render settings with recorded padding", ErrInvalidLayout)
	}

	groupIDs := make([]int, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}

	slices.Sort(groupIDs)
	for _, groupID := range groupIDs {
		members := groups[groupID]
		if len(members) < 2 {
			return fmt.Errorf(
				"%w: overlap group %d has %d canonical members, want at least 2",
				ErrInvalidLayout,
				groupID,
				len(members),
			)
		}

		block, exists := blocks[groupID]
		if !exists {
			return fmt.Errorf("%w: overlap group %d has no block metadata", ErrInvalidLayout, groupID)
		}
		bounds, err := validateOverlapBlock(block, members)
		if err != nil {
			return err
		}
		reserved, err := paddedBounds(layout, bounds, padding, fmt.Sprintf("overlap block %d", groupID))
		if err != nil {
			return err
		}

		canonicalRects = append(canonicalRects, occupiedRect{
			placementID: fmt.Sprintf("overlap block %d", groupID),
			minX:        reserved.Min.X,
			minY:        reserved.Min.Y,
			maxX:        reserved.Max.X,
			maxY:        reserved.Max.Y,
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

	// Sweep the wider axis so sorted bounds can prune more pair checks.
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

// validateOverlapBlock checks block bounds and non-overlapping reduced rectangles.
func validateOverlapBlock(block OverlapBlock, members []Placement) (image.Rectangle, error) {
	maxInt := int(^uint(0) >> 1)
	if block.Width <= 0 || block.Height <= 0 ||
		block.X < 0 || block.Y < 0 ||
		block.X > maxInt-block.Width || block.Y > maxInt-block.Height ||
		block.ShareX < 0 || block.ShareY < 0 ||
		block.ShareX == 0 && block.ShareY == 0 {
		return image.Rectangle{}, fmt.Errorf(
			"%w: overlap block %d has invalid geometry or shares",
			ErrInvalidLayout, block.ID,
		)
	}

	blockBounds := image.Rect(block.X, block.Y, block.X+block.Width, block.Y+block.Height)
	var union image.Rectangle
	workRects := make([]image.Rectangle, 0, len(members))

	for index, member := range members {
		if member.Rotated != block.Rotated ||
			block.ShareX >= member.Width ||
			block.ShareY >= member.Height {
			return image.Rectangle{}, fmt.Errorf(
				"%w: overlap block %d member %q has invalid rotation or share",
				ErrInvalidLayout, block.ID, member.ID,
			)
		}

		memberBounds := placementContentBounds(member)
		if blockBounds.Intersect(memberBounds) != memberBounds {
			return image.Rectangle{}, fmt.Errorf(
				"%w: overlap block %d does not contain member %q",
				ErrInvalidLayout, block.ID, member.ID,
			)
		}

		if index == 0 {
			union = memberBounds
		} else {
			union = union.Union(memberBounds)
		}

		localX, localY := member.X-block.X, member.Y-block.Y
		workWidth := member.Width - block.ShareX
		workHeight := member.Height - block.ShareY
		if block.Rotated {
			workWidth, workHeight = workHeight, workWidth
		}

		zoneWidth, zoneHeight := block.Width, block.Height
		memberWidth, memberHeight := member.Width, member.Height
		if block.Rotated {
			memberWidth, memberHeight = memberHeight, memberWidth
		}

		if localX < 0 || localY < 0 || localX > zoneWidth-memberWidth ||
			localY > zoneHeight-memberHeight {
			return image.Rectangle{}, fmt.Errorf(
				"%w: overlap block %d member %q transform is invalid",
				ErrInvalidLayout, block.ID, member.ID,
			)
		}

		workX := localX
		if block.Rotated {
			workX += block.ShareY
		}
		workRects = append(workRects, image.Rect(
			workX,
			localY,
			workX+workWidth,
			localY+workHeight,
		))
	}

	if union != blockBounds {
		return image.Rectangle{}, fmt.Errorf(
			"%w: overlap block %d bounds differ from member union",
			ErrInvalidLayout, block.ID,
		)
	}

	for i := range workRects {
		for j := i + 1; j < len(workRects); j++ {
			if workRects[i].Overlaps(workRects[j]) {
				return image.Rectangle{}, fmt.Errorf(
					"%w: overlap block %d reduced members %d and %d intersect: %v and %v (share=%d,%d rotated=%t)",
					ErrInvalidLayout, block.ID, i, j, workRects[i], workRects[j], block.ShareX, block.ShareY, block.Rotated,
				)
			}
		}
	}

	return blockBounds, nil
}

// paddedPlacementBounds returns one canonical placement's padded atlas bounds.
func paddedPlacementBounds(layout *Layout, placement Placement, padding int) (image.Rectangle, error) {
	return paddedBounds(layout, placementContentBounds(placement), padding, placement.ID)
}

// paddedBounds verifies that a physical rectangle and its padding fit in the atlas.
func paddedBounds(layout *Layout, bounds image.Rectangle, padding int, id string) (image.Rectangle, error) {
	if bounds.Empty() || padding < 0 ||
		padding > bounds.Min.X || padding > bounds.Min.Y ||
		bounds.Max.X > layout.Width || bounds.Max.Y > layout.Height ||
		padding > layout.Width-bounds.Max.X || padding > layout.Height-bounds.Max.Y {
		return image.Rectangle{}, fmt.Errorf(
			"%w: reserved padding exceeds atlas bounds: ID=%q Padding=%d",
			ErrInvalidLayout,
			id,
			padding,
		)
	}

	return image.Rect(
		bounds.Min.X-padding,
		bounds.Min.Y-padding,
		bounds.Max.X+padding,
		bounds.Max.Y+padding,
	), nil
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
		alias.Rotated == canonical.Rotated && alias.OverlapGroup == canonical.OverlapGroup
}

// rectanglesOverlap reports whether two reserved axis-aligned rectangles intersect.
func rectanglesOverlap(a, b occupiedRect) bool {
	return a.minX < b.maxX && b.minX < a.maxX &&
		a.minY < b.maxY && b.minY < a.maxY
}
