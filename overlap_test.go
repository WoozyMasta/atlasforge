// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestOverlapGridShapes(t *testing.T) {
	expected := map[int]image.Point{
		2:  {X: 58, Y: 32},
		3:  {X: 84, Y: 32},
		4:  {X: 58, Y: 58},
		5:  {X: 84, Y: 58},
		6:  {X: 84, Y: 58},
		9:  {X: 84, Y: 84},
		16: {X: 110, Y: 110},
	}

	for count, size := range expected {
		t.Run("count="+itoa(count), func(t *testing.T) {
			profiles := make([]overlapProfile, count)
			for i := range profiles {
				id := string(rune('a' + i))
				image := makeOverlapImage(32, 8, color.RGBA{
					R: 9,
					G: 17,
					B: 33,
					A: 255,
				}, uint8(i+1))
				profile, eligible, err := scanOverlapProfile(image, Item{
					ID:     id,
					Width:  32,
					Height: 32,
				}, trimMetadata{}, Options{})
				if err != nil || !eligible {
					t.Fatalf("scan profile eligible=%t err=%v", eligible, err)
				}

				profiles[i] = profile
			}

			zone, ok := makeOverlapZone(profiles, profiles[0].color, Options{Padding: 2, MaxSize: 256})
			if !ok {
				t.Fatal("makeOverlapZone rejected compatible icons")
			}
			if got := image.Pt(zone.width, zone.height); got != size {
				t.Fatalf("zone size = %v, want %v", got, size)
			}
			if len(zone.members) != count {
				t.Fatalf("zone members = %d, want %d", len(zone.members), count)
			}
		})
	}
}

func TestOverlapGuillotineBorderAndIcons(t *testing.T) {
	edgeColor := color.RGBA{R: 15, G: 35, B: 75, A: 255}
	sprites := make([]Sprite, 0, 5)
	sprites = append(sprites, Sprite{ID: "border", Image: makeOverlapImage(128, 8, edgeColor, 1)})
	for index := range 4 {
		sprites = append(sprites, Sprite{
			ID:    string(rune('a' + index)),
			Image: makeOverlapImage(32, 8, edgeColor, uint8(index+2)),
		})
	}

	profiles := scanProfilesForTest(t, sprites, Options{})
	zone, ok := makeOverlapZone(profiles, profiles[0].color, Options{Padding: 2, MaxSize: 256})
	if !ok {
		t.Fatal("Guillotine solver rejected mixed-size bucket")
	}
	if got := image.Pt(zone.width, zone.height); got != (image.Pt(58, 128)) {
		t.Fatalf("zone size = %v, want 58x128", got)
	}

	positions := make(map[string]image.Point, len(zone.members))
	for _, member := range zone.members {
		positions[member.item.ID] = image.Pt(member.x, member.y)
	}
	want := map[string]image.Point{
		"border": image.Pt(0, 0),
		"a":      image.Pt(26, 0),
		"b":      image.Pt(26, 26),
		"c":      image.Pt(26, 52),
		"d":      image.Pt(26, 78),
	}
	for id, position := range want {
		if positions[id] != position {
			t.Fatalf("member %q position = %v, want %v", id, positions[id], position)
		}
	}
}

func TestOverlapSharesRespectPaddingGuard(t *testing.T) {
	for _, test := range []struct {
		name      string
		depth     int
		padding   int
		wantShare int
	}{
		{name: "zero padding", depth: 8, padding: 0, wantShare: 8},
		{name: "residual guard", depth: 6, padding: 4, wantShare: 2},
		{name: "depth equals padding", depth: 4, padding: 4},
		{name: "depth below padding", depth: 2, padding: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			edgeColor := color.RGBA{R: 7, G: 17, B: 27, A: 255}
			profiles := scanProfilesForTest(t, []Sprite{
				{ID: "first", Image: makeOverlapImage(32, test.depth, edgeColor, 1)},
				{ID: "second", Image: makeOverlapImage(32, test.depth, edgeColor, 2)},
			}, Options{})
			zone, ok := makeOverlapZone(profiles, profiles[0].color, Options{
				Padding: test.padding,
				MaxSize: 128,
			})
			if test.wantShare == 0 {
				if ok {
					t.Fatalf("unexpected zone with shares %d,%d", zone.shareX, zone.shareY)
				}
				return
			}

			if !ok || zone.shareX != test.wantShare || zone.shareY != test.wantShare {
				t.Fatalf("zone=%+v accepted=%t, want share %d", zone, ok, test.wantShare)
			}
		})
	}
}

