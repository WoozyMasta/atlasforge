// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestAnalyzeLayoutCalculatesLegacyPaddingStats(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:      8,
		Height:     8,
		Placements: []Placement{{ID: "icon", X: 1, Y: 1, Width: 2, Height: 3}},
	}

	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("Marshal legacy layout: %v", err)
	}

	var decoded Layout
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal legacy layout: %v", err)
	}

	stats, err := AnalyzeLayout(&decoded, 1)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	if stats.AtlasArea != 64 || stats.ContentArea != 6 || stats.ReservedArea != 20 {
		t.Fatalf(
			"areas = atlas:%d content:%d reserved:%d",
			stats.AtlasArea, stats.ContentArea, stats.ReservedArea,
		)
	}
	if stats.LogicalCount != 1 || stats.PhysicalCount != 1 || stats.AliasCount != 0 {
		t.Fatalf(
			"counts = logical:%d physical:%d aliases:%d",
			stats.LogicalCount, stats.PhysicalCount, stats.AliasCount,
		)
	}
	if stats.ContentUtilization != 6.0/64 || stats.ReservedUtilization != 20.0/64 {
		t.Fatalf(
			"utilization = content:%v reserved:%v",
			stats.ContentUtilization, stats.ReservedUtilization,
		)
	}

	if decoded.Render != nil {
		t.Fatal("AnalyzeLayout mutated legacy layout render settings")
	}
}

func TestAnalyzeLayoutCountsTrimmedAliasesAndRotations(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  8,
		Height: 8,
		Render: &RenderSettings{Padding: 1, TrimTransparent: true},
		Placements: []Placement{
			{
				ID:             "canonical",
				X:              1,
				Y:              1,
				Width:          2,
				Height:         2,
				Rotated:        true,
				OriginalWidth:  4,
				OriginalHeight: 4,
			},
			{
				ID:             "alias",
				AliasOf:        "canonical",
				X:              1,
				Y:              1,
				Width:          2,
				Height:         2,
				Rotated:        true,
				OriginalWidth:  6,
				OriginalHeight: 6,
				OffsetX:        2,
				OffsetY:        3,
			},
		},
	}
	stats, err := AnalyzeLayout(layout, 1)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	if stats.LogicalCount != 2 || stats.PhysicalCount != 1 || stats.AliasCount != 1 {
		t.Fatalf(
			"counts = logical:%d physical:%d aliases:%d",
			stats.LogicalCount, stats.PhysicalCount, stats.AliasCount,
		)
	}
	if stats.ContentArea != 4 || stats.ReservedArea != 16 {
		t.Fatalf(
			"areas = content:%d reserved:%d",
			stats.ContentArea, stats.ReservedArea,
		)
	}
	if stats.LogicalRotationCount != 2 || stats.PhysicalRotationCount != 1 {
		t.Fatalf(
			"rotations = logical:%d physical:%d",
			stats.LogicalRotationCount, stats.PhysicalRotationCount,
		)
	}
	if stats.LogicalTrimSavedArea != 44 || stats.PhysicalTrimSavedArea != 12 {
		t.Fatalf(
			"trim saved = logical:%d physical:%d",
			stats.LogicalTrimSavedArea, stats.PhysicalTrimSavedArea,
		)
	}
}

func TestAnalyzeLayoutCountsFullyTransparentTrimArea(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  4,
		Height: 4,
		Render: &RenderSettings{TrimTransparent: true},
		Placements: []Placement{{
			ID:             "empty",
			Width:          1,
			Height:         1,
			OriginalWidth:  100,
			OriginalHeight: 50,
		}},
	}
	stats, err := AnalyzeLayout(layout, 0)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	if stats.LogicalTrimSavedArea != 4999 || stats.PhysicalTrimSavedArea != 4999 {
		t.Fatalf("trim saved = logical:%d physical:%d, want 4999 each",
			stats.LogicalTrimSavedArea, stats.PhysicalTrimSavedArea,
		)
	}
}

