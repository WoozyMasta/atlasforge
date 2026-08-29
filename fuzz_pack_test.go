// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"reflect"
	"testing"
)

func FuzzPack(f *testing.F) {
	for _, seed := range packFuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		input := decodePackFuzzInput(data)
		original := append([]Sprite(nil), input.sprites...)

		got, packErr := Pack(input.sprites, input.options)
		assertPackFuzzSpritesUnchanged(t, original, input.sprites)

		if input.wantErr != nil {
			if !errors.Is(packErr, input.wantErr) {
				t.Fatalf("Pack error = %v, want errors.Is(_, %v)", packErr, input.wantErr)
			}
			if got != nil {
				t.Fatalf("Pack returned atlas %v with error %v", got, packErr)
			}
			if input.compareReference {
				_, _, referenceErr := packFuzzPlanRender(input.sprites, input.options)
				assertPackFuzzErrorCategory(t, packErr, referenceErr)
			}

			return
		}

		wantLayout, wantImage, referenceErr := packFuzzPlanRender(input.sprites, input.options)
		if referenceErr != nil {
			assertPackFuzzErrorCategory(t, packErr, referenceErr)
			if got != nil {
				t.Fatalf("Pack returned atlas %v after reference error %v", got, referenceErr)
			}

			return
		}

		if packErr != nil {
			t.Fatalf("Pack error = %v after successful Plan+Render", packErr)
		}
		if got == nil || got.Image == nil {
			t.Fatal("Pack returned a nil atlas or image without an error")
		}
		if !reflect.DeepEqual(got.Layout, *wantLayout) {
			t.Fatalf("Pack layout = %+v, want Plan layout %+v", got.Layout, *wantLayout)
		}
		assertPackFuzzImagesEqual(t, got.Image, wantImage)

		firstHash := packFuzzPixelHash(got.Image)
		repeated, repeatErr := Pack(input.sprites, input.options)
		if repeatErr != nil {
			t.Fatalf("repeated Pack: %v", repeatErr)
		}

		assertPackFuzzSpritesUnchanged(t, original, input.sprites)
		if !reflect.DeepEqual(got.Layout, repeated.Layout) {
			t.Fatalf("repeated Pack layout = %+v, first layout %+v", repeated.Layout, got.Layout)
		}
		if repeatedHash := packFuzzPixelHash(repeated.Image); repeatedHash != firstHash {
			t.Fatalf("repeated Pack pixel hash = %x, first hash %x", repeatedHash, firstHash)
		}

		assertPackFuzzImagesEqual(t, got.Image, repeated.Image)
	})
}

// packFuzzInput contains bounded sprites, valid options, and expected failure metadata.
type packFuzzInput struct {
	sprites          []Sprite
	options          Options
	wantErr          error
	compareReference bool
}

// decodePackFuzzInput limits arbitrary bytes to small sprite and atlas inputs.
func decodePackFuzzInput(data []byte) packFuzzInput {
	if len(data) > 256 {
		data = data[:256]
	}
	byteAt := func(index int) byte {
		if index < len(data) {
			return data[index]
		}

		return 0
	}

	options := packFuzzOptions(byteAt)
	switch byteAt(0) % 8 {
	case 1:
		return packFuzzInput{
			sprites:          []Sprite{{ID: "missing"}},
			options:          options,
			wantErr:          ErrSourceImageMissing,
			compareReference: false,
		}

	case 2:
		var typedNil *image.RGBA
		return packFuzzInput{
			sprites:          []Sprite{{ID: "typed-nil", Image: typedNil}},
			options:          options,
			wantErr:          ErrSourceImageMissing,
			compareReference: false,
		}

	case 3:
		first := packFuzzSprite(byteAt, 0, "duplicate")
		second := packFuzzSprite(byteAt, 1, "duplicate")
		return packFuzzInput{
			sprites:          []Sprite{first, second},
			options:          options,
			wantErr:          ErrInvalidItem,
			compareReference: true,
		}

	case 4:
		sprite := packFuzzSprite(byteAt, 0, "")
		return packFuzzInput{
			sprites:          []Sprite{sprite},
			options:          options,
			wantErr:          ErrInvalidItem,
			compareReference: true,
		}

	case 5:
		sprite := packFuzzSprite(byteAt, 0, "mismatch-width")
		sprite.Width = sprite.Image.Bounds().Dx() + 1
		return packFuzzInput{
			sprites:          []Sprite{sprite},
			options:          options,
			wantErr:          ErrSourceSizeMismatch,
			compareReference: true,
		}

	case 6:
		sprite := packFuzzSprite(byteAt, 0, "mismatch-height")
		sprite.Height = sprite.Image.Bounds().Dy() + 1
		return packFuzzInput{
			sprites:          []Sprite{sprite},
			options:          options,
			wantErr:          ErrSourceSizeMismatch,
			compareReference: true,
		}

	default:
		return packFuzzInput{
			sprites:          packFuzzSprites(byteAt),
			options:          options,
			compareReference: true,
		}
	}
}