func TestOverlapPackFormsNMemberZonesAndRendersExactly(t *testing.T) {
	for _, count := range []int{2, 3, 4, 5, 6, 9, 16, 64} {
		t.Run("count="+itoa(count), func(t *testing.T) {
			sprites := make([]Sprite, count)
			for i := range sprites {
				sprites[i] = Sprite{
					ID:    "icon-" + itoa(i),
					Image: makeOverlapImage(32, 8, color.RGBA{R: 41, G: 79, B: 113, A: 255}, uint8(i+1)),
				}
			}
			options := overlapTestOptions()
			atlas, err := Pack(sprites, options)
			if err != nil {
				t.Fatalf("Pack: %v", err)
			}
			if len(atlas.Layout.OverlapBlocks) != 1 {
				t.Fatalf("overlap blocks = %d, want 1", len(atlas.Layout.OverlapBlocks))
			}

			block := atlas.Layout.OverlapBlocks[0]
			if got := countOverlapMembers(atlas.Layout.Placements, block.ID); got != count {
				t.Fatalf("zone members = %d, want %d", got, count)
			}
			if count == 4 && (block.Width != 58 || block.Height != 58) {
				t.Fatalf("four-icon zone = %dx%d, want 58x58", block.Width, block.Height)
			}

			assertSpritePixels(t, atlas.Image, atlas.Layout.Placements, sprites)

			encoded, err := json.Marshal(atlas.Layout)
			if err != nil {
				t.Fatalf("marshal layout: %v", err)
			}

			var layout Layout
			if err := json.Unmarshal(encoded, &layout); err != nil {
				t.Fatalf("unmarshal layout: %v", err)
			}

			inputs := make([]Source, len(sprites))
			for i := range sprites {
				inputs[i] = Source{ID: sprites[i].ID, Image: sprites[i].Image}
			}

			rendered, err := Render(&layout, inputs)
			if err != nil {
				t.Fatalf("Render after JSON round trip: %v", err)
			}
			if !sameRGBAImage(atlas.Image, rendered) {
				t.Fatal("Pack and Render pixels differ")
			}
		})
	}
}

func TestOverlapSupportsPagesLazyLoaderPaddingAndExtrusion(t *testing.T) {
	sprites := make([]Sprite, 16)
	items := make([]Item, len(sprites))
	sources := make(map[string]image.Image, len(sprites))
	for i := range sprites {
		id := "page-" + itoa(i)
		image := makeOverlapImage(32, 8, color.RGBA{R: 11, G: 51, B: 101, A: 255}, uint8(i+1))
		sprites[i] = Sprite{ID: id, Image: image}
		items[i] = Item{ID: id, Width: 32, Height: 32}
		sources[id] = image
	}
	options := overlapTestOptions()
	options.Padding = 4
	options.Extrude = 4
	options.MinSize = 32
	options.MaxSize = 128

	packed, err := Pack(sprites, options)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	loader := func(id string) (image.Image, error) {
		return sources[id], nil
	}

	var built []*Atlas
	if err := BuildPages(items, options, loader, func(_ int, atlas *Atlas) error {
		built = append(built, atlas)
		return nil
	}); err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if len(built) != 1 || !sameRGBAImage(packed.Image, built[0].Image) ||
		!samePlacements(packed.Layout.Placements, built[0].Layout.Placements) {
		t.Fatalf("Pack and BuildPages differ: pages=%d", len(built))
	}

	pages, err := PackPages(sprites, options)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(pages) != 1 || !sameRGBAImage(packed.Image, pages[0].Image) {
		t.Fatalf("Pack and PackPages differ: pages=%d", len(pages))
	}

	loaded, err := RenderWithLoader(&packed.Layout, loader)
	if err != nil || !sameRGBAImage(packed.Image, loaded) {
		t.Fatalf("RenderWithLoader parity err=%v", err)
	}
}