func TestAnalyzePagesAggregatesAreasBeforeRatios(t *testing.T) {
	t.Parallel()

	layouts := []Layout{{
		Width:      8,
		Height:     8,
		Placements: []Placement{{ID: "first", X: 0, Y: 0, Width: 2, Height: 3}},
	}, {
		Width:      16,
		Height:     16,
		Placements: []Placement{{ID: "second", X: 0, Y: 0, Width: 4, Height: 2}},
	}}
	stats, err := AnalyzePages(layouts, 0)
	if err != nil {
		t.Fatalf("AnalyzePages: %v", err)
	}

	if stats.PageCount != 2 || stats.AtlasArea != 320 || stats.ContentArea != 14 {
		t.Fatalf(
			"pages/areas = %d/%d/%d",
			stats.PageCount, stats.AtlasArea, stats.ContentArea,
		)
	}
	if stats.ContentUtilization != 14.0/320 || stats.ReservedUtilization != 14.0/320 {
		t.Fatalf(
			"aggregate utilization = content:%v reserved:%v",
			stats.ContentUtilization, stats.ReservedUtilization,
		)
	}

	first, err := AnalyzeLayout(&layouts[0], 0)
	if err != nil {
		t.Fatalf("AnalyzeLayout first: %v", err)
	}

	second, err := AnalyzeLayout(&layouts[1], 0)
	if err != nil {
		t.Fatalf("AnalyzeLayout second: %v", err)
	}
	if stats.ContentUtilization == (first.ContentUtilization+second.ContentUtilization)/2 {
		t.Fatal("aggregate utilization unexpectedly averages page ratios")
	}

	empty, err := AnalyzePages(nil, 0)
	if err != nil {
		t.Fatalf("AnalyzePages empty: %v", err)
	}

	if empty.PageCount != 0 ||
		empty.AtlasArea != 0 ||
		empty.ContentUtilization != 0 ||
		empty.ReservedUtilization != 0 {
		t.Fatalf("empty stats = %+v", empty)
	}
}

func TestAnalyzeLayoutReportsFlattenedJSONWithoutChangingLayoutJSON(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:      4,
		Height:     4,
		Placements: []Placement{{ID: "icon", Width: 2, Height: 2}},
	}
	stats, err := AnalyzeLayout(layout, 0)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	legacyJSON, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("Marshal layout: %v", err)
	}

	reportJSON, err := json.Marshal(LayoutReport{Layout: *layout, Stats: stats})
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(legacyJSON), "stats") {
		t.Fatalf("ordinary layout JSON contains stats: %s", legacyJSON)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(reportJSON, &fields); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if _, ok := fields["layout"]; ok {
		t.Fatalf("report nested layout instead of flattening it: %s", reportJSON)
	}
	if _, ok := fields["width"]; !ok {
		t.Fatalf("report has no top-level width: %s", reportJSON)
	}
	if _, ok := fields["stats"]; !ok {
		t.Fatalf("report has no stats field: %s", reportJSON)
	}

	var decoded LayoutReport
	if err := json.Unmarshal(reportJSON, &decoded); err != nil {
		t.Fatalf("Unmarshal report DTO: %v", err)
	}
	if decoded.Width != layout.Width || decoded.Stats.ContentArea != stats.ContentArea {
		t.Fatalf("report round trip = %+v", decoded)
	}

	var pagesJSON map[string]json.RawMessage
	encodedPages, err := json.Marshal(PagesStats{PageCount: 1, LayoutStats: stats})
	if err != nil {
		t.Fatalf("Marshal page stats: %v", err)
	}
	if err := json.Unmarshal(encodedPages, &pagesJSON); err != nil {
		t.Fatalf("Unmarshal page stats: %v", err)
	}
	if _, ok := pagesJSON["atlas_area"]; !ok {
		t.Fatalf("page stats not flattened: %s", encodedPages)
	}
}

