// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestRenderRequiresLayout(t *testing.T) {
	t.Parallel()

	_, err := Render(nil, nil)
	if !errors.Is(err, ErrLayoutRequired) {
		t.Fatalf("error %v, want ErrLayoutRequired", err)
	}
}

func TestRenderRequiresSourcesForPlacements(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 8, Height: 8},
		},
	}
	_, err := Render(layout, nil)
	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("error %v, want ErrSourceNotFound", err)
	}
}

func TestRenderSizeMismatch(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 8, Height: 8},
		},
	}
	sources := []Source{
		{ID: "a", Image: image.NewRGBA(image.Rect(0, 0, 9, 8))},
	}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrSourceSizeMismatch) {
		t.Fatalf("error %v, want ErrSourceSizeMismatch", err)
	}
}

func TestRenderSubImagePreservesSourceBounds(t *testing.T) {
	t.Parallel()

	full := image.NewRGBA(image.Rect(0, 0, 4, 4))
	full.SetRGBA(2, 2, color.RGBA{R: 255, A: 255})
	sub := full.SubImage(image.Rect(2, 2, 3, 3))
	layout := &Layout{
		Width:  1,
		Height: 1,
		Placements: []Placement{
			{ID: "a", Width: 1, Height: 1},
		},
	}

	got, err := Render(layout, []Source{{ID: "a", Image: sub}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if pixel := got.At(0, 0); pixel != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("pixel at (0,0) = %v, want opaque red", pixel)
	}
}

func TestRenderPreservesNonZeroSourceBounds(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(-3, 5, -1, 7)
	nrgba := image.NewNRGBA(bounds)
	nrgba.SetNRGBA(-3, 5, color.NRGBA{R: 255, A: 255})
	nrgba.SetNRGBA(-2, 5, color.NRGBA{G: 255, A: 255})
	nrgba.SetNRGBA(-3, 6, color.NRGBA{B: 255, A: 255})
	nrgba.SetNRGBA(-2, 6, color.NRGBA{R: 255, G: 255, A: 255})

	palette := color.Palette{
		color.RGBA{R: 255, A: 255},
		color.RGBA{G: 255, A: 255},
		color.RGBA{B: 255, A: 255},
		color.RGBA{R: 255, B: 255, A: 255},
	}
	paletted := image.NewPaletted(bounds, palette)
	paletted.SetColorIndex(-3, 5, 0)
	paletted.SetColorIndex(-2, 5, 1)
	paletted.SetColorIndex(-3, 6, 2)
	paletted.SetColorIndex(-2, 6, 3)

	tests := []struct {
		name string
		src  image.Image
	}{
		{name: "NRGBA", src: nrgba},
		{name: "Paletted", src: paletted},
		{name: "custom image", src: offsetTestImage{bounds: bounds}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Render(layoutForSource(test.src), []Source{{ID: "a", Image: test.src}})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			gotBounds := test.src.Bounds()
			for y := 0; y < gotBounds.Dy(); y++ {
				for x := 0; x < gotBounds.Dx(); x++ {
					want := color.RGBAModel.Convert(test.src.At(gotBounds.Min.X+x, gotBounds.Min.Y+y))
					if pixel := color.RGBAModel.Convert(got.At(x, y)); pixel != want {
						t.Errorf("pixel at (%d,%d) = %v, want %v", x, y, pixel, want)
					}
				}
			}
		})
	}
}

func TestRenderRotatedSubImagePreservesSourceBounds(t *testing.T) {
	t.Parallel()

	full := image.NewRGBA(image.Rect(-2, 4, 2, 8))
	full.SetRGBA(-2, 4, color.RGBA{R: 255, A: 255})
	full.SetRGBA(-1, 4, color.RGBA{G: 255, A: 255})
	sub := full.SubImage(image.Rect(-2, 4, 0, 5))
	layout := &Layout{
		Width:  1,
		Height: 2,
		Placements: []Placement{
			{ID: "a", Width: 2, Height: 1, Rotated: true},
		},
	}

	got, err := Render(layout, []Source{{ID: "a", Image: sub}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rgba := got.(*image.RGBA)
	if pixel := rgba.RGBAAt(0, 0); pixel != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("top rotated pixel = %+v, want opaque red", pixel)
	}
	if pixel := rgba.RGBAAt(0, 1); pixel != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("bottom rotated pixel = %+v, want opaque green", pixel)
	}
}