func TestOverlapRejectsChangedSharedPixelsAndMalformedBlocks(t *testing.T) {
	sprites := []Sprite{
		{ID: "first", Image: makeOverlapImage(32, 8, color.RGBA{R: 21, G: 31, B: 41, A: 255}, 1)},
		{ID: "second", Image: makeOverlapImage(32, 8, color.RGBA{R: 21, G: 31, B: 41, A: 255}, 2)},
	}
	atlas, err := Pack(sprites, overlapTestOptions())
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placements := placementMap(atlas.Layout.Placements)
	first, second := placements["first"], placements["second"]
	intersection := placementContentBounds(first).Intersect(placementContentBounds(second))
	if intersection.Empty() {
		t.Fatal("fixture has no shared pixel intersection")
	}

	changed := copyRGBA(sprites[1].Image.(*image.RGBA))
	changed.SetRGBA(intersection.Min.X-second.X, intersection.Min.Y-second.Y, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	if _, err := Render(&atlas.Layout, []Source{
		{ID: "first", Image: sprites[0].Image},
		{ID: "second", Image: changed},
	}); !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("changed shared pixel error = %v, want ErrInvalidLayout", err)
	}

	tests := []struct {
		name   string
		mutate func(*Layout)
	}{
		{name: "missing block", mutate: func(layout *Layout) { layout.OverlapBlocks = nil }},
		{name: "wrong bounds", mutate: func(layout *Layout) { layout.OverlapBlocks[0].Width++ }},
		{name: "invalid share", mutate: func(layout *Layout) { layout.OverlapBlocks[0].ShareX = 32 }},
		{name: "bad group reference", mutate: func(layout *Layout) { layout.Placements[0].OverlapGroup++ }},
		{name: "work rectangles intersect", mutate: func(layout *Layout) {
			layout.Placements[1].X = layout.Placements[0].X
			layout.Placements[1].Y = layout.Placements[0].Y
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := cloneLayout(t, atlas.Layout)
			test.mutate(&layout)
			if _, err := Render(&layout, []Source{
				{ID: "first", Image: sprites[0].Image},
				{ID: "second", Image: sprites[1].Image},
			}); !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("Render error = %v, want ErrInvalidLayout", err)
			}
		})
	}
}

