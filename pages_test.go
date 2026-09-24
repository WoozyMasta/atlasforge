// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"reflect"
	"strconv"
	"testing"
)

func TestPlanPagesEmptyReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	layouts, err := PlanPages(nil, Options{MinSize: 16, MaxSize: 64})
	if err != nil {
		t.Fatalf("PlanPages: %v", err)
	}
	if layouts == nil || len(layouts) != 0 {
		t.Fatalf("layouts = %#v, want non-nil empty slice", layouts)
	}

	atlases, err := PackPages(nil, Options{MinSize: 16, MaxSize: 64})
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if atlases == nil || len(atlases) != 0 {
		t.Fatalf("atlases = %#v, want non-nil empty slice", atlases)
	}
}

func TestPlanPagesSinglePageMatchesPlan(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 10, Height: 12},
		{ID: "b", Width: 8, Height: 8},
		{ID: "c", Width: 5, Height: 14},
		{ID: "d", Width: 11, Height: 7},
	}
	for _, heuristic := range []Heuristic{
		HeuristicBestShortSideFit,
		HeuristicBestLongSideFit,
		HeuristicBestAreaFit,
		HeuristicBottomLeft,
		HeuristicContactPoint,
		HeuristicFirstFit,
	} {
		t.Run(heuristicName(heuristic), func(t *testing.T) {
			t.Parallel()

			opts := Options{
				MinSize: 32, MaxSize: 64, Padding: 1, Heuristic: heuristic,
			}
			want, err := Plan(items, opts)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			got, err := PlanPages(items, opts)
			if err != nil {
				t.Fatalf("PlanPages: %v", err)
			}
			if len(got) != 1 || !reflect.DeepEqual(got[0], *want) {
				t.Fatalf("PlanPages = %+v, want one layout matching Plan: %+v", got, want)
			}
		})
	}
}

func TestPlanPagesSupportsRotationAndNonPowerOfTwoMaxSize(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "wide", Width: 88, Height: 50},
		{ID: "small", Width: 20, Height: 24},
	}
	opts := Options{
		MinSize: 64, MaxSize: 100, AllowRotate: true,
		Heuristic: HeuristicBestShortSideFit,
	}

	want, err := Plan(items, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	got, err := PlanPages(items, opts)
	if err != nil {
		t.Fatalf("PlanPages: %v", err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0], *want) {
		t.Fatalf("PlanPages = %+v, want one layout matching Plan: %+v", got, want)
	}
	if got[0].Width > opts.MaxSize || got[0].Height > opts.MaxSize {
		t.Fatalf("page size = %dx%d, exceeds MaxSize=%d", got[0].Width, got[0].Height, opts.MaxSize)
	}
}

func TestPlanPagesPlacesRotatedItemInRemainingFreeArea(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "tall", Width: 60, Height: 100},
		{ID: "wide", Width: 80, Height: 40},
	}
	opts := Options{MinSize: 128, MaxSize: 128, AllowRotate: true, ForceSquare: true}

	pages, err := PlanPages(items, opts)
	if err != nil {
		t.Fatalf("PlanPages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	var wideRotated bool
	for _, placement := range pages[0].Placements {
		if placement.ID == "wide" {
			wideRotated = placement.Rotated
		}
	}
	if !wideRotated {
		t.Fatal("wide item was not rotated into the remaining free area")
	}
}

func TestPlanPagesContinuesAfterUnplacedItemAndCompactsLastPage(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "large", Width: 80, Height: 80},
		{ID: "skipped", Width: 60, Height: 60},
		{ID: "small", Width: 40, Height: 40},
	}
	opts := Options{
		MinSize: 64, MaxSize: 128, Padding: 2, Extrude: 2,
		Heuristic: HeuristicBestShortSideFit, ForceSquare: true,
	}

	pages, err := PlanPages(items, opts)
	if err != nil {
		t.Fatalf("PlanPages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if pages[0].Width != 128 || pages[0].Height != 128 {
		t.Fatalf("first page size = %dx%d, want 128x128", pages[0].Width, pages[0].Height)
	}
	if pages[1].Width != 64 || pages[1].Height != 64 {
		t.Fatalf("last page size = %dx%d, want compact 64x64", pages[1].Width, pages[1].Height)
	}

	repeated, err := PlanPages(items, opts)
	if err != nil {
		t.Fatalf("repeat PlanPages: %v", err)
	}
	if !reflect.DeepEqual(pages, repeated) {
		t.Fatal("repeated PlanPages call returned different layouts")
	}
	if pages[0].Render == nil || *pages[0].Render != (RenderSettings{Padding: 2, Extrude: 2}) {
		t.Fatalf("render settings = %+v, want padding=2/extrude=2", pages[0].Render)
	}

	seen := make(map[string]int, len(items))
	placementCount := 0
	for pageIndex, page := range pages {
		assertPagePlacementsWithinBounds(t, page, opts.Padding)
		for _, placement := range page.Placements {
			placementCount++
			seen[placement.ID] = pageIndex
		}
	}

	wantPageByID := map[string]int{"large": 0, "small": 0, "skipped": 1}
	if placementCount != len(items) || !reflect.DeepEqual(seen, wantPageByID) {
		t.Fatalf("page assignment = %v, want %v", seen, wantPageByID)
	}
}