func TestRenderRotatedRGBAFastPathMatchesPixelBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		make func() *image.RGBA
	}{
		{
			name: "one pixel",
			make: func() *image.RGBA {
				return image.NewRGBA(image.Rect(-4, 7, -3, 8))
			},
		},
		{
			name: "one column",
			make: func() *image.RGBA {
				return image.NewRGBA(image.Rect(3, -5, 4, 0))
			},
		},
		{
			name: "one row",
			make: func() *image.RGBA {
				return image.NewRGBA(image.Rect(-6, 2, -1, 3))
			},
		},
		{
			name: "small rectangle",
			make: func() *image.RGBA {
				return image.NewRGBA(image.Rect(-3, 5, -1, 8))
			},
		},
		{
			name: "larger rectangle",
			make: func() *image.RGBA {
				return image.NewRGBA(image.Rect(9, -11, 25, 5))
			},
		},
		{
			name: "subimage with parent stride",
			make: func() *image.RGBA {
				parent := image.NewRGBA(image.Rect(-10, 20, 20, 50))

				return parent.SubImage(image.Rect(-5, 24, 5, 30)).(*image.RGBA)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			src := test.make()
			bounds := src.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					a := byte(((x-bounds.Min.X)*29 + (y-bounds.Min.Y)*47) & 0xff)
					src.SetRGBA(x, y, color.RGBA{R: a / 2, G: a / 3, B: a / 4, A: a})
				}
			}

			layout := &Layout{
				Width:  bounds.Dy(),
				Height: bounds.Dx(),
				Placements: []Placement{{
					ID:      "sprite",
					Width:   bounds.Dx(),
					Height:  bounds.Dy(),
					Rotated: true,
				}},
			}
			gotImage, err := Render(layout, []Source{{ID: "sprite", Image: src}})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			got, ok := gotImage.(*image.RGBA)
			if !ok {
				t.Fatalf("Render type = %T, want *image.RGBA", gotImage)
			}

			want := image.NewRGBA(image.Rect(0, 0, bounds.Dy(), bounds.Dx()))
			for sy := 0; sy < bounds.Dy(); sy++ {
				for sx := 0; sx < bounds.Dx(); sx++ {
					want.SetRGBA(bounds.Dy()-1-sy, sx, src.RGBAAt(bounds.Min.X+sx, bounds.Min.Y+sy))
				}
			}

			if !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("rendered pixel bytes differ from clockwise RGBA reference")
			}
		})
	}
}

// layoutForSource returns a layout sized to the source image bounds.
func layoutForSource(src image.Image) *Layout {
	bounds := src.Bounds()
	return &Layout{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Placements: []Placement{
			{ID: "a", Width: bounds.Dx(), Height: bounds.Dy()},
		},
	}
}

// offsetTestImage provides deterministic pixels in a non-zero coordinate space.
type offsetTestImage struct {
	bounds image.Rectangle
}