func TestOverlapDedupAliasesAndStatsUseUnionArea(t *testing.T) {
	edgeColor := color.RGBA{R: 14, G: 54, B: 94, A: 255}
	first := makeOverlapImage(32, 8, edgeColor, 1)
	second := makeOverlapImage(32, 8, edgeColor, 2)
	duplicate := copyRGBA(first)
	options := overlapTestOptions()
	options.Deduplicate = true
	atlas, err := Pack([]Sprite{
		{ID: "a", Image: first},
		{ID: "b", Image: second},
		{ID: "alias", Image: duplicate},
	}, options)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	if len(atlas.Layout.OverlapBlocks) != 1 ||
		countOverlapMembers(atlas.Layout.Placements, atlas.Layout.OverlapBlocks[0].ID) != 2 {
		t.Fatalf("blocks=%+v placements=%+v", atlas.Layout.OverlapBlocks, atlas.Layout.Placements)
	}

	stats, err := AnalyzeLayout(&atlas.Layout, options.Padding)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	if stats.LogicalCount != 3 ||
		stats.PhysicalCount != 2 ||
		stats.AliasCount != 1 ||
		stats.OverlapGroupCount != 1 ||
		stats.OverlapGroupMembers != 2 ||
		stats.OverlapBlockArea != int64(atlas.Layout.OverlapBlocks[0].Width*atlas.Layout.OverlapBlocks[0].Height) {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	memberArea := int64(32 * 32 * 2)
	if stats.OverlapSharedPixels <= 0 || stats.ContentArea != memberArea-stats.OverlapSharedPixels {
		t.Fatalf("union stats inconsistent: %+v", stats)
	}
}

func TestOverlapColorKeyTrimAndUnmatchedFallback(t *testing.T) {
	key := color.RGBA{R: 255, G: 0, B: 255, A: 255}
	images := []*image.RGBA{
		makeOverlapImage(32, 8, key, 1),
		makeOverlapImage(32, 8, key, 2),
	}
	sprites := []Sprite{{ID: "a", Image: images[0]}, {ID: "b", Image: images[1]}}
	options := overlapTestOptions()
	options.ColorKey = &ColorKeyOptions{Color: color.NRGBA(key), ToAlpha: true}

	atlas, err := Pack(sprites, options)
	if err != nil {
		t.Fatalf("Pack ColorKey: %v", err)
	}
	if len(atlas.Layout.OverlapBlocks) != 1 {
		t.Fatalf("ColorKey blocks=%d, want 1", len(atlas.Layout.OverlapBlocks))
	}

	options.ColorKey = nil
	options.TrimTransparent = true
	for _, sprite := range sprites {
		img := sprite.Image.(*image.RGBA)
		for y := range 32 {
			for x := range 32 {
				if x < 8 || x >= 24 || y < 8 || y >= 24 {
					img.SetRGBA(x, y, color.RGBA{})
				}
			}
		}
	}

	trimmed, err := Pack(sprites, options)
	if err != nil {
		t.Fatalf("Pack trim: %v", err)
	}
	if len(trimmed.Layout.OverlapBlocks) != 0 {
		t.Fatalf("trimmed-away edges produced %d zones", len(trimmed.Layout.OverlapBlocks))
	}

	options.TrimTransparent = false
	plain := []Sprite{
		{ID: "red", Image: makeOverlapImage(32, 8, color.RGBA{R: 1, G: 2, B: 3, A: 255}, 1)},
		{ID: "blue", Image: makeOverlapImage(32, 8, color.RGBA{R: 3, G: 2, B: 1, A: 255}, 2)},
	}
	baseline, err := Pack(plain, options)
	if err != nil {
		t.Fatalf("Pack unmatched: %v", err)
	}
	if len(baseline.Layout.OverlapBlocks) != 0 {
		t.Fatalf("different colors created %d zones", len(baseline.Layout.OverlapBlocks))
	}
}

func TestOverlapPlannerFallbackAndDeterminism(t *testing.T) {
	sprites := []Sprite{
		{ID: "a", Image: makeOverlapImage(32, 2, color.RGBA{R: 71, G: 82, B: 93, A: 255}, 1)},
		{ID: "b", Image: makeOverlapImage(32, 2, color.RGBA{R: 71, G: 82, B: 93, A: 255}, 2)},
	}
	options := overlapTestOptions()
	options.MinSize = 128
	options.MaxSize = 128
	options.Padding = 0
	baselineOptions := options
	baselineOptions.Overlap = false

	baseline, err := Pack(sprites, baselineOptions)
	if err != nil {
		t.Fatalf("baseline Pack: %v", err)
	}

	withOverlap, err := Pack(sprites, options)
	if err != nil {
		t.Fatalf("overlap Pack: %v", err)
	}

	if withOverlap.Layout.Width != baseline.Layout.Width ||
		withOverlap.Layout.Height != baseline.Layout.Height {
		t.Fatalf("overlap worsened dimensions: baseline=%dx%d overlap=%dx%d",
			baseline.Layout.Width, baseline.Layout.Height, withOverlap.Layout.Width, withOverlap.Layout.Height)
	}

	firstBytes, _ := json.Marshal(withOverlap.Layout)
	for range 5 {
		repeated, packErr := Pack(sprites, options)
		if packErr != nil {
			t.Fatalf("repeat Pack: %v", packErr)
		}

		repeatedBytes, _ := json.Marshal(repeated.Layout)
		if string(firstBytes) != string(repeatedBytes) {
			t.Fatal("layout changed across deterministic runs")
		}
	}
}

func TestOverlapRotationTransformsWholeBlock(t *testing.T) {
	zone := overlapZone{
		id:     1,
		color:  packRGBA8(color.RGBA{R: 22, G: 33, B: 44, A: 255}),
		shareX: 6,
		shareY: 6,
		width:  58,
		height: 58,
		members: []overlapMember{
			{item: Item{ID: "a", Width: 32, Height: 32}, x: 0, y: 0},
			{item: Item{ID: "b", Width: 32, Height: 32}, x: 26, y: 0},
			{item: Item{ID: "c", Width: 32, Height: 32}, x: 0, y: 26},
			{item: Item{ID: "d", Width: 32, Height: 32}, x: 26, y: 26},
		},
	}
	grouping := overlapGrouping{
		zonesByID: map[string]overlapZone{"virtual": zone},
		memberZone: map[string]string{
			"a": "virtual", "b": "virtual", "c": "virtual", "d": "virtual",
		},
	}
	layout := Layout{
		Width:  128,
		Height: 128,
		Placements: []Placement{{
			ID:      "virtual",
			X:       10,
			Y:       20,
			Width:   58,
			Height:  58,
			Rotated: true,
		}},
	}
	expandOverlapLayout(&layout, grouping, nil, Options{Padding: 2})

	if len(layout.OverlapBlocks) != 1 ||
		!layout.OverlapBlocks[0].Rotated ||
		layout.OverlapBlocks[0].Width != 58 ||
		layout.OverlapBlocks[0].Height != 58 {
		t.Fatalf("rotated block = %+v", layout.OverlapBlocks)
	}

	want := map[string]image.Point{
		"a": {X: 36, Y: 20}, "b": {X: 36, Y: 46},
		"c": {X: 10, Y: 20}, "d": {X: 10, Y: 46},
	}
	for _, placement := range layout.Placements {
		if point := image.Pt(placement.X, placement.Y); point != want[placement.ID] || !placement.Rotated {
			t.Fatalf("rotated placement %q = %v, want %v", placement.ID, point, want[placement.ID])
		}
	}
	if err := validateLayout(&layout); err != nil {
		t.Fatalf("validate rotated block: %v", err)
	}
}

func TestOverlapRotationValidatesMixedMemberWorkRectangles(t *testing.T) {
	zone := overlapZone{
		id:     1,
		shareX: 6,
		shareY: 6,
		width:  58,
		height: 128,
		members: []overlapMember{
			{item: Item{ID: "border", Width: 32, Height: 128}, x: 0, y: 0},
			{item: Item{ID: "a", Width: 32, Height: 32}, x: 26, y: 0},
			{item: Item{ID: "b", Width: 32, Height: 32}, x: 26, y: 26},
			{item: Item{ID: "c", Width: 32, Height: 32}, x: 26, y: 52},
			{item: Item{ID: "d", Width: 32, Height: 32}, x: 26, y: 78},
		},
	}
	grouping := overlapGrouping{
		zonesByID: map[string]overlapZone{"virtual": zone},
		memberZone: map[string]string{
			"border": "virtual",
			"a":      "virtual",
			"b":      "virtual",
			"c":      "virtual",
			"d":      "virtual",
		},
	}
	layout := Layout{Width: 140, Height: 80, Placements: []Placement{{
		ID:      "virtual",
		X:       5,
		Y:       5,
		Width:   58,
		Height:  128,
		Rotated: true,
	}}}

	expandOverlapLayout(&layout, grouping, nil, Options{Padding: 2})
	if got := layout.OverlapBlocks[0]; got.Width != 128 || got.Height != 58 || !got.Rotated {
		t.Fatalf("rotated mixed block = %+v", got)
	}
	if err := validateLayout(&layout); err != nil {
		t.Fatalf("validate rotated mixed block: %v", err)
	}
}

func TestOverlapStatsSweepCountsTripleAndFourWayUnion(t *testing.T) {
	profiles := make([]overlapProfile, 4)
	for i := range profiles {
		image := makeOverlapImage(32, 8, color.RGBA{R: 10, G: 20, B: 30, A: 255}, uint8(i+1))
		profile, eligible, err := scanOverlapProfile(image, Item{
			ID:     "s" + itoa(i),
			Width:  32,
			Height: 32,
		}, trimMetadata{}, Options{})
		if err != nil || !eligible {
			t.Fatalf("profile eligible=%t err=%v", eligible, err)
		}
		profiles[i] = profile
	}
	zone, ok := makeOverlapZone(profiles, profiles[0].color, Options{Padding: 2, MaxSize: 128})
	if !ok {
		t.Fatal("zone creation failed")
	}

	zone.id = 1
	grouping := overlapGrouping{
		zonesByID: map[string]overlapZone{"virtual": zone},
		memberZone: map[string]string{
			"s0": "virtual",
			"s1": "virtual",
			"s2": "virtual",
			"s3": "virtual",
		},
	}
	layout := Layout{
		Width:  128,
		Height: 128,
		Placements: []Placement{{
			ID:     "virtual",
			X:      2,
			Y:      2,
			Width:  zone.width,
			Height: zone.height,
		}},
	}
	expandOverlapLayout(&layout, grouping, nil, Options{Padding: 2})
	stats, err := AnalyzeLayout(&layout, 2)
	if err != nil {
		t.Fatalf("AnalyzeLayout: %v", err)
	}

	if stats.ContentArea != 58*58 ||
		stats.OverlapSharedPixels != 4*32*32-58*58 ||
		stats.OverlapGroupMembers != 4 ||
		stats.OverlapBlockArea != 58*58 {
		t.Fatalf("union stats = %+v", stats)
	}
}

func TestOverlapNonzeroBoundsAndAlphaAreExact(t *testing.T) {
	base := makeOverlapImage(32, 8, color.RGBA{R: 90, G: 60, B: 30, A: 128}, 1)
	padded := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 32 {
		for x := range 32 {
			padded.SetRGBA(x+5, y+7, base.RGBAAt(x, y))
		}
	}

	view := padded.SubImage(image.Rect(5, 7, 37, 39))
	profile, eligible, err := scanOverlapProfile(view, Item{
		ID:     "subimage",
		Width:  32,
		Height: 32,
	}, trimMetadata{}, Options{})
	if err != nil || !eligible || profile.depths[overlapLeft] != 8 {
		t.Fatalf("strided subimage profile=%+v eligible=%t err=%v", profile, eligible, err)
	}

	nrgba := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			nrgba.SetNRGBA(x, y, color.NRGBA{R: 100, G: 60, B: 20, A: 128})
		}
	}
	profile, eligible, err = scanOverlapProfile(nrgba, Item{
		ID:     "nrgba",
		Width:  32,
		Height: 32,
	}, trimMetadata{}, Options{})
	if err != nil || !eligible || profile.depths != [4]int{32, 32, 32, 32} {
		t.Fatalf("NRGBA partial-alpha profile=%+v eligible=%t err=%v", profile, eligible, err)
	}

	sub := base.SubImage(image.Rect(0, 0, 32, 32))
	imageView := &offsetImage{
		image:  sub,
		bounds: image.Rect(7, -3, 39, 29),
	}
	profile, eligible, err = scanOverlapProfile(imageView, Item{
		ID:     "offset",
		Width:  32,
		Height: 32,
	}, trimMetadata{}, Options{})
	if err != nil ||
		!eligible ||
		profile.depths[overlapLeft] != 8 ||
		profile.color != packRGBA8(color.RGBA{R: 90, G: 60, B: 30, A: 128}) {
		t.Fatalf("offset RGBA profile=%+v eligible=%t err=%v", profile, eligible, err)
	}

	base.SetRGBA(0, 10, color.RGBA{R: 90, G: 60, B: 30, A: 127})
	if _, eligible, err := scanOverlapProfile(base, Item{
		ID:     "alpha",
		Width:  32,
		Height: 32,
	}, trimMetadata{}, Options{}); err != nil || eligible {
		t.Fatalf("different alpha profile eligible=%t err=%v", eligible, err)
	}
}

