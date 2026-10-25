// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"fmt"
	"image"
	"slices"
)

// PagesStats contains aggregate metrics for a collection of atlas layouts.
type PagesStats struct {
	// PageCount is the number of layouts analyzed.
	PageCount int `json:"page_count" yaml:"page_count"`

	// LayoutStats contains sums and ratios calculated from aggregate areas.
	LayoutStats `json:",inline" yaml:",inline"`
}

// LayoutReport wraps a layout and its optional derived statistics.
// Embedding preserves the layout's top-level JSON and YAML fields.
type LayoutReport struct {
	// Layout is the layout being reported.
	Layout `json:",inline" yaml:",inline"`

	// Stats contains metrics calculated from Layout without source images.
	Stats LayoutStats `json:"stats" yaml:"stats"`
}

// LayoutStats contains image-independent metrics derived from one layout.
type LayoutStats struct {
	// AtlasArea is the atlas width multiplied by its height.
	AtlasArea int64 `json:"atlas_area" yaml:"atlas_area"`

	// LogicalCount is the number of sprite IDs represented in the layout.
	LogicalCount int64 `json:"logical_count" yaml:"logical_count"`

	// PhysicalCount is the number of canonical sprite placements.
	// An overlap group counts two placements but occupies one planner item.
	PhysicalCount int64 `json:"physical_count" yaml:"physical_count"`

	// AliasCount is the number of placements sharing canonical rectangles.
	AliasCount int64 `json:"alias_count" yaml:"alias_count"`

	// ContentArea is the union area of canonical sprite rectangles.
	ContentArea int64 `json:"content_area" yaml:"content_area"`

	// ReservedArea is the summed canonical area including the requested padding.
	ReservedArea int64 `json:"reserved_area" yaml:"reserved_area"`

	// ContentUtilization is ContentArea divided by AtlasArea.
	ContentUtilization float64 `json:"content_utilization" yaml:"content_utilization"`

	// ReservedUtilization is ReservedArea divided by AtlasArea.
	// It measures occupancy, not optimal packing quality.
	ReservedUtilization float64 `json:"reserved_utilization" yaml:"reserved_utilization"`

	// PhysicalRotationCount counts rotated canonical placements.
	PhysicalRotationCount int64 `json:"physical_rotation_count" yaml:"physical_rotation_count"`

	// LogicalRotationCount counts rotated placements including aliases.
	LogicalRotationCount int64 `json:"logical_rotation_count" yaml:"logical_rotation_count"`

	// LogicalTrimSavedArea sums removed source area for all logical placements.
	// It does not measure atlas pixel or encoded file savings.
	LogicalTrimSavedArea int64 `json:"logical_trim_saved_area" yaml:"logical_trim_saved_area"`

	// PhysicalTrimSavedArea sums removed source area for canonical placements.
	// It does not measure atlas pixel or encoded file savings.
	PhysicalTrimSavedArea int64 `json:"physical_trim_saved_area" yaml:"physical_trim_saved_area"`

	// OverlapGroupCount counts physical N-member overlap blocks.
	OverlapGroupCount int64 `json:"overlap_group_count,omitempty" yaml:"overlap_group_count,omitempty"`

	// OverlapGroupMembers counts canonical placements inside overlap blocks.
	OverlapGroupMembers int64 `json:"overlap_group_members,omitempty" yaml:"overlap_group_members,omitempty"`

	// OverlapBlockArea sums physical bounding areas of overlap blocks.
	OverlapBlockArea int64 `json:"overlap_block_area,omitempty" yaml:"overlap_block_area,omitempty"`

	// OverlapSharedPixels counts member pixels covered more than once.
	OverlapSharedPixels int64 `json:"overlap_shared_pixels,omitempty" yaml:"overlap_shared_pixels,omitempty"`
}

// statsSum identifies one non-negative aggregate field.
type statsSum struct {
	total *int64 // total points to the accumulated result.
	name  string // name labels the field in overflow errors.
	value int64  // value is the current page's contribution.
}