// ColorModel reports the pixel format used by offsetTestImage.
func (i offsetTestImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds reports the image's non-zero source coordinate space.
func (i offsetTestImage) Bounds() image.Rectangle {
	return i.bounds
}

// At returns a pixel whose channels encode its position within the image.
func (i offsetTestImage) At(x, y int) color.Color {
	var r, g uint8
	if x != i.bounds.Min.X {
		r = 1
	}
	if y != i.bounds.Min.Y {
		g = 1
	}

	return color.RGBA{R: r, G: g, A: 255}
}

func TestRenderInvalidLayoutSize(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  0,
		Height: 32,
	}
	_, err := Render(layout, nil)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderValidatesLayoutBeforeSources(t *testing.T) {
	t.Parallel()

	layout := &Layout{Width: 0, Height: 1}
	sources := []Source{{ID: "", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))}}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderRejectsInvalidPlacementGeometry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		placement Placement
	}{
		{name: "rotated right edge", placement: Placement{ID: "a", X: 2, Width: 2, Height: 1, Rotated: true}},
		{name: "negative x", placement: Placement{ID: "a", X: -1, Width: 1, Height: 1}},
		{name: "negative y", placement: Placement{ID: "a", Y: -1, Width: 1, Height: 1}},
		{name: "zero width", placement: Placement{ID: "a", Width: 0, Height: 1}},
		{name: "zero height", placement: Placement{ID: "a", Width: 1, Height: 0}},
		{name: "rotated effective width", placement: Placement{ID: "a", X: 1, Width: 1, Height: 2, Rotated: true}},
		{name: "rotated effective height", placement: Placement{ID: "a", Y: 1, Width: 2, Height: 1, Rotated: true}},
		{name: "unrotated clipping", placement: Placement{ID: "a", X: 1, Width: 2, Height: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			layout := &Layout{Width: 2, Height: 2, Placements: []Placement{test.placement}}
			src := image.NewRGBA(image.Rect(0, 0, test.placement.Width, test.placement.Height))
			got, err := Render(layout, []Source{{ID: "a", Image: src}})
			if !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("error %v, want ErrInvalidLayout", err)
			}
			if got != nil {
				t.Fatalf("image %v, want nil", got)
			}
		})
	}
}

func TestRenderRejectsUnrepresentableAtlasSize(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	layout := &Layout{Width: maxInt, Height: 1}

	_, err := Render(layout, nil)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
}

func TestRenderValidRotatedPlacementAtBoundary(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  2,
		Height: 2,
		Placements: []Placement{
			{ID: "a", X: 1, Y: 0, Width: 2, Height: 1, Rotated: true},
		},
	}
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, image.White)
	src.Set(1, 0, image.Black)

	got, err := Render(layout, []Source{{ID: "a", Image: src}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rgba := got.(*image.RGBA)
	if pixel := rgba.RGBAAt(1, 0); pixel.R != 255 || pixel.A != 255 {
		t.Fatalf("top rotated pixel mismatch: %+v", pixel)
	}
	if pixel := rgba.RGBAAt(1, 1); pixel.R != 0 || pixel.A != 255 {
		t.Fatalf("bottom rotated pixel mismatch: %+v", pixel)
	}
}

func TestRenderValidatesAllPlacementsBeforeDrawing(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  2,
		Height: 2,
		Placements: []Placement{
			{ID: "a", X: 0, Y: 0, Width: 1, Height: 1},
			{ID: "b", X: 2, Y: 0, Width: 1, Height: 1},
		},
	}
	sources := []Source{
		{ID: "a", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))},
		{ID: "b", Image: image.NewRGBA(image.Rect(0, 0, 1, 1))},
	}

	got, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidLayout) {
		t.Fatalf("error %v, want ErrInvalidLayout", err)
	}
	if got != nil {
		t.Fatalf("image %v, want nil", got)
	}
}

