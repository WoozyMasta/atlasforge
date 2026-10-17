// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "fmt"

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

	// PhysicalCount is the number of canonical packed rectangles.
	PhysicalCount int64 `json:"physical_count" yaml:"physical_count"`

	// AliasCount is the number of placements sharing canonical rectangles.
	AliasCount int64 `json:"alias_count" yaml:"alias_count"`

	// ContentArea is the summed area of canonical sprite rectangles.
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

			contentArea, areaErr := checkedArea(int64(placement.Width), int64(placement.Height))
			if areaErr != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: content area for %q: %v",
					ErrInvalidLayout, placement.ID, areaErr,
				)
			}

			stats.ContentArea, err = checkedAdd(stats.ContentArea, contentArea)
			if err != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: total content area: %v",
					ErrInvalidLayout, err,
				)
			}

			reservedWidth, widthErr := checkedPaddedDimension(int64(placement.Width), int64(padding))
			if widthErr != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: reserved width for %q: %v",
					ErrInvalidLayout, placement.ID, widthErr,
				)
			}

			reservedHeight, heightErr := checkedPaddedDimension(int64(placement.Height), int64(padding))
			if heightErr != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: reserved height for %q: %v", ErrInvalidLayout, placement.ID, heightErr)
			}

			reservedArea, areaErr := checkedArea(reservedWidth, reservedHeight)
			if areaErr != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: reserved area for %q: %v",
					ErrInvalidLayout, placement.ID, areaErr,
				)
			}

			stats.ReservedArea, err = checkedAdd(stats.ReservedArea, reservedArea)
			if err != nil {
				return LayoutStats{}, fmt.Errorf(
					"%w: total reserved area: %v",
					ErrInvalidLayout, err,
				)
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

	stats.AliasCount = stats.LogicalCount - stats.PhysicalCount
	stats.ContentUtilization = float64(stats.ContentArea) / float64(stats.AtlasArea)
	stats.ReservedUtilization = float64(stats.ReservedArea) / float64(stats.AtlasArea)

	return stats, nil
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