// AnalyzeLayout calculates image-independent statistics for one layout.
//
// Padding is required because legacy layouts do not record it.
// When the layout has Render settings, their recorded padding must match the argument.
func AnalyzeLayout(layout *Layout, padding int) (LayoutStats, error) {
	if layout == nil {
		return LayoutStats{}, ErrLayoutRequired
	}
	if padding < 0 {
		return LayoutStats{}, fmt.Errorf("%w: padding must be non-negative: %d", ErrInvalidOptions, padding)
	}

	if layout.Render != nil && layout.Render.Padding != padding {
		return LayoutStats{}, fmt.Errorf(
			"%w: requested padding=%d differs from layout padding=%d",
			ErrInvalidLayout,
			padding,
			layout.Render.Padding,
		)
	}

	validatedLayout := *layout
	if layout.Render == nil {
		validatedLayout.Render = &RenderSettings{Padding: padding}
	}
	if err := validateLayout(&validatedLayout); err != nil {
		return LayoutStats{}, err
	}

	stats := LayoutStats{
		LogicalCount: int64(len(layout.Placements)),
	}
	groups := make(map[int][]Placement)
	var err error
	stats.AtlasArea, err = checkedArea(int64(layout.Width), int64(layout.Height))
	if err != nil {
		return LayoutStats{}, fmt.Errorf("%w: atlas area: %v", ErrInvalidLayout, err)
	}

	for i := range layout.Placements {
		placement := layout.Placements[i]
		if placement.Rotated {
			stats.LogicalRotationCount++
		}
		if placement.AliasOf == "" {
			stats.PhysicalCount++
			if placement.Rotated {
				stats.PhysicalRotationCount++
			}
			if placement.OverlapGroup > 0 {
				groups[placement.OverlapGroup] = append(groups[placement.OverlapGroup], placement)
			} else {
				if err := addPlacementAreas(&stats, placement, padding); err != nil {
					return LayoutStats{}, err
				}
			}
		}

		trimSaved, trimErr := trimmedAreaSaved(placement)
		if trimErr != nil {
			return LayoutStats{}, fmt.Errorf(
				"%w: trim area for %q: %v",
				ErrInvalidLayout, placement.ID, trimErr,
			)
		}

		stats.LogicalTrimSavedArea, err = checkedAdd(stats.LogicalTrimSavedArea, trimSaved)
		if err != nil {
			return LayoutStats{}, fmt.Errorf(
				"%w: total logical trim area: %v",
				ErrInvalidLayout, err,
			)
		}

		if placement.AliasOf == "" {
			stats.PhysicalTrimSavedArea, err = checkedAdd(stats.PhysicalTrimSavedArea, trimSaved)
			if err != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: total physical trim area: %v",
					ErrInvalidLayout, err,
				)
			}
		}
	}

	if err := addOverlapAreas(&stats, groups, layout.OverlapBlocks, padding); err != nil {
		return LayoutStats{}, err
	}

	stats.AliasCount = stats.LogicalCount - stats.PhysicalCount
	stats.ContentUtilization = float64(stats.ContentArea) / float64(stats.AtlasArea)
	stats.ReservedUtilization = float64(stats.ReservedArea) / float64(stats.AtlasArea)

	return stats, nil
}

// addPlacementAreas accumulates content and reserved areas for one non-group member.
func addPlacementAreas(stats *LayoutStats, placement Placement, padding int) error {
	contentArea, err := checkedArea(int64(placement.Width), int64(placement.Height))
	if err != nil {
		return fmt.Errorf("%w: content area for %q: %v", ErrInvalidLayout, placement.ID, err)
	}
	stats.ContentArea, err = checkedAdd(stats.ContentArea, contentArea)
	if err != nil {
		return fmt.Errorf("%w: total content area: %v", ErrInvalidLayout, err)
	}

	reservedWidth, err := checkedPaddedDimension(int64(placement.Width), int64(padding))
	if err != nil {
		return fmt.Errorf("%w: reserved width for %q: %v", ErrInvalidLayout, placement.ID, err)
	}
	reservedHeight, err := checkedPaddedDimension(int64(placement.Height), int64(padding))
	if err != nil {
		return fmt.Errorf("%w: reserved height for %q: %v", ErrInvalidLayout, placement.ID, err)
	}

	reservedArea, err := checkedArea(reservedWidth, reservedHeight)
	if err != nil {
		return fmt.Errorf("%w: reserved area for %q: %v", ErrInvalidLayout, placement.ID, err)
	}
	stats.ReservedArea, err = checkedAdd(stats.ReservedArea, reservedArea)
	if err != nil {
		return fmt.Errorf("%w: total reserved area: %v", ErrInvalidLayout, err)
	}

	return nil
}