func overlapTestOptions() Options {
	options := DefaultOptions()
	options.MinSize = 32
	options.MaxSize = 512
	options.Padding = 2
	options.Overlap = true
	options.AllowRotate = false
	options.ForceSquare = true
	return options
}

// makeOverlapImage creates an opaque solid RGBA8 border around distinct interiors.
func makeOverlapImage(size, border int, edgeColor color.RGBA, seed uint8) *image.RGBA {
	width, height := 32, size
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			pixel := edgeColor
			if x >= border && x < width-border && y >= border && y < height-border {
				pixel = color.RGBA{R: seed, G: seed*3 + 1, B: seed*7 + 2, A: 255}
				if pixel == edgeColor {
					pixel.R++
				}
			}

			img.SetRGBA(x, y, pixel)
		}
	}

	return img
}

// scanProfilesForTest scans a sprite set into eligible overlap profiles.
func scanProfilesForTest(t *testing.T, sprites []Sprite, options Options) []overlapProfile {
	t.Helper()
	profiles := make([]overlapProfile, len(sprites))
	for i, sprite := range sprites {
		item := Item{
			ID:     sprite.ID,
			Width:  sprite.Image.Bounds().Dx(),
			Height: sprite.Image.Bounds().Dy(),
		}
		profile, eligible, err := scanOverlapProfile(sprite.Image, item, trimMetadata{}, options)
		if err != nil || !eligible {
			t.Fatalf("scan %q eligible=%t err=%v", sprite.ID, eligible, err)
		}

		profiles[i] = profile
	}

	return profiles
}