func TestRenderExtrudesNearestPixels(t *testing.T) {
	t.Parallel()

	inputs := makeRenderExtrusionTestInputs()

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			t.Parallel()

			for _, rotated := range []bool{false, true} {
				for _, depth := range []int{1, 2} {
					name := fmt.Sprintf("rotated=%t/depth=%d", rotated, depth)
					t.Run(name, func(t *testing.T) {
						t.Parallel()

						placement := Placement{ID: "sprite", X: 3, Y: 3, Width: 2, Height: 3, Rotated: rotated}
						baseLayout := Layout{
							Width:      8,
							Height:     9,
							Placements: []Placement{placement},
						}
						baseImage, err := Render(&baseLayout, []Source{{ID: "sprite", Image: input.image}})
						if err != nil {
							t.Fatalf("Render without extrusion: %v", err)
						}

						extrudedLayout := baseLayout
						extrudedLayout.Render = &RenderSettings{Padding: 2, Extrude: depth}
						gotImage, err := Render(&extrudedLayout, []Source{{ID: "sprite", Image: input.image}})
						if err != nil {
							t.Fatalf("Render with extrusion: %v", err)
						}

						base := baseImage.(*image.RGBA)
						got := gotImage.(*image.RGBA)
						width, height := placement.Width, placement.Height
						if rotated {
							width, height = height, width
						}

						for dy := -depth; dy < height+depth; dy++ {
							for dx := -depth; dx < width+depth; dx++ {
								clampedX := min(max(dx, 0), width-1)
								clampedY := min(max(dy, 0), height-1)
								want := base.RGBAAt(placement.X+clampedX, placement.Y+clampedY)
								if pixel := got.RGBAAt(placement.X+dx, placement.Y+dy); pixel != want {
									t.Errorf("pixel at (%d,%d) = %+v, want %+v", placement.X+dx, placement.Y+dy, pixel, want)
								}
							}
						}

						for y := 0; y < extrudedLayout.Height; y++ {
							for x := 0; x < extrudedLayout.Width; x++ {
								insideExtrusion := x >= placement.X-depth && x < placement.X+width+depth &&
									y >= placement.Y-depth && y < placement.Y+height+depth
								if !insideExtrusion && got.RGBAAt(x, y) != (color.RGBA{}) {
									t.Errorf("pixel outside extrusion at (%d,%d) = %+v, want transparent", x, y, got.RGBAAt(x, y))
								}
							}
						}
					})
				}
			}
		})
	}
}

func TestRenderRejectsInvalidExtrusionSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		layout Layout
	}{
		{
			name: "negative padding",
			layout: Layout{
				Width:  4,
				Height: 4,
				Render: &RenderSettings{Padding: -1},
			},
		},
		{
			name: "negative extrusion",
			layout: Layout{
				Width:  4,
				Height: 4,
				Render: &RenderSettings{Padding: 1, Extrude: -1},
			},
		},
		{
			name: "extrusion exceeds padding",
			layout: Layout{
				Width:  4,
				Height: 4,
				Render: &RenderSettings{Padding: 1, Extrude: 2},
			},
		},
		{
			name: "reserved padding outside atlas",
			layout: Layout{
				Width:      4,
				Height:     4,
				Placements: []Placement{{ID: "sprite", Width: 1, Height: 1}},
				Render:     &RenderSettings{Padding: 1, Extrude: 1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Render(&test.layout, nil)
			if !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("error %v, want ErrInvalidLayout", err)
			}
			if got != nil {
				t.Fatalf("image %v, want nil", got)
			}
		})
	}
}

func TestRenderExtrusionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	source := image.NewRGBA(image.Rect(-2, 5, 0, 8))
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			source.SetRGBA(x, y, color.RGBA{
				R: byte((20 + (x+2)*30) & 0xff),
				G: byte((30 + (y-5)*20) & 0xff),
				B: 10,
				A: 255,
			})
		}
	}

	opts := Options{MinSize: 8, MaxSize: 16, Padding: 2, Extrude: 2}
	layout, err := Plan([]Item{{ID: "sprite", Width: 2, Height: 3}}, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Layout
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	got, err := Render(&decoded, []Source{{ID: "sprite", Image: source}})
	if err != nil {
		t.Fatalf("Render round-tripped layout: %v", err)
	}

	packed, err := Pack([]Sprite{{ID: "sprite", Image: source}}, opts)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if packed.Layout.Render == nil || *packed.Layout.Render != *layout.Render {
		t.Fatalf("Pack render settings = %+v, want %+v", packed.Layout.Render, layout.Render)
	}
	if !bytes.Equal(got.(*image.RGBA).Pix, packed.Image.(*image.RGBA).Pix) {
		t.Fatal("JSON round-trip render differs from Pack image")
	}
}