// addOverlapAreas accumulates member unions and one padded bbox per overlap block.
func addOverlapAreas(stats *LayoutStats, groups map[int][]Placement, blocks []OverlapBlock, padding int) error {
	groupIDs := make([]int, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}
	slices.Sort(groupIDs)
	blockByID := make(map[int]OverlapBlock, len(blocks))
	for _, block := range blocks {
		blockByID[block.ID] = block
	}

	for _, groupID := range groupIDs {
		members := groups[groupID]
		block, exists := blockByID[groupID]
		if !exists || len(members) < 2 {
			return fmt.Errorf("%w: overlap group %d metadata or members are missing", ErrInvalidLayout, groupID)
		}
		rectangles := make([]image.Rectangle, 0, len(members))
		var memberArea int64
		for _, member := range members {
			area, err := checkedArea(int64(member.Width), int64(member.Height))
			if err != nil {
				return fmt.Errorf("%w: overlap member area: %v", ErrInvalidLayout, err)
			}

			memberArea, err = checkedAdd(memberArea, area)
			if err != nil {
				return fmt.Errorf("%w: total overlap member area: %v", ErrInvalidLayout, err)
			}

			rectangles = append(rectangles, placementContentBounds(member))
		}

		unionArea, err := rectangleUnionArea(rectangles)
		if err != nil || unionArea > memberArea {
			return fmt.Errorf("%w: overlap union area is not representable", ErrInvalidLayout)
		}

		sharedArea := memberArea - unionArea
		stats.ContentArea, err = checkedAdd(stats.ContentArea, unionArea)
		if err != nil {
			return fmt.Errorf("%w: total content area: %v", ErrInvalidLayout, err)
		}

		stats.OverlapSharedPixels, err = checkedAdd(stats.OverlapSharedPixels, sharedArea)
		if err != nil {
			return fmt.Errorf("%w: total shared overlap pixels: %v", ErrInvalidLayout, err)
		}

		stats.OverlapGroupCount++
		stats.OverlapGroupMembers, err = checkedAdd(stats.OverlapGroupMembers, int64(len(members)))
		if err != nil {
			return fmt.Errorf("%w: total overlap members: %v", ErrInvalidLayout, err)
		}

		blockArea, err := checkedArea(int64(block.Width), int64(block.Height))
		if err != nil {
			return fmt.Errorf("%w: overlap block area: %v", ErrInvalidLayout, err)
		}

		stats.OverlapBlockArea, err = checkedAdd(stats.OverlapBlockArea, blockArea)
		if err != nil {
			return fmt.Errorf("%w: total overlap block area: %v", ErrInvalidLayout, err)
		}

		reservedWidth, err := checkedPaddedDimension(int64(block.Width), int64(padding))
		if err != nil {
			return fmt.Errorf("%w: overlap group reserved width: %v", ErrInvalidLayout, err)
		}

		reservedHeight, err := checkedPaddedDimension(int64(block.Height), int64(padding))
		if err != nil {
			return fmt.Errorf("%w: overlap group reserved height: %v", ErrInvalidLayout, err)
		}

		reservedArea, err := checkedArea(reservedWidth, reservedHeight)
		if err != nil {
			return fmt.Errorf("%w: overlap group reserved area: %v", ErrInvalidLayout, err)
		}

		stats.ReservedArea, err = checkedAdd(stats.ReservedArea, reservedArea)
		if err != nil {
			return fmt.Errorf("%w: total reserved area: %v", ErrInvalidLayout, err)
		}
	}

	return nil
}

// rectangleUnionArea sweeps rectangle Y boundaries and unions X intervals per slab.
func rectangleUnionArea(rectangles []image.Rectangle) (int64, error) {
	if len(rectangles) == 0 {
		return 0, nil
	}

	boundaries := make([]int, 0, len(rectangles)*2)
	for _, rectangle := range rectangles {
		if rectangle.Empty() {
			return 0, errors.New("empty rectangle in overlap union")
		}

		boundaries = append(boundaries, rectangle.Min.Y, rectangle.Max.Y)
	}

	slices.Sort(boundaries)
	boundaries = slices.Compact(boundaries)
	var union int64

	for index := 0; index+1 < len(boundaries); index++ {
		top, bottom := boundaries[index], boundaries[index+1]
		if top == bottom {
			continue
		}

		intervals := make([][2]int, 0, len(rectangles))
		for _, rectangle := range rectangles {
			if rectangle.Min.Y <= top && rectangle.Max.Y >= bottom {
				intervals = append(intervals, [2]int{rectangle.Min.X, rectangle.Max.X})
			}
		}

		slices.SortFunc(intervals, func(a, b [2]int) int {
			if a[0] != b[0] {
				return a[0] - b[0]
			}
			return a[1] - b[1]
		})

		var coveredWidth int64
		if len(intervals) > 0 {
			left, right := intervals[0][0], intervals[0][1]
			for _, interval := range intervals[1:] {
				if interval[0] > right {
					coveredWidth += int64(right - left)
					left, right = interval[0], interval[1]
				} else {
					right = max(right, interval[1])
				}
			}
			coveredWidth += int64(right - left)
		}

		slabArea, err := checkedArea(coveredWidth, int64(bottom-top))
		if err != nil {
			return 0, err
		}

		union, err = checkedAdd(union, slabArea)
		if err != nil {
			return 0, err
		}
	}

	return union, nil
}

