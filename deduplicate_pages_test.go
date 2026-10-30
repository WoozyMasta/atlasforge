// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"reflect"
	"testing"
)

func TestPackPagesAndBuildPagesDeduplicateAcrossPages(t *testing.T) {
	t.Parallel()

	items := make([]Item, 5)
	sprites := make([]Sprite, len(items))
	sources := make(map[string]image.Image, len(items))
	for i := range items {
		id := fmt.Sprintf("sprite-%d", i)
		img := image.NewRGBA(image.Rect(-2, 5, 0, 7))
		component := uint8(30 + i*25)
		if i == 4 {
			component = 30
		}

		for y := 5; y < 7; y++ {
			for x := -2; x < 0; x++ {
				img.SetRGBA(x, y, color.RGBA{R: component, A: 255})
			}
		}

		items[i] = Item{ID: id, Width: 2, Height: 2}
		sprites[i] = Sprite{ID: id, Image: img}
		sources[id] = img
	}
	opts := Options{
		MinSize:     4,
		MaxSize:     4,
		ForceSquare: true,
		Heuristic:   HeuristicFirstFit,
		Deduplicate: true,
	}

	packed, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 1 {
		t.Fatalf("PackPages pages = %d, want 1 after dedup", len(packed))
	}
	if len(packed[0].Layout.Placements) != len(items) {
		t.Fatalf("PackPages placements = %d, want %d logical IDs",
			len(packed[0].Layout.Placements), len(items))
	}

	physicalCount := 0
	for _, placement := range packed[0].Layout.Placements {
		if placement.AliasOf == "" {
			physicalCount++
		}
	}
	if physicalCount != 4 {
		t.Fatalf("physical placements = %d, want 4", physicalCount)
	}

	loadCounts := make(map[string]int, len(items))
	builtPages := 0
	err = BuildPages(items, opts, func(id string) (image.Image, error) {
		loadCounts[id]++
		return sources[id], nil
	}, func(pageIndex int, got *Atlas) error {
		if pageIndex != 0 || builtPages != 0 {
			return fmt.Errorf("unexpected page index %d", pageIndex)
		}
		if !reflect.DeepEqual(got.Layout, packed[0].Layout) {
			return errors.New("BuildPages layout differs from PackPages")
		}
		if !bytes.Equal(got.Image.(*image.RGBA).Pix, packed[0].Image.(*image.RGBA).Pix) {
			return errors.New("BuildPages image differs from PackPages")
		}

		builtPages++
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if builtPages != 1 {
		t.Fatalf("BuildPages pages = %d, want 1", builtPages)
	}

	for _, item := range items {
		want := 2
		switch item.ID {
		case "sprite-0":
			want = 3
		case "sprite-4":
			want = 1
		}

		if loadCounts[item.ID] != want {
			t.Errorf("loader calls for %q = %d, want %d", item.ID, loadCounts[item.ID], want)
		}
	}
}

func TestPackPagesKeepsAliasWithCanonicalPage(t *testing.T) {
	t.Parallel()

	items := make([]Item, 66)
	sprites := make([]Sprite, len(items))
	sources := make(map[string]image.Image, len(items))

	for i := range items {
		id := fmt.Sprintf("page-sprite-%02d", i)
		pixel := color.RGBA{R: uint8(i), G: uint8(i >> 8), A: 255}
		if i == len(items)-1 {
			pixel = color.RGBA{A: 255}
		}
		img := solidRGBA(32, 32, pixel)
		items[i] = Item{ID: id, Width: 32, Height: 32}
		sprites[i] = Sprite{ID: id, Image: img}
		sources[id] = img
	}
	opts := pagePackingTestOptions()
	opts.Deduplicate = true
	packed, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 2 {
		t.Fatalf("PackPages pages = %d, want 2", len(packed))
	}

	placementPage := make(map[string]int, len(items))
	for pageIndex := range packed {
		for _, placement := range packed[pageIndex].Layout.Placements {
			placementPage[placement.ID] = pageIndex
		}
	}

	canonicalID := items[0].ID
	aliasID := items[len(items)-1].ID
	if placementPage[canonicalID] != 0 || placementPage[aliasID] != placementPage[canonicalID] {
		t.Fatalf(
			"canonical page=%d alias page=%d, want both on page 0",
			placementPage[canonicalID], placementPage[aliasID],
		)
	}

	for _, atlas := range packed {
		for _, placement := range atlas.Layout.Placements {
			if placement.ID == aliasID && placement.AliasOf != canonicalID {
				t.Fatalf("alias placement = %+v, want alias of %q", placement, canonicalID)
			}
		}
	}

	pageCount := 0
	err = BuildPages(items, opts, func(id string) (image.Image, error) {
		return sources[id], nil
	}, func(pageIndex int, got *Atlas) error {
		if pageIndex >= len(packed) || !reflect.DeepEqual(got.Layout, packed[pageIndex].Layout) {
			return fmt.Errorf("BuildPages layout differs on page %d", pageIndex)
		}
		pageCount++
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if pageCount != len(packed) {
		t.Fatalf("BuildPages pages = %d, want %d", pageCount, len(packed))
	}
}

func TestPackPagesAndBuildPagesCombineTrimDedupAndExtrusion(t *testing.T) {
	t.Parallel()

	first := image.NewNRGBA(image.Rect(-3, 6, 2, 11))
	second := image.NewNRGBA(image.Rect(12, -8, 18, -3))
	for y := 7; y < 9; y++ {
		for x := -2; x < 0; x++ {
			first.SetNRGBA(x, y, color.NRGBA{R: 200, G: 80, A: 255})
		}
	}
	for y := -6; y < -4; y++ {
		for x := 15; x < 17; x++ {
			second.SetNRGBA(x, y, color.NRGBA{R: 200, G: 80, A: 255})
		}
	}

	sprites := []Sprite{{ID: "first", Image: first}, {ID: "second", Image: second}}
	items := []Item{{ID: "first", Width: 5, Height: 5}, {ID: "second", Width: 6, Height: 5}}
	opts := Options{
		MinSize:         6,
		MaxSize:         6,
		Padding:         1,
		Extrude:         1,
		ForceSquare:     true,
		TrimTransparent: true,
		Deduplicate:     true,
	}

	packed, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 1 {
		t.Fatalf("PackPages pages = %d, want 1", len(packed))
	}
	if len(packed[0].Layout.Placements) != 2 {
		t.Fatalf("PackPages placements = %d, want 2", len(packed[0].Layout.Placements))
	}

	placements := map[string]Placement{}
	for _, placement := range packed[0].Layout.Placements {
		placements[placement.ID] = placement
	}

	canonical, alias := placements["first"], placements["second"]
	if alias.AliasOf != "first" || !samePlacementGeometry(alias, canonical) {
		t.Fatalf("second placement = %+v, want alias of first", alias)
	}

	if canonical.OffsetX != 1 || canonical.OffsetY != 1 ||
		alias.OriginalWidth != 6 || alias.OriginalHeight != 5 ||
		alias.OffsetX != 3 || alias.OffsetY != 2 {
		t.Fatalf("trim metadata canonical=%+v alias=%+v", canonical, alias)
	}

	if got, want := packed[0].Image.(*image.RGBA).RGBAAt(canonical.X-1, canonical.Y-1),
		(color.RGBA{R: 200, G: 80, A: 255}); got != want {
		t.Fatalf("extruded pixel = %+v, want %+v", got, want)
	}

	loadCounts := make(map[string]int, len(items))
	consumed := 0
	err = BuildPages(items, opts, func(id string) (image.Image, error) {
		loadCounts[id]++
		if id == "first" {
			return first, nil
		}
		return second, nil
	}, func(pageIndex int, got *Atlas) error {
		if pageIndex != 0 || consumed != 0 {
			return fmt.Errorf("unexpected page index %d", pageIndex)
		}
		if !reflect.DeepEqual(got.Layout, packed[0].Layout) {
			return errors.New("BuildPages layout differs from PackPages")
		}
		if !bytes.Equal(got.Image.(*image.RGBA).Pix, packed[0].Image.(*image.RGBA).Pix) {
			return errors.New("BuildPages pixels differ from PackPages")
		}

		consumed++
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}

	if consumed != 1 || loadCounts["first"] != 3 || loadCounts["second"] != 1 {
		t.Fatalf(
			"consumed=%d loader calls=%v, want one page and first=3 second=1",
			consumed, loadCounts,
		)
	}
}

func TestBuildPagesDedupCollisionAndComparisonErrors(t *testing.T) {
	t.Parallel()

	items := []Item{{ID: "first", Width: 2, Height: 2}, {ID: "second", Width: 2, Height: 2}}
	sources := map[string]image.Image{
		"first":  solidRGBA(2, 2, color.RGBA{R: 255, A: 255}),
		"second": solidRGBA(2, 2, color.RGBA{B: 255, A: 255}),
	}
	constantHash := func(image.Image) uint64 { return 7 }
	loadCounts := make(map[string]int)

	canonical, trimByID, aliases, err := prepareDeduplicatedItems(
		items, Options{}, func(id string) (image.Image, error) {
			loadCounts[id]++
			return sources[id], nil
		}, constantHash,
	)
	if err != nil {
		t.Fatalf("prepareDeduplicatedItems: %v", err)
	}
	if len(canonical) != 2 || len(aliases) != 0 || len(trimByID) != 0 {
		t.Fatalf("canonical=%v aliases=%v trim metadata=%v", canonical, aliases, trimByID)
	}
	if loadCounts["first"] != 2 || loadCounts["second"] != 1 {
		t.Fatalf("loader calls for collision = %v, want first=2 second=1", loadCounts)
	}

	comparisonErr := errors.New("canonical reload failed")
	duplicateItems := []Item{{ID: "canonical", Width: 2, Height: 2}, {ID: "alias", Width: 2, Height: 2}}
	duplicateImage := solidRGBA(2, 2, color.RGBA{G: 255, A: 255})
	calls := make(map[string]int)
	consumerCalls := 0
	options := Options{MinSize: 4, MaxSize: 4, Deduplicate: true}
	err = BuildPages(duplicateItems, options, func(id string) (image.Image, error) {
		calls[id]++
		if id == "canonical" && calls[id] == 2 {
			return nil, comparisonErr
		}
		return duplicateImage, nil
	}, func(int, *Atlas) error {
		consumerCalls++
		return nil
	})
	if !errors.Is(err, comparisonErr) {
		t.Fatalf("BuildPages comparison error = %v, want reload error", err)
	}
	if calls["canonical"] != 2 || calls["alias"] != 1 || consumerCalls != 0 {
		t.Fatalf(
			"loader calls=%v consumer calls=%d, want canonical=2 alias=1 consumer=0",
			calls, consumerCalls,
		)
	}
}

// solidRGBA creates a filled image used by comparison and paging tests.
func solidRGBA(width, height int, pixel color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, pixel)
		}
	}

	return img
}

// pagePackingTestOptions builds a fixed configuration for 64 sprites per page.
func pagePackingTestOptions() Options {
	opts := DefaultOptions()
	opts.MinSize = 256
	opts.MaxSize = 256
	opts.ForceSquare = true
	opts.Heuristic = HeuristicFirstFit

	return opts
}