func TestRenderExtrusionKeepsAdjacentSpritesDisjoint(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "red", Width: 3, Height: 2},
		{ID: "green", Width: 3, Height: 2},
	}
	opts := Options{MinSize: 8, MaxSize: 32, Padding: 2, Extrude: 2}
	layout, err := Plan(items, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	redImage := image.NewRGBA(image.Rect(0, 0, 3, 2))
	greenImage := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			redImage.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			greenImage.SetRGBA(x, y, color.RGBA{G: 255, A: 255})
		}
	}
	sources := []Source{
		{ID: "red", Image: redImage},
		{ID: "green", Image: greenImage},
	}
	got, err := Render(layout, sources)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	atlas := got.(*image.RGBA)
	reserved := make([]image.Rectangle, len(layout.Placements))

	for i, placement := range layout.Placements {
		width, height := placement.Width, placement.Height
		if placement.Rotated {
			width, height = height, width
		}
		reserved[i] = image.Rect(
			placement.X-opts.Padding,
			placement.Y-opts.Padding,
			placement.X+width+opts.Padding,
			placement.Y+height+opts.Padding,
		)
		want := color.RGBA{R: 255, A: 255}
		if placement.ID == "green" {
			want = color.RGBA{G: 255, A: 255}
		}
		for y := placement.Y; y < placement.Y+height; y++ {
			for x := placement.X; x < placement.X+width; x++ {
				if pixel := atlas.RGBAAt(x, y); pixel != want {
					t.Fatalf("placement %q pixel at (%d,%d) = %+v, want %+v", placement.ID, x, y, pixel, want)
				}
			}
		}
	}

	if reserved[0].Overlaps(reserved[1]) {
		t.Fatalf("reserved regions overlap: %v and %v", reserved[0], reserved[1])
	}
}

// makeRenderExtrusionTestInputs covers source storage and coordinate models.
func makeRenderExtrusionTestInputs() []struct {
	name  string
	image image.Image
} {
	bounds := image.Rect(-3, 5, -1, 8)
	rgba := image.NewRGBA(bounds)
	nrgba := image.NewNRGBA(bounds)
	parent := image.NewRGBA(image.Rect(-5, 3, 3, 11))
	subimage := parent.SubImage(bounds).(*image.RGBA)
	inputs := []struct {
		name  string
		image image.Image
	}{
		{name: "RGBA", image: rgba},
		{name: "NRGBA with hidden transparent RGB", image: nrgba},
		{name: "RGBA subimage with offset bounds", image: subimage},
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			alpha := byte((96 + (x-bounds.Min.X)*30 + (y-bounds.Min.Y)*20) & 0xff)
			pixel := color.RGBA{R: alpha / 2, G: alpha / 3, B: alpha / 4, A: alpha}
			rgba.SetRGBA(x, y, pixel)
			subimage.SetRGBA(x, y, pixel)
			nrgbaPixel := color.NRGBA{R: 210, G: 70, B: 25, A: alpha}
			if x == bounds.Min.X && y == bounds.Min.Y {
				nrgbaPixel.A = 0
			}
			nrgba.SetNRGBA(x, y, nrgbaPixel)
		}
	}

	return inputs
}

func TestRenderDuplicateSourceID(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  32,
		Height: 32,
	}
	sources := []Source{
		{ID: "dup", Image: image.NewRGBA(image.Rect(0, 0, 8, 8))},
		{ID: "dup", Image: image.NewRGBA(image.Rect(0, 0, 8, 8))},
	}

	_, err := Render(layout, sources)
	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("error %v, want ErrInvalidItem", err)
	}
}

func TestRenderRotatedPlacement(t *testing.T) {
	t.Parallel()

	layout := &Layout{
		Width:  4,
		Height: 4,
		Placements: []Placement{
			{
				ID:      "a",
				X:       1,
				Y:       1,
				Width:   2,
				Height:  1,
				Rotated: true,
			},
		},
	}

	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, image.White)
	src.Set(1, 0, image.Black)

	atlas, err := Render(layout, []Source{{ID: "a", Image: src}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	rgba, ok := atlas.(*image.RGBA)
	if !ok {
		t.Fatalf("atlas type %T, want *image.RGBA", atlas)
	}

	// Original row: [white, black], rotated clockwise into column:
	// top pixel becomes white at (1,1), bottom pixel becomes black at (1,2).
	white := rgba.RGBAAt(1, 1)
	black := rgba.RGBAAt(1, 2)
	if white.A == 0 || white.R == 0 {
		t.Fatalf("top rotated pixel mismatch: %+v", white)
	}
	if black.A == 0 || black.R != 0 {
		t.Fatalf("bottom rotated pixel mismatch: %+v", black)
	}
}