func TestPlanPagesAllowsAggregateAreaAcrossMultiplePages(t *testing.T) {
	t.Parallel()

	items := squarePageItems(65, 32)
	opts := Options{MinSize: 256, MaxSize: 256, ForceSquare: true}

	pages, err := PlanPages(items, opts)
	if err != nil {
		t.Fatalf("PlanPages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Placements) != 64 || len(pages[1].Placements) != 1 {
		t.Fatalf("placement counts = %d and %d, want 64 and 1", len(pages[0].Placements), len(pages[1].Placements))
	}
}

func TestPlanPagesSupportsOneTwoEightAndTwelvePages(t *testing.T) {
	t.Parallel()

	for _, pageCount := range []int{1, 2, 8, 12} {
		t.Run(strconv.Itoa(pageCount)+"-pages", func(t *testing.T) {
			t.Parallel()

			items := squarePageItems(pageCount*64, 32)
			pages, err := PlanPages(items, Options{
				MinSize: 256, MaxSize: 256, ForceSquare: true,
			})
			if err != nil {
				t.Fatalf("PlanPages: %v", err)
			}
			if len(pages) != pageCount {
				t.Fatalf("page count = %d, want %d", len(pages), pageCount)
			}
			for _, page := range pages {
				if len(page.Placements) != 64 {
					t.Fatalf("page has %d placements, want 64", len(page.Placements))
				}
			}
		})
	}
}

func TestPlanPagesRejectsOversizedItemsAndDuplicateIDs(t *testing.T) {
	t.Parallel()

	opts := Options{MinSize: 16, MaxSize: 64}
	t.Run("oversized", func(t *testing.T) {
		t.Parallel()

		_, err := PlanPages([]Item{{ID: "too-large", Width: 65, Height: 1}}, opts)
		if !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("error = %v, want ErrInvalidOptions", err)
		}
	})

	t.Run("duplicate IDs", func(t *testing.T) {
		t.Parallel()

		_, err := PlanPages([]Item{
			{ID: "duplicate", Width: 8, Height: 8},
			{ID: "duplicate", Width: 4, Height: 4},
		}, opts)
		if !errors.Is(err, ErrInvalidItem) {
			t.Fatalf("error = %v, want ErrInvalidItem", err)
		}
	})

	t.Run("invalid options", func(t *testing.T) {
		t.Parallel()

		_, err := PlanPages(nil, Options{MinSize: 0, MaxSize: 64})
		if !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("error = %v, want ErrInvalidOptions", err)
		}
	})
}