// AnalyzePages calculates aggregate statistics without averaging page ratios.
func AnalyzePages(layouts []Layout, padding int) (PagesStats, error) {
	if padding < 0 {
		return PagesStats{}, fmt.Errorf(
			"%w: padding must be non-negative: %d",
			ErrInvalidOptions, padding,
		)
	}

	stats := PagesStats{PageCount: len(layouts)}
	seenIDs := make(map[string]struct{})
	for pageIndex := range layouts {
		layoutStats, err := AnalyzeLayout(&layouts[pageIndex], padding)
		if err != nil {
			return PagesStats{}, fmt.Errorf(
				"analyze page %d: %w", pageIndex, err,
			)
		}

		for i := range layouts[pageIndex].Placements {
			id := layouts[pageIndex].Placements[i].ID
			if _, exists := seenIDs[id]; exists {
				return PagesStats{}, fmt.Errorf(
					"%w: duplicate placement ID %q across pages",
					ErrInvalidLayout, id,
				)
			}

			seenIDs[id] = struct{}{}
		}

		if err := addLayoutStats(&stats.LayoutStats, layoutStats); err != nil {
			return PagesStats{}, err
		}
	}

	if stats.AtlasArea > 0 {
		stats.ContentUtilization = float64(stats.ContentArea) / float64(stats.AtlasArea)
		stats.ReservedUtilization = float64(stats.ReservedArea) / float64(stats.AtlasArea)
	}

	return stats, nil
}

// addLayoutStats combines sums while deferring aggregate ratio calculation.
func addLayoutStats(total *LayoutStats, page LayoutStats) error {
	fields := []statsSum{
		{name: "atlas area", total: &total.AtlasArea, value: page.AtlasArea},
		{name: "logical count", total: &total.LogicalCount, value: page.LogicalCount},
		{name: "physical count", total: &total.PhysicalCount, value: page.PhysicalCount},
		{name: "alias count", total: &total.AliasCount, value: page.AliasCount},
		{name: "content area", total: &total.ContentArea, value: page.ContentArea},
		{name: "reserved area", total: &total.ReservedArea, value: page.ReservedArea},
		{name: "physical rotations", total: &total.PhysicalRotationCount, value: page.PhysicalRotationCount},
		{name: "logical rotations", total: &total.LogicalRotationCount, value: page.LogicalRotationCount},
		{name: "logical trim area", total: &total.LogicalTrimSavedArea, value: page.LogicalTrimSavedArea},
		{name: "physical trim area", total: &total.PhysicalTrimSavedArea, value: page.PhysicalTrimSavedArea},
		{name: "overlap groups", total: &total.OverlapGroupCount, value: page.OverlapGroupCount},
		{name: "overlap members", total: &total.OverlapGroupMembers, value: page.OverlapGroupMembers},
		{name: "overlap block area", total: &total.OverlapBlockArea, value: page.OverlapBlockArea},
		{name: "shared overlap pixels", total: &total.OverlapSharedPixels, value: page.OverlapSharedPixels},
	}

	for _, field := range fields {
		value, err := checkedAdd(*field.total, field.value)
		if err != nil {
			return fmt.Errorf("%w: aggregate %s: %v", ErrInvalidLayout, field.name, err)
		}

		*field.total = value
	}

	return nil
}

// trimmedAreaSaved returns the original canvas area minus the packed crop.
func trimmedAreaSaved(placement Placement) (int64, error) {
	if placement.OriginalWidth == 0 && placement.OriginalHeight == 0 {
		return 0, nil
	}

	originalArea, err := checkedArea(int64(placement.OriginalWidth), int64(placement.OriginalHeight))
	if err != nil {
		return 0, err
	}
	contentArea, err := checkedArea(int64(placement.Width), int64(placement.Height))
	if err != nil {
		return 0, err
	}

	if contentArea > originalArea {
		return 0, fmt.Errorf("crop area %d exceeds original area %d", contentArea, originalArea)
	}

	return originalArea - contentArea, nil
}

// checkedPaddedDimension adds both sides of padding without overflowing.
func checkedPaddedDimension(dimension, padding int64) (int64, error) {
	if dimension <= 0 || padding < 0 || padding > (int64(^uint64(0)>>1)-dimension)/2 {
		return 0, fmt.Errorf("dimension=%d padding=%d is not representable", dimension, padding)
	}

	return dimension + 2*padding, nil
}

// checkedArea multiplies non-negative dimensions with int64 overflow detection.
func checkedArea(width, height int64) (int64, error) {
	if width < 0 || height < 0 {
		return 0, fmt.Errorf("negative dimensions %dx%d", width, height)
	}
	if width != 0 && height > int64(^uint64(0)>>1)/width {
		return 0, fmt.Errorf("area %dx%d is not representable", width, height)
	}

	return width * height, nil
}

// checkedAdd adds non-negative statistics while detecting int64 overflow.
func checkedAdd(a, b int64) (int64, error) {
	if a < 0 || b < 0 || b > int64(^uint64(0)>>1)-a {
		return 0, fmt.Errorf("sum %d+%d is not representable", a, b)
	}

	return a + b, nil
}
