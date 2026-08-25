// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"image"
	"image/color"
	"testing"
)

func FuzzRender(f *testing.F) {
	for _, seed := range renderFuzzSeeds() {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		input := decodeRenderFuzzInput(data)
		got, err := Render(input.layout, input.sources)
		if input.wantErr != nil {
			if !errors.Is(err, input.wantErr) {
				t.Fatalf("Render error = %v, want errors.Is(_, %v)", err, input.wantErr)
			}
			if got != nil {
				t.Fatalf("Render returned image %T with error %v", got, err)
			}

			return
		}
		if err != nil {
			t.Fatalf("Render: %v", err)
		}

		assertRenderFuzzPixels(t, input.layout, input.sources, got)
	})
}

// renderFuzzInput contains a bounded render case and its expected error.
type renderFuzzInput struct {
	layout  *Layout
	sources []Source
	wantErr error
}

// decodeRenderFuzzInput limits arbitrary bytes to small layouts and images.
func decodeRenderFuzzInput(data []byte) renderFuzzInput {
	if len(data) > 256 {
		data = data[:256]
	}
	byteAt := func(index int) byte {
		if index < len(data) {
			return data[index]
		}

		return 0
	}

	switch byteAt(0) % 10 {
	case 1:
		return renderFuzzInput{wantErr: ErrLayoutRequired}

	case 2:
		return invalidRenderGeometry(byteAt(1) % 8)

	case 3:
		return renderFuzzInput{
			layout:  renderFuzzLayout(2, 2, Placement{ID: "sprite", Width: 2, Height: 2}),
			wantErr: ErrSourceNotFound,
		}

	case 4:
		return renderFuzzInput{
			layout:  renderFuzzLayout(2, 2, Placement{ID: "sprite", Width: 2, Height: 2}),
			sources: []Source{{Image: image.NewRGBA(image.Rect(0, 0, 2, 2))}},
			wantErr: ErrSourceNotFound,
		}

	case 5:
		return renderFuzzInput{
			layout:  renderFuzzLayout(2, 2, Placement{ID: "sprite", Width: 2, Height: 2}),
			sources: []Source{{ID: "sprite", Image: (*renderFuzzNilImage)(nil)}},
			wantErr: ErrSourceImageMissing,
		}

	case 6:
		return renderFuzzInput{
			layout:  renderFuzzLayout(2, 2, Placement{ID: "sprite", Width: 2, Height: 2}),
			sources: []Source{{ID: "sprite", Image: image.NewRGBA(image.Rect(0, 0, 1, 2))}},
			wantErr: ErrSourceSizeMismatch,
		}

	case 7:
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		return renderFuzzInput{
			layout: renderFuzzLayout(2, 2),
			sources: []Source{
				{ID: "duplicate", Image: img},
				{ID: "duplicate", Image: img},
			},
			wantErr: ErrInvalidItem,
		}

	case 8:
		return renderFuzzInput{
			layout:  renderFuzzLayout(0, 2),
			wantErr: ErrInvalidLayout,
		}

	case 9:
		return invalidRenderGeometry(4)

	default:
		return validRenderFuzzInput(byteAt)
	}
}

// invalidRenderGeometry selects malformed placements with a specific error.
func invalidRenderGeometry(variant byte) renderFuzzInput {
	placement := Placement{ID: "sprite", Width: 1, Height: 1}
	switch variant {
	case 0:
		placement.X = -1

	case 1:
		placement.Y = -1

	case 2:
		placement.Width = 0

	case 3:
		placement.Height = -1

	case 4:
		placement.X = 3
		placement.Width = 2

	case 5:
		placement.Rotated = true
		placement.Width = 2
		placement.Height = 1
		placement.X = 4

	case 6:
		placement.Rotated = true
		placement.Width = 2
		placement.Height = 1
		placement.Y = 3

	case 7:
		placement.Rotated = true
		placement.Width = 1
		placement.Height = 2
		placement.X = 3
	}

	return renderFuzzInput{
		layout:  renderFuzzLayout(4, 4, placement),
		sources: []Source{{ID: "sprite", Image: image.NewRGBA(image.Rect(0, 0, max(placement.Width, 1), max(placement.Height, 1)))}},
		wantErr: ErrInvalidLayout,
	}
}

// validRenderFuzzInput builds non-overlapping bounded placements and images.
func validRenderFuzzInput(byteAt func(int) byte) renderFuzzInput {
	count := int(byteAt(1) % 9)
	layout := &Layout{Placements: make([]Placement, 0, count)}
	sources := make([]Source, 0, count)
	if count == 0 {
		layout.Width = 1 + int(byteAt(2)%64)
		layout.Height = 1 + int(byteAt(3)%64)

		return renderFuzzInput{layout: layout, sources: sources}
	}

	maxDimension := min(16, 64/count)
	x := 0
	height := 0
	for i := range count {
		base := 4 + i*5
		width := 1 + int(byteAt(base)%byte(maxDimension))
		itemHeight := 1 + int(byteAt(base+1)%byte(maxDimension))
		rotated := byteAt(base+2)&1 != 0
		effectiveWidth, effectiveHeight := width, itemHeight
		if rotated {
			effectiveWidth, effectiveHeight = itemHeight, width
		}

		id := renderFuzzID(i)
		layout.Placements = append(layout.Placements, Placement{
			ID:      id,
			X:       x,
			Y:       0,
			Width:   width,
			Height:  itemHeight,
			Rotated: rotated,
		})
		sources = append(sources, Source{
			ID:    id,
			Image: renderFuzzImage(width, itemHeight, int(byteAt(base+3)%6), int(byteAt(base+4))),
		})
		x += effectiveWidth
		height = max(height, effectiveHeight)
	}
	layout.Width = x
	layout.Height = height

	return renderFuzzInput{layout: layout, sources: sources}
}