func TestAnalyzeLayoutRejectsInvalidPaddingLayoutsAndOverflow(t *testing.T) {
	t.Parallel()

	layout := &Layout{Width: 4, Height: 4, Placements: []Placement{{ID: "icon", Width: 1, Height: 1}}}
	if _, err := AnalyzeLayout(nil, 0); !errors.Is(err, ErrLayoutRequired) {
		t.Fatalf("AnalyzeLayout nil error = %v, want ErrLayoutRequired", err)
	}
	if _, err := AnalyzeLayout(layout, -1); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("AnalyzeLayout negative padding error = %v, want ErrInvalidOptions", err)
	}
	if _, err := AnalyzePages(nil, -1); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("AnalyzePages negative padding error = %v, want ErrInvalidOptions", err)
	}

	withRender := *layout
	withRender.Render = &RenderSettings{Padding: 1}
	if _, err := AnalyzeLayout(&withRender, 0); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("padding mismatch error = %v, want ErrInvalidLayout", err)
	}

	overlapping := &Layout{
		Width:  4,
		Height: 4,
		Placements: []Placement{
			{ID: "first", Width: 2, Height: 2},
			{ID: "second", X: 1, Width: 2, Height: 2},
		},
	}
	if _, err := AnalyzeLayout(overlapping, 0); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("overlap error = %v, want ErrInvalidLayout", err)
	}

	badAlias := &Layout{
		Width:      4,
		Height:     4,
		Placements: []Placement{{ID: "alias", AliasOf: "missing", Width: 1, Height: 1}},
	}
	if _, err := AnalyzeLayout(badAlias, 0); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("alias error = %v, want ErrInvalidLayout", err)
	}
	if _, err := checkedArea(math.MaxInt64, 2); err == nil {
		t.Fatal("checkedArea accepted overflowing dimensions")
	}
	if _, err := checkedAdd(math.MaxInt64, 1); err == nil {
		t.Fatal("checkedAdd accepted overflowing sum")
	}
	if _, err := checkedPaddedDimension(math.MaxInt64, 1); err == nil {
		t.Fatal("checkedPaddedDimension accepted overflowing dimension")
	}

	maxInt := int(^uint(0) >> 1)
	if maxInt > math.MaxInt32 {
		oversizedTrim := &Layout{
			Width: 2, Height: 2,
			Placements: []Placement{{
				ID: "huge-origin", Width: 1, Height: 1,
				OriginalWidth: maxInt, OriginalHeight: maxInt,
			}},
		}
		if _, err := AnalyzeLayout(oversizedTrim, 0); !errors.Is(err, ErrInvalidLayout) {
			t.Fatalf("overflowing trim area error = %v, want ErrInvalidLayout", err)
		}
	}
}

func TestAnalyzePagesRejectsDuplicateIDsAcrossPages(t *testing.T) {
	t.Parallel()

	layout := Layout{
		Width:      2,
		Height:     2,
		Placements: []Placement{{ID: "same", Width: 1, Height: 1}},
	}
	if _, err := AnalyzePages([]Layout{layout, layout}, 0); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("AnalyzePages duplicate IDs error = %v, want ErrInvalidLayout", err)
	}
}

func TestAnalyzePagesSumsAliasAndTrimMetrics(t *testing.T) {
	t.Parallel()

	layouts := []Layout{{
		Width:  8,
		Height: 8,
		Render: &RenderSettings{TrimTransparent: true},
		Placements: []Placement{
			{
				ID:             "canonical",
				Width:          2,
				Height:         2,
				OriginalWidth:  4,
				OriginalHeight: 4,
			},
			{
				ID:             "alias",
				AliasOf:        "canonical",
				Width:          2,
				Height:         2,
				OriginalWidth:  6,
				OriginalHeight: 6,
				OffsetX:        2,
				OffsetY:        3,
			},
		},
	}, {
		Width:  8,
		Height: 8,
		Placements: []Placement{{
			ID:      "rotated",
			Width:   2,
			Height:  1,
			Rotated: true,
		}},
	}}
	stats, err := AnalyzePages(layouts, 0)
	if err != nil {
		t.Fatalf("AnalyzePages: %v", err)
	}

	if stats.LogicalCount != 3 || stats.PhysicalCount != 2 || stats.AliasCount != 1 {
		t.Fatalf(
			"counts = logical:%d physical:%d aliases:%d",
			stats.LogicalCount, stats.PhysicalCount, stats.AliasCount,
		)
	}
	if stats.LogicalTrimSavedArea != 44 || stats.PhysicalTrimSavedArea != 12 {
		t.Fatalf(
			"trim saved = logical:%d physical:%d",
			stats.LogicalTrimSavedArea, stats.PhysicalTrimSavedArea,
		)
	}
	if stats.LogicalRotationCount != 1 || stats.PhysicalRotationCount != 1 {
		t.Fatalf(
			"rotations = logical:%d physical:%d",
			stats.LogicalRotationCount, stats.PhysicalRotationCount,
		)
	}
}

func ExampleAnalyzeLayout() {
	layout := &Layout{
		Width:      4,
		Height:     4,
		Placements: []Placement{{ID: "icon", X: 1, Y: 1, Width: 2, Height: 2}},
	}
	stats, err := AnalyzeLayout(layout, 1)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf(
		"content=%d/%d utilization=%.2f\n",
		stats.ContentArea, stats.AtlasArea, stats.ContentUtilization,
	)
	// Output: content=4/16 utilization=0.25
}