// assertSpritePixels verifies every canonical source pixel at its atlas transform.
func assertSpritePixels(t *testing.T, atlas image.Image, placements []Placement, sprites []Sprite) {
	t.Helper()
	sourceByID := make(map[string]image.Image, len(sprites))
	for _, sprite := range sprites {
		sourceByID[sprite.ID] = sprite.Image
	}

	for _, placement := range placements {
		if placement.AliasOf != "" {
			continue
		}

		source := sourceByID[placement.ID]
		bounds := source.Bounds()
		for sy := range placement.Height {
			for sx := range placement.Width {
				x, y := placement.X+sx, placement.Y+sy
				if placement.Rotated {
					x, y = placement.X+placement.Height-1-sy, placement.Y+sx
				}

				want := canonicalImageRGBA8(source, bounds.Min.X+sx, bounds.Min.Y+sy)
				got := canonicalImageRGBA8(atlas, x, y)
				if got != want {
					t.Fatalf("pixel mismatch ID=%q source=(%d,%d) atlas=(%d,%d) got=%v want=%v",
						placement.ID, sx, sy, x, y, got, want)
				}
			}
		}
	}
}

// sameRGBAImage compares full bounds and canonical RGBA pixels.
func sameRGBAImage(first, second image.Image) bool {
	if first.Bounds() != second.Bounds() {
		return false
	}

	bounds := first.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if canonicalImageRGBA8(first, x, y) != canonicalImageRGBA8(second, x, y) {
				return false
			}
		}
	}

	return true
}