// assertRenderFuzzPixels compares the entire atlas with an independent mapping.
func assertRenderFuzzPixels(t *testing.T, layout *Layout, sources []Source, got image.Image) {
	t.Helper()
	if got == nil {
		t.Fatal("Render returned a nil image without an error")
	}
	if got.Bounds() != image.Rect(0, 0, layout.Width, layout.Height) {
		t.Fatalf("Render bounds = %v, want %v", got.Bounds(), image.Rect(0, 0, layout.Width, layout.Height))
	}

	sourceByID := make(map[string]image.Image, len(sources))
	for _, source := range sources {
		sourceByID[source.ID] = source.Image
	}

	want := image.NewRGBA(got.Bounds())
	for _, placement := range layout.Placements {
		src := sourceByID[placement.ID]
		bounds := src.Bounds()
		for sy := 0; sy < bounds.Dy(); sy++ {
			for sx := 0; sx < bounds.Dx(); sx++ {
				dx, dy := placement.X+sx, placement.Y+sy
				if placement.Rotated {
					dx = placement.X + bounds.Dy() - 1 - sy
					dy = placement.Y + sx
				}
				pixel := color.RGBAModel.Convert(src.At(bounds.Min.X+sx, bounds.Min.Y+sy)).(color.RGBA)
				want.SetRGBA(dx, dy, pixel)
			}
		}
	}

	for y := 0; y < layout.Height; y++ {
		for x := 0; x < layout.Width; x++ {
			gotPixel := color.RGBAModel.Convert(got.At(x, y)).(color.RGBA)
			wantPixel := want.RGBAAt(x, y)
			if gotPixel != wantPixel {
				t.Fatalf("pixel (%d,%d) = %+v, want %+v", x, y, gotPixel, wantPixel)
			}
		}
	}
}

// renderFuzzImage creates a small patterned image with the requested bounds.
func renderFuzzImage(width, height, imageType, salt int) image.Image {
	origin := image.Point{}
	switch imageType {
	case 3:
		origin = image.Pt(2, 2)

	case 4:
		origin = image.Pt(-3, -2)

	case 5:
		return renderFuzzCustomImage{
			bounds: image.Rect(-2, 3, -2+width, 3+height),
			salt:   salt,
		}
	}

	bounds := image.Rectangle{Min: origin, Max: image.Pt(origin.X+width, origin.Y+height)}

	switch imageType {
	case 0:
		img := image.NewRGBA(bounds)
		fillRenderFuzzRGBA(img, bounds, salt)
		return img

	case 1:
		img := image.NewNRGBA(bounds)
		fillRenderFuzzNRGBA(img, bounds, salt)
		return img

	case 2:
		img := image.NewPaletted(bounds, renderFuzzPalette())
		fillRenderFuzzPaletted(img, bounds, salt)
		return img

	case 3, 4:
		parentBounds := image.Rect(origin.X-2, origin.Y-2, origin.X+width+2, origin.Y+height+2)
		parent := image.NewRGBA(parentBounds)
		fillRenderFuzzRGBA(parent, bounds, salt)
		return parent.SubImage(bounds)

	default:
		panic("unreachable image type")
	}
}

// fillRenderFuzzRGBA writes distinct premultiplied pixels into an RGBA image.
func fillRenderFuzzRGBA(img *image.RGBA, bounds image.Rectangle, salt int) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			alpha := renderFuzzAlpha(x, y, salt)
			img.SetRGBA(x, y, color.RGBA{
				R: renderFuzzChannel(x, y, salt) * alpha / 255,
				G: renderFuzzChannel(y, x, salt+17) * alpha / 255,
				B: renderFuzzChannel(x+y, x-y, salt+31) * alpha / 255,
				A: alpha,
			})
		}
	}
}

// fillRenderFuzzNRGBA writes varied unpremultiplied colors into an NRGBA image.
func fillRenderFuzzNRGBA(img *image.NRGBA, bounds image.Rectangle, salt int) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetNRGBA(x, y, renderFuzzNRGBAColor(x, y, salt))
		}
	}
}

// fillRenderFuzzPaletted chooses varied entries in a palette with alpha values.
func fillRenderFuzzPaletted(img *image.Paletted, bounds image.Rectangle, salt int) {
	paletteIndexes := [...]uint8{0, 1, 2, 3, 4}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			index := (x*3 + y*5 + salt) % len(paletteIndexes)
			img.SetColorIndex(x, y, paletteIndexes[index])
		}
	}
}