func TestPackPagesRendersEachPlannedPage(t *testing.T) {
	t.Parallel()

	sprites := []Sprite{
		solidPageSprite("large", 80, color.RGBA{R: 220, A: 255}),
		solidPageSprite("skipped", 60, color.RGBA{G: 210, A: 255}),
		solidPageSprite("small", 40, color.RGBA{B: 200, A: 255}),
	}
	opts := Options{
		MinSize: 64, MaxSize: 128, Padding: 2, Extrude: 2,
		Heuristic: HeuristicBestShortSideFit, ForceSquare: true,
	}

	atlases, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(atlases) != 2 {
		t.Fatalf("atlas count = %d, want 2", len(atlases))
	}

	wantColors := map[string]color.RGBA{
		"large":   {R: 220, A: 255},
		"skipped": {G: 210, A: 255},
		"small":   {B: 200, A: 255},
	}
	for _, atlas := range atlases {
		bounds := atlas.Image.Bounds()
		if bounds.Dx() != atlas.Layout.Width || bounds.Dy() != atlas.Layout.Height {
			t.Fatalf("image bounds = %v, layout = %dx%d", bounds, atlas.Layout.Width, atlas.Layout.Height)
		}
		for _, placement := range atlas.Layout.Placements {
			got := atlas.Image.At(placement.X, placement.Y)
			if !sameRGBA(got, wantColors[placement.ID]) {
				t.Fatalf("pixel for %q = %v, want %v", placement.ID, got, wantColors[placement.ID])
			}
		}
	}
}

func TestPackPagesSingleAtlasMatchesPack(t *testing.T) {
	t.Parallel()

	sprites := []Sprite{
		solidPageSprite("first", 12, color.RGBA{R: 170, A: 255}),
		solidPageSprite("second", 8, color.RGBA{G: 180, A: 255}),
	}
	opts := Options{MinSize: 16, MaxSize: 64, Padding: 1, Extrude: 1}
	want, err := Pack(sprites, opts)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	got, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Layout, want.Layout) {
		t.Fatalf("PackPages layout = %+v, want Pack layout %+v", got, want.Layout)
	}
	if !imagesEqual(got[0].Image, want.Image) {
		t.Fatal("PackPages image differs from Pack image")
	}
}

// assertPagePlacementsWithinBounds checks reserved rectangles and page bounds.
func assertPagePlacementsWithinBounds(t *testing.T, page Layout, padding int) {
	t.Helper()

	reserved := make([]image.Rectangle, len(page.Placements))
	for i, placement := range page.Placements {
		width, height := placement.Width, placement.Height
		if placement.Rotated {
			width, height = height, width
		}
		x, y := placement.X-padding, placement.Y-padding
		if x < 0 || y < 0 || x+width+2*padding > page.Width || y+height+2*padding > page.Height {
			t.Errorf(
				"placement %q exceeds page bounds: %+v in %dx%d",
				placement.ID, placement, page.Width, page.Height,
			)
		}

		reserved[i] = image.Rect(x, y, x+width+2*padding, y+height+2*padding)
	}

	for i := range reserved {
		for j := i + 1; j < len(reserved); j++ {
			if reserved[i].Overlaps(reserved[j]) {
				t.Errorf(
					"placements %q and %q overlap including padding",
					page.Placements[i].ID, page.Placements[j].ID,
				)
			}
		}
	}
}

// squarePageItems creates deterministic square items for page-count tests.
func squarePageItems(count, size int) []Item {
	items := make([]Item, count)
	for i := range items {
		items[i] = Item{ID: fmt.Sprintf("item-%03d", i), Width: size, Height: size}
	}

	return items
}

// solidPageSprite creates a uniformly colored square sprite.
func solidPageSprite(id string, size int, c color.RGBA) Sprite {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)

	return Sprite{ID: id, Image: img}
}

// sameRGBA reports whether img has the expected 8-bit RGBA value.
func sameRGBA(img color.Color, want color.RGBA) bool {
	r, g, b, a := img.RGBA()
	return r == uint32(want.R)*257 && g == uint32(want.G)*257 && b == uint32(want.B)*257 && a == uint32(want.A)*257
}

// imagesEqual compares image bounds and premultiplied pixel values.
func imagesEqual(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}

	return true
}

func ExamplePlanPages() {
	opts := DefaultOptions()
	opts.MinSize = 32
	opts.MaxSize = 64
	items := []Item{
		{ID: "background", Width: 48, Height: 48},
		{ID: "foreground", Width: 48, Height: 48},
	}

	pages, err := PlanPages(items, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(pages))
	// Output: 2
}
