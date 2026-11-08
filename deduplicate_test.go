// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"encoding/json"
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"
)

func TestPackDeduplicatesRenderedPixelsAcrossImageTypes(t *testing.T) {
	t.Parallel()

	const (
		width  = 2
		height = 2
	)
	rgba := image.NewRGBA(image.Rect(-3, 7, -1, 9))
	nrgba := image.NewNRGBA(image.Rect(4, -2, 6, 0))
	for y := range height {
		for x := range width {
			rgba.SetRGBA(-3+x, 7+y, color.RGBA{R: 100, G: 50, A: 128})
			nrgba.SetNRGBA(4+x, -2+y, color.NRGBA{R: 200, G: 100, A: 128})
		}
	}

	paletted := image.NewPaletted(image.Rect(10, -9, 12, -7), color.Palette{
		color.RGBA{R: 100, G: 50, A: 128},
	})
	for y := -9; y < -7; y++ {
		for x := 10; x < 12; x++ {
			paletted.SetColorIndex(x, y, 0)
		}
	}

	base := image.NewNRGBA(image.Rect(-20, 30, -16, 34))
	for y := 31; y < 33; y++ {
		for x := -19; x < -17; x++ {
			base.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, A: 128})
		}
	}

	subimage := base.SubImage(image.Rect(-19, 31, -17, 33))
	different := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			different.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
		}
	}

	sprites := []Sprite{
		{ID: "rgba", Image: rgba},
		{ID: "nrgba", Image: nrgba},
		{ID: "paletted", Image: paletted},
		{ID: "subimage", Image: subimage},
		{ID: "different", Image: different},
	}
	atlas, err := Pack(sprites, Options{
		MinSize:     8,
		MaxSize:     8,
		Deduplicate: true,
		Padding:     1,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if got := len(atlas.Layout.Placements); got != len(sprites) {
		t.Fatalf("logical placements = %d, want %d", got, len(sprites))
	}
	if atlas.Layout.Render == nil || atlas.Layout.Render.Padding != 1 {
		t.Fatalf("deduplicated layout render settings = %+v, want padding=1", atlas.Layout.Render)
	}

	canonicalCount := 0
	placementByID := make(map[string]Placement, len(sprites))
	for _, placement := range atlas.Layout.Placements {
		placementByID[placement.ID] = placement
		if placement.AliasOf == "" {
			canonicalCount++
		}
	}
	if canonicalCount != 2 {
		t.Fatalf("canonical placements = %d, want 2", canonicalCount)
	}

	for _, id := range []string{"nrgba", "paletted", "subimage"} {
		if got := placementByID[id].AliasOf; got != "rgba" {
			t.Errorf("AliasOf for %q = %q, want rgba", id, got)
		}
		if !samePlacementGeometry(placementByID[id], placementByID["rgba"]) {
			t.Errorf("alias %q has different physical geometry", id)
		}
	}
	if placementByID["different"].AliasOf != "" {
		t.Fatalf("different pixels became alias of %q", placementByID["different"].AliasOf)
	}

	expected := color.RGBA{R: 100, G: 50, A: 128}
	canonical := placementByID["rgba"]
	if got := atlas.Image.(*image.RGBA).RGBAAt(canonical.X, canonical.Y); got != expected {
		t.Fatalf("canonical rendered pixel = %+v, want %+v", got, expected)
	}
}

func TestPackDedupTrimsBeforeComparingAndPreservesAliasOrigins(t *testing.T) {
	t.Parallel()

	large := image.NewNRGBA(image.Rect(-5, 8, 3, 16))
	for y := 11; y < 13; y++ {
		for x := -3; x < -1; x++ {
			large.SetNRGBA(x, y, color.NRGBA{G: 220, A: 255})
		}
	}

	small := image.NewNRGBA(image.Rect(20, -9, 26, -3))
	for y := -6; y < -4; y++ {
		for x := 21; x < 23; x++ {
			small.SetNRGBA(x, y, color.NRGBA{G: 220, A: 255})
		}
	}

	atlas, err := Pack([]Sprite{
		{ID: "large", Image: large},
		{ID: "small", Image: small},
	}, Options{
		MinSize: 4, MaxSize: 4, ForceSquare: true,
		TrimTransparent: true, Deduplicate: true,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(atlas.Layout.Placements) != 2 {
		t.Fatalf("placements = %d, want 2 logical IDs", len(atlas.Layout.Placements))
	}

	placements := map[string]Placement{}
	for _, placement := range atlas.Layout.Placements {
		placements[placement.ID] = placement
	}

	canonical, alias := placements["large"], placements["small"]
	if alias.AliasOf != "large" || !samePlacementGeometry(alias, canonical) {
		t.Fatalf("small placement = %+v, want alias of large at same geometry", alias)
	}

	if canonical.OriginalWidth != 8 || canonical.OriginalHeight != 8 ||
		canonical.OffsetX != 2 || canonical.OffsetY != 3 {
		t.Fatalf("large source metadata = %+v", canonical)
	}
	if alias.OriginalWidth != 6 || alias.OriginalHeight != 6 ||
		alias.OffsetX != 1 || alias.OffsetY != 3 {
		t.Fatalf("small source metadata = %+v", alias)
	}
	if canonical.Width != 2 || canonical.Height != 2 {
		t.Fatalf("canonical crop = %dx%d, want 2x2", canonical.Width, canonical.Height)
	}
}

func TestPackDedupNormalizesHiddenRGBOnlyAsRendererDoes(t *testing.T) {
	t.Parallel()

	first := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	second := image.NewNRGBA(image.Rect(3, -2, 5, 0))
	for y := range 2 {
		for x := range 2 {
			first.SetNRGBA(x, y, color.NRGBA{R: 255, G: 40, B: 90})
			second.SetNRGBA(3+x, -2+y, color.NRGBA{R: 20, G: 190, B: 50})
		}
	}

	atlas, err := Pack([]Sprite{
		{ID: "a", Image: first},
		{ID: "b", Image: second},
	}, Options{
		MinSize:     4,
		MaxSize:     4,
		Deduplicate: true,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if got := atlas.Layout.Placements[1].AliasOf; got != "a" {
		t.Fatalf("transparent alias = %q, want a", got)
	}
}

func TestPackDedupUsesUnrotatedDimensionsBeforePacking(t *testing.T) {
	t.Parallel()

	wide := image.NewRGBA(image.Rect(0, 0, 2, 3))
	tall := image.NewRGBA(image.Rect(4, -2, 7, 0))
	for y := range 3 {
		for x := range 2 {
			wide.SetRGBA(x, y, color.RGBA{R: uint8(x + y*2), A: 255})
		}
	}
	for y := -2; y < 0; y++ {
		for x := 4; x < 7; x++ {
			tall.SetRGBA(x, y, color.RGBA{R: uint8((x-4)*3 + y + 2), A: 255})
		}
	}

	atlas, err := Pack([]Sprite{
		{ID: "wide", Image: wide},
		{ID: "tall", Image: tall},
	}, Options{
		MinSize:     8,
		MaxSize:     8,
		AllowRotate: true,
		Deduplicate: true,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(atlas.Layout.Placements) != 2 {
		t.Fatalf("placements = %d, want 2 different unrotated dimensions", len(atlas.Layout.Placements))
	}
	for _, placement := range atlas.Layout.Placements {
		if placement.AliasOf != "" {
			t.Fatalf("different source geometry became alias: %+v", placement)
		}
	}
}

func TestDedupExactComparisonRejectsForcedHashCollision(t *testing.T) {
	t.Parallel()

	first := image.NewRGBA(image.Rect(0, 0, 2, 2))
	duplicate := image.NewNRGBA(image.Rect(4, 7, 6, 9))
	different := image.NewRGBA(image.Rect(-2, 3, 0, 5))
	for y := range 2 {
		for x := range 2 {
			first.SetRGBA(x, y, color.RGBA{R: 40, A: 255})
			duplicate.SetNRGBA(4+x, 7+y, color.NRGBA{R: 40, A: 255})
			different.SetRGBA(-2+x, 3+y, color.RGBA{B: 230, A: 255})
		}
	}

	sprites := []Sprite{
		{ID: "first", Image: first},
		{ID: "duplicate", Image: duplicate},
		{ID: "collision", Image: different},
	}
	items, sources, trimByID, err := prepareSpritesWithOptions(sprites, Options{})
	if err != nil {
		t.Fatalf("prepareSprites: %v", err)
	}

	canonical, aliases := deduplicatePreparedSprites(
		items,
		sources,
		trimByID,
		false,
		func(image.Image) uint64 { return 1 },
	)
	if got := []string{canonical[0].ID, canonical[1].ID}; !reflect.DeepEqual(got, []string{"first", "collision"}) {
		t.Fatalf("canonical IDs = %v", got)
	}
	if !reflect.DeepEqual(aliases, []aliasRelation{{id: "duplicate", canonicalID: "first"}}) {
		t.Fatalf("aliases = %+v", aliases)
	}
}

func TestDeduplicateOptionDoesNotChangeGeometryPlanning(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "one", Width: 3, Height: 2},
		{ID: "two", Width: 2, Height: 2},
	}
	without := Options{MinSize: 8, MaxSize: 8}
	with := without
	with.Deduplicate = true

	planWithout, err := Plan(items, without)
	if err != nil {
		t.Fatalf("Plan without dedup: %v", err)
	}

	planWith, err := Plan(items, with)
	if err != nil {
		t.Fatalf("Plan with dedup: %v", err)
	}
	if !reflect.DeepEqual(planWith, planWithout) {
		t.Fatalf("Plan with Deduplicate changed geometry: %+v vs %+v", planWith, planWithout)
	}

	pagesWithout, err := PlanPages(items, without)
	if err != nil {
		t.Fatalf("PlanPages without dedup: %v", err)
	}

	pagesWith, err := PlanPages(items, with)
	if err != nil {
		t.Fatalf("PlanPages with dedup: %v", err)
	}
	if !reflect.DeepEqual(pagesWith, pagesWithout) {
		t.Fatalf("PlanPages with Deduplicate changed geometry")
	}

	atlas, err := Pack([]Sprite{
		{ID: "one", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))},
	}, without)
	if err != nil {
		t.Fatalf("Pack without dedup: %v", err)
	}

	encoded, err := json.Marshal(atlas.Layout)
	if err != nil {
		t.Fatalf("Marshal layout: %v", err)
	}
	if strings.Contains(string(encoded), "alias_of") {
		t.Fatalf("dedup-disabled layout contains alias metadata: %s", encoded)
	}
}