// renderFuzzPalette contains transparent, premultiplied, and NRGBA colors.
func renderFuzzPalette() color.Palette {
	return color.Palette{
		color.RGBA{A: 0},
		color.RGBA{R: 64, G: 32, B: 16, A: 128},
		color.NRGBA{R: 255, G: 64, B: 17, A: 127},
		color.NRGBA{R: 13, G: 220, B: 91, A: 128},
		color.NRGBA{R: 37, G: 81, B: 199, A: 255},
	}
}

// renderFuzzNRGBAColor creates a deterministic pixel with varied alpha.
func renderFuzzNRGBAColor(x, y, salt int) color.NRGBA {
	return color.NRGBA{
		R: renderFuzzChannel(x, y, salt),
		G: renderFuzzChannel(y, x, salt+17),
		B: renderFuzzChannel(x+y, x-y, salt+31),
		A: renderFuzzAlpha(x, y, salt),
	}
}

// renderFuzzAlpha selects edge alpha values, including transparent pixels.
func renderFuzzAlpha(x, y, salt int) uint8 {
	values := [...]uint8{0, 1, 127, 128, 255}
	index := (x*7 + y*11 + salt) % len(values)
	if index < 0 {
		index += len(values)
	}

	return values[index]
}

// renderFuzzChannel derives a stable color channel from coordinates and salt.
func renderFuzzChannel(x, y, salt int) uint8 {
	value := x*37 + y*19 + salt*13
	value %= 256
	if value < 0 {
		value += 256
	}
	value &= 0xff

	return uint8(value)
}

// renderFuzzID returns a stable, unique source ID for a placement index.
func renderFuzzID(index int) string {
	ids := [...]string{"a", "b", "c", "d", "e", "f", "g", "h"}
	return ids[index]
}

// renderFuzzLayout constructs a layout with the supplied dimensions and placements.
func renderFuzzLayout(width, height int, placements ...Placement) *Layout {
	return &Layout{Width: width, Height: height, Placements: placements}
}

// renderFuzzSeeds covers positive image types and known invalid render cases.
func renderFuzzSeeds() [][]byte {
	seeds := make([][]byte, 0, 44)
	seeds = append(seeds,
		[]byte{0, 0},
		[]byte{0, 1, 0, 0, 1, 1, 0, 0, 0},
		[]byte{0, 8, 0, 0, 1, 1, 0, 0, 0},
		[]byte{0, 1, 0, 0, 1, 1, 1, 3, 0},
		[]byte{0, 1, 0, 0, 1, 1, 1, 4, 0},
		[]byte{0, 1, 0, 0, 1, 8, 0, 0, 0},
		[]byte{0, 1, 0, 0, 8, 1, 1, 0, 0},
	)
	for imageType := range 6 {
		imageTypeByte := byte(imageType)
		seeds = append(seeds, []byte{0, 1, 0, 0, 1, 1, imageTypeByte & 1, imageTypeByte, 0})
	}
	for imageType := range 3 {
		for alpha := range 5 {
			seeds = append(seeds, []byte{0, 1, 0, 0, 1, 1, 0, byte(imageType), byte(alpha)})
		}
	}
	seeds = append(seeds,
		[]byte{1},
		[]byte{2, 0},
		[]byte{2, 1},
		[]byte{2, 2},
		[]byte{2, 3},
		[]byte{2, 4},
		[]byte{2, 5},
		[]byte{2, 6},
		[]byte{2, 7},
		[]byte{3},
		[]byte{4},
		[]byte{5},
		[]byte{6},
		[]byte{7},
		[]byte{8},
		[]byte{9},
	)

	return seeds
}

// renderFuzzCustomImage returns deterministic pixels from arbitrary coordinates.
type renderFuzzCustomImage struct {
	bounds image.Rectangle
	salt   int
}

// ColorModel reports the custom image's unpremultiplied color model.
func (i renderFuzzCustomImage) ColorModel() color.Model {
	return color.NRGBAModel
}

// Bounds reports the custom image's offset coordinate space.
func (i renderFuzzCustomImage) Bounds() image.Rectangle {
	return i.bounds
}

// At returns a stable pixel pattern for coordinates inside the image bounds.
func (i renderFuzzCustomImage) At(x, y int) color.Color {
	return renderFuzzNRGBAColor(x, y, i.salt)
}

// renderFuzzNilImage is a typed nil source used by negative fuzz cases.
type renderFuzzNilImage struct{}

// ColorModel satisfies image.Image for a typed nil fuzz source.
func (*renderFuzzNilImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds satisfies image.Image for a typed nil fuzz source.
func (*renderFuzzNilImage) Bounds() image.Rectangle {
	return image.Rectangle{}
}

// At satisfies image.Image for a typed nil fuzz source.
func (*renderFuzzNilImage) At(int, int) color.Color {
	return color.RGBA{}
}