// packFuzzOptions decodes only valid options and keeps atlas dimensions bounded.
func packFuzzOptions(byteAt func(int) byte) Options {
	penalties := [...]float64{0, 0.25, 1, 4}
	flags := byteAt(2)

	return Options{
		MinSize:       1 + int(byteAt(5)%32),
		MaxSize:       64 + int(byteAt(6)%193),
		Padding:       int(byteAt(7) % 4),
		AspectPenalty: penalties[byteAt(3)%byte(len(penalties))],
		Heuristic:     Heuristic(byteAt(4) % 6),
		AllowRotate:   flags&1 != 0,
		ForceSquare:   flags&2 != 0,
		PreferHeight:  flags&4 != 0,
	}
}

// packFuzzSprites generates up to sixteen valid sprites with optional matching overrides.
func packFuzzSprites(byteAt func(int) byte) []Sprite {
	count := int(byteAt(1) % 17)
	sprites := make([]Sprite, 0, count)
	for i := range count {
		sprites = append(sprites, packFuzzSprite(byteAt, i, packFuzzID(i)))
	}

	return sprites
}

// packFuzzSprite generates one image with bounded dimensions and optional exact overrides.
func packFuzzSprite(byteAt func(int) byte, index int, id string) Sprite {
	base := 8 + index*7
	width := 1 + int(byteAt(base)%32)
	height := 1 + int(byteAt(base+1)%32)
	imageType := int(byteAt(base+2) % 6)
	salt := int(byteAt(base + 3))
	img := renderFuzzImage(width, height, imageType, salt)
	sprite := Sprite{ID: id, Image: img}

	switch byteAt(base+4) % 4 {
	case 1:
		sprite.Width = width

	case 2:
		sprite.Height = height

	case 3:
		sprite.Width = width
		sprite.Height = height
	}

	return sprite
}

// packFuzzPlanRender builds the equivalent public Plan+Render pipeline.
func packFuzzPlanRender(sprites []Sprite, opts Options) (*Layout, image.Image, error) {
	items := make([]Item, 0, len(sprites))
	sources := make([]Source, 0, len(sprites))
	for _, sprite := range sprites {
		if isNilImage(sprite.Image) {
			return nil, nil, ErrSourceImageMissing
		}

		bounds := sprite.Image.Bounds()
		width, height := sprite.Width, sprite.Height
		if width <= 0 {
			width = bounds.Dx()
		}
		if height <= 0 {
			height = bounds.Dy()
		}

		items = append(items, Item{ID: sprite.ID, Width: width, Height: height})
		sources = append(sources, Source{ID: sprite.ID, Image: sprite.Image})
	}

	layout, err := Plan(items, opts)
	if err != nil {
		return nil, nil, err
	}

	img, err := Render(layout, sources)
	if err != nil {
		return layout, nil, err
	}

	return layout, img, nil
}

// assertPackFuzzErrorCategory checks that both APIs return the same known sentinel.
func assertPackFuzzErrorCategory(t *testing.T, got, want error) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("Pack/reference errors = %v/%v, want matching non-nil errors", got, want)
	}

	sentinels := [...]error{
		ErrInvalidOptions,
		ErrInvalidItem,
		ErrPlacementFailed,
		ErrSourceNotFound,
		ErrSourceImageMissing,
		ErrSourceSizeMismatch,
	}
	for _, sentinel := range sentinels {
		if errors.Is(got, sentinel) || errors.Is(want, sentinel) {
			if errors.Is(got, sentinel) && errors.Is(want, sentinel) {
				return
			}

			t.Fatalf("Pack/reference errors = %v/%v, different categories", got, want)
		}
	}

	t.Fatalf("Pack/reference errors = %v/%v, no matching known category", got, want)
}

