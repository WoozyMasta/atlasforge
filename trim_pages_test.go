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
	"reflect"
	"strings"
	"testing"
)

func TestPackPagesAndBuildPagesTrimEquivalentLayoutsAndPixels(t *testing.T) {
	t.Parallel()

	for _, pageCount := range []int{0, 1, 2, 12} {
		t.Run(fmt.Sprintf("pages=%d", pageCount), func(t *testing.T) {
			t.Parallel()

			items, sprites, sources := trimPageInputs(pageCount * 4)
			opts := Options{
				MinSize:         6,
				MaxSize:         6,
				ForceSquare:     true,
				Heuristic:       HeuristicFirstFit,
				TrimTransparent: true,
			}

			packed, err := PackPages(sprites, opts)
			if err != nil {
				t.Fatalf("PackPages: %v", err)
			}
			if len(packed) != pageCount {
				t.Fatalf("PackPages returned %d pages, want %d", len(packed), pageCount)
			}

			loadCounts := make(map[string]int, len(items))
			seenIDs := make(map[string]struct{}, len(items))
			builtPages := 0
			err = BuildPages(items, opts, func(id string) (image.Image, error) {
				loadCounts[id]++
				return sources[id], nil
			}, func(pageIndex int, got *Atlas) error {
				if pageIndex != builtPages {
					return fmt.Errorf("page index = %d, want %d", pageIndex, builtPages)
				}
				if pageIndex >= len(packed) {
					return fmt.Errorf("unexpected page index %d", pageIndex)
				}
				if !reflect.DeepEqual(got.Layout, packed[pageIndex].Layout) {
					return fmt.Errorf("page %d layout differs from PackPages", pageIndex)
				}
				for _, placement := range got.Layout.Placements {
					if _, exists := seenIDs[placement.ID]; exists {
						return fmt.Errorf("ID %q appears on more than one placement", placement.ID)
					}
					seenIDs[placement.ID] = struct{}{}
				}
				if !bytes.Equal(got.Image.(*image.RGBA).Pix, packed[pageIndex].Image.(*image.RGBA).Pix) {
					return fmt.Errorf("page %d pixels differ from PackPages", pageIndex)
				}

				encoded, err := json.Marshal(got.Layout)
				if err != nil {
					return fmt.Errorf("marshal page %d layout: %w", pageIndex, err)
				}
				var decoded Layout
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					return fmt.Errorf("unmarshal page %d layout: %w", pageIndex, err)
				}
				if !reflect.DeepEqual(decoded, got.Layout) {
					return fmt.Errorf("page %d layout changed after JSON round trip", pageIndex)
				}

				builtPages++
				return nil
			})
			if err != nil {
				t.Fatalf("BuildPages: %v", err)
			}

			if builtPages != pageCount {
				t.Fatalf("BuildPages returned %d pages, want %d", builtPages, pageCount)
			}
			if len(seenIDs) != len(items) {
				t.Fatalf("BuildPages emitted %d IDs, want %d", len(seenIDs), len(items))
			}
			for _, item := range items {
				if loadCounts[item.ID] != 2 {
					t.Errorf("loader calls for %q = %d, want scan + render", item.ID, loadCounts[item.ID])
				}
			}
		})
	}
}

