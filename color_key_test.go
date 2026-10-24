// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestPackColorKeyTrimOnlyPreservesPixelsAndMetadata(t *testing.T) {
	key := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	source := image.NewNRGBA(image.Rect(-4, 7, 1, 12))
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			source.SetNRGBA(x, y, key)
		}
	}
	for y := 8; y < 11; y++ {
		for x := -3; x < 0; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 220, G: 35, B: 70, A: 255})
		}
	}
	source.SetNRGBA(-2, 9, key)

	atlas, err := Pack([]Sprite{{ID: "trim", Image: source}}, Options{
		MinSize: 4,
		MaxSize: 8,
		ColorKey: &ColorKeyOptions{
			Color: key,
			Trim:  true,
		},
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 3 || placement.Height != 3 ||
		placement.OriginalWidth != 5 || placement.OriginalHeight != 5 ||
		placement.OffsetX != 1 || placement.OffsetY != 1 {
		t.Fatalf("placement = %+v, want 3x3 crop at offset 1,1 from 5x5 source", placement)
	}

	if got := canonicalRGBA8(
		atlas.Image.At(placement.X+1, placement.Y+1),
	); got != canonicalColorKey(key) {
		t.Fatalf("trim-only interior key pixel = %#v, want unchanged %#v", got, canonicalColorKey(key))
	}

	if source.NRGBAAt(-2, 9) != key {
		t.Fatal("Pack modified the source image")
	}
	if atlas.Layout.Render == nil ||
		atlas.Layout.Render.ColorKey == nil ||
		!atlas.Layout.Render.ColorKey.Trim {
		t.Fatalf("ColorKey render settings were not recorded: %+v", atlas.Layout.Render)
	}
}

func TestPackColorKeyToAlphaKeepsGeometryAndCanonicalizesPixels(t *testing.T) {
	key := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	source := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			source.SetNRGBA(x, y, color.NRGBA{R: 190, G: 30, B: 70, A: 255})
		}
	}
	source.SetNRGBA(0, 0, key)
	source.SetNRGBA(2, 0, key)
	source.SetNRGBA(2, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 128})
	source.SetNRGBA(3, 1, color.NRGBA{R: 210, G: 50, B: 80, A: 0})

	atlas, err := Pack([]Sprite{{ID: "alpha", Image: source}}, Options{
		MinSize: 4,
		MaxSize: 4,
		ColorKey: &ColorKeyOptions{
			Color:   key,
			ToAlpha: true,
		},
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 4 || placement.Height != 2 || placement.OriginalWidth != 0 {
		t.Fatalf("ToAlpha changed placement geometry: %+v", placement)
	}

	for _, point := range []image.Point{{X: 0, Y: 0}, {X: 2, Y: 0}} {
		got := color.RGBAModel.
			Convert(atlas.Image.At(placement.X+point.X, placement.Y+point.Y)).(color.RGBA)
		if got != (color.RGBA{}) {
			t.Errorf("key pixel at %v = %#v, want transparent zero", point, got)
		}
	}

	partial := color.RGBAModel.
		Convert(atlas.Image.At(placement.X+2, placement.Y+1)).(color.RGBA)
	wantPartial := canonicalRGBA8(color.NRGBA{R: 255, G: 255, B: 255, A: 128})
	if partial != wantPartial {
		t.Errorf("partially transparent non-match = %#v, want %#v", partial, wantPartial)
	}

	if got := color.RGBAModel.
		Convert(atlas.Image.At(placement.X+3, placement.Y+1)).(color.RGBA); got != (color.RGBA{}) {
		t.Errorf("transparent source pixel = %#v, want canonical zero", got)
	}

	if source.NRGBAAt(0, 0) != key {
		t.Fatal("Pack modified the source image")
	}
	if atlas.Layout.Render == nil ||
		atlas.Layout.Render.ColorKey == nil ||
		!atlas.Layout.Render.ColorKey.ToAlpha {
		t.Fatalf("ToAlpha settings were not recorded without extrusion: %+v", atlas.Layout.Render)
	}
}

func TestColorKeyToAlphaCombinesWithTransparentTrimming(t *testing.T) {
	key := color.NRGBA{R: 250, G: 250, B: 250, A: 255}
	source := image.NewNRGBA(image.Rect(-2, 5, 3, 10))
	for y := 5; y < 10; y++ {
		for x := -2; x < 3; x++ {
			source.SetNRGBA(x, y, key)
		}
	}
	for y := 6; y < 9; y++ {
		for x := -1; x < 2; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 25, G: 90, B: 175, A: 255})
		}
	}
	source.SetNRGBA(-2, 5, color.NRGBA{R: 10, G: 80, B: 150, A: 0})
	source.SetNRGBA(2, 9, key)
	source.SetNRGBA(0, 7, color.NRGBA{R: 25, G: 90, B: 175, A: 255})

	atlas, err := Pack([]Sprite{{
		ID:    "combined",
		Image: source,
	}}, Options{
		MinSize:         1,
		MaxSize:         8,
		TrimTransparent: true,
		ColorKey:        &ColorKeyOptions{Color: key, ToAlpha: true},
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 3 || placement.Height != 3 ||
		placement.OffsetX != 1 || placement.OffsetY != 1 {
		t.Fatalf("transformed-alpha crop = %+v, want 3x3 at offset 1,1", placement)
	}
}

func TestPackColorKeyCombinesWithTransparentTrimAndDedup(t *testing.T) {
	key := color.NRGBA{R: 255, G: 0, B: 255, A: 255}

	first := keyedContentImage(
		image.Rect(-3, 4, 2, 9),
		key,
		image.Point{-2, 5},
	)
	second := keyedContentImage(
		image.Rect(10, -8, 17, -2),
		key,
		image.Point{12, -7},
	)

	first.SetNRGBA(-3, 4, color.NRGBA{R: 20, G: 90, B: 180, A: 0})
	second.SetNRGBA(10, -8, color.NRGBA{R: 80, G: 20, B: 90, A: 0})

	keyOptions := &ColorKeyOptions{Color: key, Trim: true, ToAlpha: true}
	atlas, err := Pack([]Sprite{
		{ID: "first", Image: first},
		{ID: "second", Image: second},
	}, Options{
		MinSize:         2,
		MaxSize:         8,
		TrimTransparent: true,
		Deduplicate:     true,
		ColorKey:        keyOptions,
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(atlas.Layout.Placements) != 2 {
		t.Fatalf("placements = %d, want logical canonical and alias", len(atlas.Layout.Placements))
	}

	var canonical, alias *Placement
	for i := range atlas.Layout.Placements {
		placement := &atlas.Layout.Placements[i]
		if placement.AliasOf == "" {
			canonical = placement
		} else {
			alias = placement
		}
	}
	if canonical == nil || alias == nil || alias.AliasOf != canonical.ID {
		t.Fatalf("dedup placements = %+v", atlas.Layout.Placements)
	}
	if canonical.Width != 1 || canonical.Height != 1 {
		t.Fatalf("canonical crop = %dx%d, want 1x1", canonical.Width, canonical.Height)
	}
	if alias.OriginalWidth != 7 || alias.OriginalHeight != 6 ||
		alias.OffsetX != 2 || alias.OffsetY != 1 {
		t.Fatalf("alias trim metadata = %+v", alias)
	}

	if got := color.RGBAModel.
		Convert(atlas.Image.At(canonical.X, canonical.Y)).(color.RGBA); got != canonicalRGBA8(color.NRGBA{
		R: 30,
		G: 140,
		B: 210,
		A: 255,
	}) {
		t.Fatalf("canonical content pixel = %#v", got)
	}
}

func TestColorKeyPixelFormatsProduceSameTrimmedAtlas(t *testing.T) {
	key := color.NRGBA{R: 210, G: 120, B: 40, A: 128}
	keyRGBA := canonicalRGBA8(key)
	content := color.NRGBA{R: 20, G: 190, B: 60, A: 255}
	contentRGBA := canonicalRGBA8(content)
	newImage := func(set func(image.Image)) image.Image {
		img := image.NewNRGBA(image.Rect(-3, 5, 0, 8))
		for y := 5; y < 8; y++ {
			for x := -3; x < 0; x++ {
				img.SetNRGBA(x, y, key)
			}
		}
		img.SetNRGBA(-2, 6, content)
		set(img)

		return img
	}

	base := image.NewNRGBA(image.Rect(-8, 1, 4, 13))
	for y := 1; y < 13; y++ {
		for x := -8; x < 4; x++ {
			base.SetNRGBA(x, y, key)
		}
	}
	base.SetNRGBA(-5, 8, content)
	subImage := base.SubImage(image.Rect(-6, 7, -3, 10))

	paletted := image.NewPaletted(image.Rect(-3, 5, 0, 8), color.Palette{key, content})
	for y := 5; y < 8; y++ {
		for x := -3; x < 0; x++ {
			paletted.SetColorIndex(x, y, 0)
		}
	}
	paletted.SetColorIndex(-2, 6, 1)

	rgba := image.NewRGBA(image.Rect(-3, 5, 0, 8))
	for y := 5; y < 8; y++ {
		for x := -3; x < 0; x++ {
			rgba.SetRGBA(x, y, keyRGBA)
		}
	}
	rgba.SetRGBA(-2, 6, contentRGBA)

	rgba64 := image.NewRGBA64(image.Rect(-3, 5, 0, 8))
	for y := 5; y < 8; y++ {
		for x := -3; x < 0; x++ {
			rgba64.Set(x, y, keyRGBA)
		}
	}
	rgba64.Set(-2, 6, contentRGBA)

	images := []struct {
		name string
		img  image.Image
	}{
		{name: "NRGBA", img: newImage(func(image.Image) {})},
		{name: "RGBA", img: rgba},
		{name: "Paletted", img: paletted},
		{name: "RGBA64", img: rgba64},
		{name: "SubImage", img: subImage},
		{name: "generic", img: genericColorKeyImage{bounds: image.Rect(-3, 5, 0, 8), key: key, content: content}},
	}

	var want image.Image
	for _, test := range images {
		t.Run(test.name, func(t *testing.T) {
			atlas, err := Pack([]Sprite{{ID: test.name, Image: test.img}}, Options{
				MinSize: 1,
				MaxSize: 4,
				ColorKey: &ColorKeyOptions{
					Color:   key,
					Trim:    true,
					ToAlpha: true,
				},
			})
			if err != nil {
				t.Fatalf("Pack: %v", err)
			}

			if len(atlas.Layout.Placements) != 1 ||
				atlas.Layout.Placements[0].Width != 1 ||
				atlas.Layout.Placements[0].Height != 1 {
				t.Fatalf("trimmed placement = %+v", atlas.Layout.Placements)
			}

			if got := color.RGBAModel.Convert(
				atlas.Image.At(atlas.Layout.Placements[0].X, atlas.Layout.Placements[0].Y),
			).(color.RGBA); got != contentRGBA {
				t.Fatalf("content pixel = %#v, want %#v", got, contentRGBA)
			}

			if want == nil {
				want = atlas.Image
			} else if !imagesEqual(atlas.Image, want) {
				t.Fatal("canonical output differs from other source formats")
			}
		})
	}
}

func TestColorKeyRenderRoundTripPagesAndOptionCopy(t *testing.T) {
	key := &ColorKeyOptions{
		Color:   color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		Trim:    true,
		ToAlpha: true,
	}
	originalKey := *key

	first := keyedContentImage(
		image.Rect(0, 0, 3, 3),
		key.Color,
		image.Point{X: 1, Y: 1},
	)
	second := keyedContentImage(
		image.Rect(4, -2, 7, 1),
		key.Color,
		image.Point{X: 5, Y: -1},
	)

	items := []Item{
		{ID: "first", Width: 3, Height: 3},
		{ID: "second", Width: 3, Height: 3},
	}
	sources := []Source{
		{ID: "first", Image: first},
		{ID: "second", Image: second},
	}
	options := Options{MinSize: 1, MaxSize: 1, ColorKey: key}

	packed, err := PackPages([]Sprite{
		{ID: "first", Image: first},
		{ID: "second", Image: second},
	}, options)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 2 {
		t.Fatalf("pages = %d, want 2", len(packed))
	}

	key.Color = color.NRGBA{R: 1, G: 2, B: 3, A: 255}
	for page := range packed {
		settings := packed[page].Layout.Render
		if settings == nil ||
			settings.ColorKey == nil ||
			settings.ColorKey.Color.A != 255 ||
			settings.ColorKey.Color.R != 255 {
			t.Fatalf("page %d did not retain a copied ColorKey: %+v", page, settings)
		}

		encoded, err := json.Marshal(packed[page].Layout)
		if err != nil {
			t.Fatalf("marshal page %d layout: %v", page, err)
		}

		var layout Layout
		if err := json.Unmarshal(encoded, &layout); err != nil {
			t.Fatalf("unmarshal page %d layout: %v", page, err)
		}

		want, err := Render(&layout, sources)
		if err != nil {
			t.Fatalf("Render page %d: %v", page, err)
		}

		got, err := RenderWithLoader(&layout, func(id string) (image.Image, error) {
			if id == "first" {
				return first, nil
			}
			return second, nil
		})
		if err != nil {
			t.Fatalf("RenderWithLoader page %d: %v", page, err)
		}
		if !imagesEqual(got, want) || !imagesEqual(got, packed[page].Image) {
			t.Fatalf("page %d render results differ", page)
		}
	}
	key.Color = originalKey.Color

	var built []*Atlas
	err = BuildPages(items, options, func(id string) (image.Image, error) {
		if id == "first" {
			return first, nil
		}
		return second, nil
	}, func(_ int, atlas *Atlas) error {
		built = append(built, atlas)
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if len(built) != len(packed) {
		t.Fatalf("BuildPages pages = %d, PackPages pages = %d", len(built), len(packed))
	}
	for i := range built {
		if !imagesEqual(built[i].Image, packed[i].Image) {
			t.Errorf("BuildPages page %d differs from PackPages", i)
		}
	}
}

func TestColorKeyDeduplicatesAfterTransformAndPreservesAliasTrim(t *testing.T) {
	key := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	first := keyedContentImage(
		image.Rect(0, 0, 5, 5),
		key,
		image.Point{X: 2, Y: 2},
	)
	second := keyedContentImage(
		image.Rect(10, -4, 17, 2),
		key,
		image.Point{X: 13, Y: -1},
	)

	for y := 1; y < 4; y++ {
		for x := 1; x < 4; x++ {
			first.SetNRGBA(x, y, color.NRGBA{R: 40, G: 110, B: 180, A: 255})
		}
	}
	for y := -2; y < 1; y++ {
		for x := 12; x < 15; x++ {
			second.SetNRGBA(x, y, color.NRGBA{R: 40, G: 110, B: 180, A: 255})
		}
	}

	first.SetNRGBA(1, 1, key)
	second.SetNRGBA(12, -2, color.NRGBA{})

	atlas, err := Pack([]Sprite{
		{ID: "first", Image: first},
		{ID: "second", Image: second},
	}, Options{
		MinSize:     4,
		MaxSize:     8,
		Deduplicate: true,
		ColorKey:    &ColorKeyOptions{Color: key, Trim: true, ToAlpha: true},
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(atlas.Layout.Placements) != 2 {
		t.Fatalf("placements = %+v", atlas.Layout.Placements)
	}

	var alias *Placement
	for i := range atlas.Layout.Placements {
		if atlas.Layout.Placements[i].AliasOf != "" {
			alias = &atlas.Layout.Placements[i]
		}
	}
	if alias == nil || alias.ID != "second" || alias.AliasOf != "first" {
		t.Fatalf("expected second to alias first: %+v", atlas.Layout.Placements)
	}
	if alias.OriginalWidth != 7 || alias.OriginalHeight != 6 ||
		alias.OffsetX != 2 || alias.OffsetY != 2 {
		t.Fatalf("alias crop metadata = %+v", alias)
	}
}

func TestColorKeyValidationAndGeometryOnlyPlanning(t *testing.T) {
	options := Options{
		MinSize:  1,
		MaxSize:  8,
		ColorKey: &ColorKeyOptions{},
	}
	if _, err := Plan([]Item{{
		ID:     "a",
		Width:  1,
		Height: 1,
	}}, options); err == nil || !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Plan empty ColorKey error = %v, want ErrInvalidOptions", err)
	}

	options.ColorKey = &ColorKeyOptions{Color: color.NRGBA{A: 255}, Trim: true}
	layout, err := Plan([]Item{{ID: "a", Width: 5, Height: 4}}, options)
	if err != nil {
		t.Fatalf("Plan with ColorKey: %v", err)
	}
	if layout.Placements[0].Width != 5 || layout.Placements[0].Height != 4 || layout.Render != nil {
		t.Fatalf("geometry-only Plan applied ColorKey: %+v", layout)
	}

	pages, err := PlanPages([]Item{{ID: "a", Width: 5, Height: 4}}, options)
	if err != nil {
		t.Fatalf("PlanPages with ColorKey: %v", err)
	}
	if pages[0].Placements[0].Width != 5 ||
		pages[0].Placements[0].Height != 4 ||
		pages[0].Render != nil {
		t.Fatalf("geometry-only PlanPages applied ColorKey: %+v", pages[0])
	}
}

func TestColorKeyNilKeepsLegacyLayoutJSON(t *testing.T) {
	legacy, err := json.Marshal(Layout{
		Placements: []Placement{{ID: "a", Width: 1, Height: 1}},
		Width:      1,
		Height:     1,
	})
	if err != nil {
		t.Fatalf("marshal legacy layout: %v", err)
	}
	if bytes.Contains(legacy, []byte("color_key")) {
		t.Fatalf("nil ColorKey changed legacy JSON: %s", legacy)
	}
}

func TestColorKeyTrimmingUsesCanonicalTransparentFallback(t *testing.T) {
	key := color.NRGBA{R: 230, G: 40, B: 80, A: 255}
	source := image.NewNRGBA(image.Rect(-2, 3, 2, 6))
	for y := 3; y < 6; y++ {
		for x := -2; x < 2; x++ {
			source.SetNRGBA(x, y, key)
		}
	}

	atlas, err := Pack([]Sprite{{ID: "empty", Image: source}}, Options{
		MinSize: 1,
		MaxSize: 4,
		ColorKey: &ColorKeyOptions{
			Color:   key,
			Trim:    true,
			ToAlpha: true,
		},
	})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}

	placement := atlas.Layout.Placements[0]
	if placement.Width != 1 || placement.Height != 1 ||
		placement.OriginalWidth != 4 || placement.OriginalHeight != 3 ||
		placement.OffsetX != 0 || placement.OffsetY != 0 {
		t.Fatalf("full-key fallback placement = %+v", placement)
	}
	if got := color.RGBAModel.Convert(
		atlas.Image.At(placement.X, placement.Y),
	).(color.RGBA); got != (color.RGBA{}) {
		t.Fatalf("full-key fallback pixel = %#v, want transparent zero", got)
	}
}

func TestRenderColorKeyWithRotationAndExtrusion(t *testing.T) {
	key := color.NRGBA{R: 245, G: 240, B: 235, A: 255}
	source := image.NewNRGBA(image.Rect(-2, 4, 0, 7))
	for y := 4; y < 7; y++ {
		for x := -2; x < 0; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 40, G: 110, B: 180, A: 255})
		}
	}
	source.SetNRGBA(-2, 4, key)
	source.SetNRGBA(-1, 5, key)
	layout := &Layout{
		Render: &RenderSettings{
			Padding: 1,
			Extrude: 1,
			ColorKey: &ColorKeyOptions{
				Color:   key,
				ToAlpha: true,
			},
		},
		Placements: []Placement{{ID: "rotated", X: 1, Y: 1, Width: 2, Height: 3, Rotated: true}},
		Width:      5,
		Height:     4,
	}

	got, err := Render(layout, []Source{{ID: "rotated", Image: source}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if pixel := canonicalRGBA8(got.At(3, 1)); pixel != (color.RGBA{}) {
		t.Errorf("rotated key pixel = %#v, want transparent zero", pixel)
	}
	if pixel := canonicalRGBA8(got.At(2, 2)); pixel != (color.RGBA{}) {
		t.Errorf("rotated interior key pixel = %#v, want transparent zero", pixel)
	}
	if pixel := canonicalRGBA8(got.At(1, 1)); pixel != canonicalRGBA8(color.NRGBA{
		R: 40,
		G: 110,
		B: 180,
		A: 255,
	}) {
		t.Errorf("rotated non-key pixel = %#v", pixel)
	}
}

// keyedContentImage creates a key-colored canvas with one opaque content pixel.
func keyedContentImage(bounds image.Rectangle, key color.NRGBA, content image.Point) *image.NRGBA {
	img := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetNRGBA(x, y, key)
		}
	}

	img.SetNRGBA(content.X, content.Y, color.NRGBA{R: 30, G: 140, B: 210, A: 255})
	return img
}

// genericColorKeyImage exposes arbitrary color.Color values through image.Image.
type genericColorKeyImage struct {
	bounds  image.Rectangle // bounds defines source coordinates including a nonzero origin.
	key     color.NRGBA     // key is returned outside the single content pixel.
	content color.NRGBA     // content is returned at the center of bounds.
}

// Bounds returns the configured generic image bounds.
func (img genericColorKeyImage) Bounds() image.Rectangle {
	return img.bounds
}

// ColorModel reports straight-alpha source colors.
func (genericColorKeyImage) ColorModel() color.Model {
	return color.NRGBAModel
}

// At returns the center content pixel and the key color elsewhere.
func (img genericColorKeyImage) At(x, y int) color.Color {
	center := image.Point{
		X: img.bounds.Min.X + img.bounds.Dx()/2,
		Y: img.bounds.Min.Y + img.bounds.Dy()/2,
	}
	if x == center.X && y == center.Y {
		return img.content
	}

	return img.key
}