// assertPackFuzzSpritesUnchanged checks the public sprite fields and image references.
func assertPackFuzzSpritesUnchanged(t *testing.T, before, after []Sprite) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("sprite count changed from %d to %d", len(before), len(after))
	}

	for i := range before {
		if before[i].ID != after[i].ID ||
			before[i].Width != after[i].Width ||
			before[i].Height != after[i].Height ||
			!reflect.DeepEqual(before[i].Image, after[i].Image) {
			t.Fatalf("Pack mutated sprite %d: before=%+v after=%+v", i, before[i], after[i])
		}
	}
}

// assertPackFuzzImagesEqual compares bounds and every pixel in RGBA space.
func assertPackFuzzImagesEqual(t *testing.T, got, want image.Image) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("images = %T/%T, want non-nil", got, want)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("image bounds = %v, want %v", got.Bounds(), want.Bounds())
	}

	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			gotPixel := color.RGBAModel.Convert(got.At(x, y)).(color.RGBA)
			wantPixel := color.RGBAModel.Convert(want.At(x, y)).(color.RGBA)
			if gotPixel != wantPixel {
				t.Fatalf("pixel (%d,%d) = %+v, want %+v", x, y, gotPixel, wantPixel)
			}
		}
	}
}

// packFuzzPixelHash hashes every converted pixel for repeatability checks.
func packFuzzPixelHash(img image.Image) uint64 {
	hash := fnv.New64a()
	bounds := img.Bounds()
	var pixel [4]byte
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			pixel = [4]byte{rgba.R, rgba.G, rgba.B, rgba.A}
			_, _ = hash.Write(pixel[:])
		}
	}

	return hash.Sum64()
}

// packFuzzID creates stable identifiers for up to sixteen generated sprites.
func packFuzzID(index int) string {
	return fmt.Sprintf("sprite-%d", index)
}

// packFuzzSeeds combines valid equivalence cases and invalid sprite inputs.
func packFuzzSeeds() [][]byte {
	seeds := make([][]byte, 0, 28)
	seeds = append(seeds,
		packFuzzSeed(0, 0, 0, 0, 0),
		packFuzzSeed(1, 0, 0, 0, 0),
		packFuzzSeed(2, 0, 0, 0, 0),
		packFuzzSeed(1, 1, 0, 0, 0),
		packFuzzSeed(1, 1, 1, 1, 3),
		packFuzzSeed(16, 0, 1, 2, 3),
		packFuzzSeed(2, 4, 2, 4, 1),
		[]byte{1},
		[]byte{2},
		[]byte{3},
		[]byte{4},
		[]byte{5},
		[]byte{6},
	)

	for _, heuristic := range [...]byte{0, 1, 2, 3, 4, 5} {
		seed := packFuzzSeed(2, 2, 0, 3, 1)
		seed[4] = heuristic
		seeds = append(seeds, seed)
	}
	for _, penalty := range [...]byte{0, 1, 2, 3} {
		seed := packFuzzSeed(1, 3, 0, 2, 0)
		seed[3] = penalty
		seeds = append(seeds, seed)
	}
	for _, override := range [...]byte{0, 1, 2, 3} {
		seeds = append(seeds, packFuzzSeed(2, 3, 3, 0, override))
	}

	largeImage := packFuzzSeed(1, 3, 0, 0, 1)
	largeImage[8] = 31
	largeImage[9] = 31
	seeds = append(seeds, largeImage)

	return seeds
}

// packFuzzSeed encodes a deterministic positive case with varied small images.
func packFuzzSeed(count, flags, imageType, padding, override byte) []byte {
	data := make([]byte, 8+int(count)*7)
	data[0] = 0
	data[1] = count
	data[2] = flags
	data[5] = 0
	data[6] = 192
	data[7] = padding

	for i := range int(count) {
		base := 8 + i*7
		data[base] = 2 + byte(i%3)
		data[base+1] = 3 + byte(i%2)
		data[base+2] = (imageType + byte(i)) % 6
		data[base+3] = byte(i * 7)
		data[base+4] = override % 4
	}

	return data
}