func TestPackPagesAndBuildPagesTrimFullyTransparentSource(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(-4, 9, 2, 14))
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 240, G: 80, B: 35})
		}
	}
	item := Item{ID: "empty", Width: 6, Height: 5}
	opts := Options{MinSize: 1, MaxSize: 1, TrimTransparent: true}

	packed, err := PackPages([]Sprite{{ID: item.ID, Image: source}}, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 1 {
		t.Fatalf("PackPages returned %d pages, want 1", len(packed))
	}

	placement := packed[0].Layout.Placements[0]
	if placement.Width != 1 || placement.Height != 1 ||
		placement.OriginalWidth != 6 || placement.OriginalHeight != 5 ||
		placement.OffsetX != 0 || placement.OffsetY != 0 {
		t.Fatalf("fully transparent placement = %+v", placement)
	}
	if got := packed[0].Image.At(placement.X, placement.Y); got != (color.RGBA{}) {
		t.Fatalf("rendered pixel = %v, want transparent zero RGBA", got)
	}

	calls := 0
	err = BuildPages([]Item{item}, opts, func(string) (image.Image, error) {
		calls++
		return source, nil
	}, func(_ int, got *Atlas) error {
		if !reflect.DeepEqual(got.Layout, packed[0].Layout) {
			return errors.New("BuildPages layout differs from PackPages")
		}
		if pixel := got.Image.At(placement.X, placement.Y); pixel != (color.RGBA{}) {
			return fmt.Errorf("rendered pixel = %v, want transparent zero RGBA", pixel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loader calls = %d, want scan + render", calls)
	}
}

func TestPackPagesAndBuildPagesTrimPreservesExtrusion(t *testing.T) {
	t.Parallel()

	source := image.NewRGBA(image.Rect(-3, 6, 1, 10))
	for y := 7; y < 9; y++ {
		for x := -2; x < 0; x++ {
			source.SetRGBA(x, y, color.RGBA{R: 210, G: 40, A: 255})
		}
	}
	items := []Item{{ID: "sprite", Width: 4, Height: 4}}
	sprites := []Sprite{{ID: "sprite", Image: source}}
	opts := Options{
		MinSize:         6,
		MaxSize:         6,
		Padding:         1,
		Extrude:         1,
		ForceSquare:     true,
		TrimTransparent: true,
	}

	packed, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	if len(packed) != 1 {
		t.Fatalf("PackPages returned %d pages, want 1", len(packed))
	}

	wantSettings := &RenderSettings{
		Padding:         1,
		Extrude:         1,
		TrimTransparent: true,
	}
	if !reflect.DeepEqual(packed[0].Layout.Render, wantSettings) {
		t.Fatalf("render settings = %+v, want %+v", packed[0].Layout.Render, wantSettings)
	}

	placement := packed[0].Layout.Placements[0]
	if got := packed[0].Image.(*image.RGBA).
		RGBAAt(placement.X-1, placement.Y-1); got != (color.RGBA{R: 210, G: 40, A: 255}) {
		t.Fatalf("extruded corner pixel = %+v", got)
	}

	err = BuildPages(items, opts, func(string) (image.Image, error) {
		return source, nil
	}, func(_ int, got *Atlas) error {
		if !reflect.DeepEqual(got.Layout, packed[0].Layout) {
			return errors.New("BuildPages layout differs from PackPages")
		}
		if !bytes.Equal(got.Image.(*image.RGBA).Pix, packed[0].Image.(*image.RGBA).Pix) {
			return errors.New("BuildPages image differs from PackPages")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
}

func TestBuildPagesTrimScanErrorsAndChangedRenderSource(t *testing.T) {
	t.Parallel()

	item := Item{ID: "sprite", Width: 5, Height: 5}
	opts := Options{MinSize: 3, MaxSize: 3, TrimTransparent: true}
	validSource := trimPageImage(true, 2)
	loaderErr := errors.New("decode failed")

	t.Run("scan loader error", func(t *testing.T) {
		t.Parallel()

		calls := 0
		consumed := 0
		err := BuildPages([]Item{item}, opts, func(string) (image.Image, error) {
			calls++
			return nil, loaderErr
		}, func(int, *Atlas) error {
			consumed++
			return nil
		})

		if !errors.Is(err, loaderErr) {
			t.Fatalf("BuildPages error = %v, want loader error", err)
		}
		if calls != 1 || consumed != 0 {
			t.Fatalf("loader calls=%d consumer calls=%d, want 1 and 0", calls, consumed)
		}
	})

	t.Run("scan source size mismatch", func(t *testing.T) {
		t.Parallel()

		calls := 0
		consumed := 0
		err := BuildPages([]Item{item}, opts, func(string) (image.Image, error) {
			calls++
			return image.NewRGBA(image.Rect(0, 0, 4, 5)), nil
		}, func(int, *Atlas) error {
			consumed++
			return nil
		})

		if !errors.Is(err, ErrSourceSizeMismatch) {
			t.Fatalf("BuildPages error = %v, want ErrSourceSizeMismatch", err)
		}
		if calls != 1 || consumed != 0 {
			t.Fatalf("loader calls=%d consumer calls=%d, want 1 and 0", calls, consumed)
		}
	})

	t.Run("source size changes before render", func(t *testing.T) {
		t.Parallel()

		secondSource := image.NewNRGBA(image.Rect(2, -3, 6, 2))
		for y := secondSource.Bounds().Min.Y + 1; y < secondSource.Bounds().Min.Y+4; y++ {
			for x := secondSource.Bounds().Min.X; x < secondSource.Bounds().Min.X+2; x++ {
				secondSource.SetNRGBA(x, y, color.NRGBA{G: 180, A: 255})
			}
		}
		items := []Item{
			{ID: "first", Width: 5, Height: 5},
			{ID: "second", Width: 4, Height: 5},
		}
		sources := map[string]image.Image{"first": validSource, "second": secondSource}
		calls := make(map[string]int, len(items))
		consumed := 0
		err := BuildPages(items, opts, func(id string) (image.Image, error) {
			calls[id]++
			if id == "second" && calls[id] == 2 {
				return image.NewRGBA(image.Rect(0, 0, 3, 5)), nil
			}
			return sources[id], nil
		}, func(pageIndex int, _ *Atlas) error {
			if pageIndex != 0 {
				return fmt.Errorf("unexpected consumed page %d", pageIndex)
			}
			consumed++
			return nil
		})
		if !errors.Is(err, ErrSourceSizeMismatch) ||
			!strings.Contains(err.Error(), "page 1") {
			t.Fatalf("BuildPages error = %v, want page 1 ErrSourceSizeMismatch", err)
		}
		if calls["first"] != 2 || calls["second"] != 2 || consumed != 1 {
			t.Fatalf("loader calls=%v consumer calls=%d, want two per ID and one page", calls, consumed)
		}
	})
}

func TestBuildPagesTrimDisabledLoadsEachSourceOnce(t *testing.T) {
	t.Parallel()

	items := buildPagesItems(2)
	calls := 0
	err := BuildPages(items, Options{MinSize: 1, MaxSize: 1, ForceSquare: true},
		func(string) (image.Image, error) {
			calls++
			return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
		}, func(int, *Atlas) error { return nil })
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if calls != len(items) {
		t.Fatalf("loader calls = %d, want %d without trim scan", calls, len(items))
	}
}

// trimPageInputs creates mixed cropped and untouched squares for page tests.
func trimPageInputs(count int) ([]Item, []Sprite, map[string]image.Image) {
	items := make([]Item, count)
	sprites := make([]Sprite, count)
	sources := make(map[string]image.Image, count)
	for i := range count {
		id := fmt.Sprintf("sprite-%03d", i)
		trim := i%2 == 0
		img := trimPageImage(trim, i)
		bounds := img.Bounds()
		items[i] = Item{ID: id, Width: bounds.Dx(), Height: bounds.Dy()}
		sprites[i] = Sprite{ID: id, Image: img}
		sources[id] = img
	}

	return items, sprites, sources
}

// trimPageImage builds a unique source whose visible 3x3 region is optionally cropped.
func trimPageImage(trim bool, index int) *image.NRGBA {
	bounds := image.Rect(-7, 11, -4, 14)
	if trim {
		bounds = image.Rect(-7, 11, -2, 16)
	}
	img := image.NewNRGBA(bounds)
	color := color.NRGBA{R: 71, G: 90, B: 150, A: 255}
	content := bounds
	if trim {
		offsetX := index % 3
		offsetY := index / 3 % 3
		content = image.Rect(bounds.Min.X+offsetX, bounds.Min.Y+offsetY,
			bounds.Min.X+offsetX+3, bounds.Min.Y+offsetY+3)
	}
	for y := content.Min.Y; y < content.Max.Y; y++ {
		for x := content.Min.X; x < content.Max.X; x++ {
			img.SetNRGBA(x, y, color)
		}
	}

	return img
}