// samePlacements compares placement slices without relying on their order.
func samePlacements(first, second []Placement) bool {
	if len(first) != len(second) {
		return false
	}

	byID := make(map[string]Placement, len(first))
	for _, placement := range first {
		byID[placement.ID] = placement
	}
	for _, placement := range second {
		if byID[placement.ID] != placement {
			return false
		}
	}

	return true
}

// countOverlapMembers returns the number of canonical placements in one block.
func countOverlapMembers(placements []Placement, groupID int) int {
	count := 0
	for _, placement := range placements {
		if placement.AliasOf == "" && placement.OverlapGroup == groupID {
			count++
		}
	}

	return count
}

// placementMap indexes placements by their stable IDs.
func placementMap(placements []Placement) map[string]Placement {
	result := make(map[string]Placement, len(placements))
	for _, placement := range placements {
		result[placement.ID] = placement
	}

	return result
}

// copyRGBA copies a concrete RGBA source for source-mutation tests.
func copyRGBA(source *image.RGBA) *image.RGBA {
	cloned := image.NewRGBA(source.Rect)
	cloned.Pix = append(cloned.Pix[:0], source.Pix...)
	return cloned
}

// cloneLayout copies a layout through its public JSON contract.
func cloneLayout(t *testing.T, layout Layout) Layout {
	t.Helper()
	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal layout: %v", err)
	}

	var clone Layout
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatalf("unmarshal layout: %v", err)
	}

	return clone
}

// itoa formats small test indexes without pulling formatting into test loops.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}

	return string(buffer[index:])
}

// offsetImage changes reported bounds while preserving source pixel storage.
type offsetImage struct {
	image  image.Image     // backing image.
	bounds image.Rectangle // requested nonzero rectangle.
}

// ColorModel reports the backing image's color model.
func (img *offsetImage) ColorModel() color.Model {
	return img.image.ColorModel()
}

// Bounds reports the requested nonzero image bounds.
func (img *offsetImage) Bounds() image.Rectangle {
	return img.bounds
}

// At translates nonzero image coordinates into backing image coordinates.
func (img *offsetImage) At(x, y int) color.Color {
	return img.image.At(x-img.bounds.Min.X, y-img.bounds.Min.Y)
}
