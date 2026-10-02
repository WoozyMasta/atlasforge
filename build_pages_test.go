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
	"strings"
	"testing"
)

func TestBuildPagesEmitsZeroOneTwoAndTwelvePages(t *testing.T) {
	t.Parallel()

	for _, itemCount := range []int{0, 1, 2, 12} {
		t.Run(fmt.Sprintf("items=%d", itemCount), func(t *testing.T) {
			t.Parallel()

			items := buildPagesItems(itemCount)
			opts := Options{MinSize: 1, MaxSize: 1, ForceSquare: true}
			loaded := make([]string, 0, itemCount)
			consumed := make([]int, 0, itemCount)
			seen := make(map[string]struct{}, itemCount)

			err := BuildPages(
				items,
				opts,
				func(id string) (image.Image, error) {
					loaded = append(loaded, id)
					return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
				},
				func(pageIndex int, atlas *Atlas) error {
					if pageIndex != len(consumed) {
						return fmt.Errorf("page index = %d, want %d", pageIndex, len(consumed))
					}
					if atlas == nil || atlas.Image == nil {
						return errors.New("consumer received nil atlas image")
					}
					if len(atlas.Layout.Placements) != 1 {
						return fmt.Errorf("page %d has %d placements, want 1",
							pageIndex, len(atlas.Layout.Placements))
					}

					id := atlas.Layout.Placements[0].ID
					if len(loaded) != len(consumed)+1 || loaded[len(loaded)-1] != id {
						return fmt.Errorf("page %d was not rendered immediately before consume", pageIndex)
					}
					if _, exists := seen[id]; exists {
						return fmt.Errorf("item %q was consumed more than once", id)
					}
					seen[id] = struct{}{}
					consumed = append(consumed, pageIndex)

					return nil
				},
			)
			if err != nil {
				t.Fatalf("BuildPages: %v", err)
			}
			if len(consumed) != itemCount || len(loaded) != itemCount || len(seen) != itemCount {
				t.Fatalf("consumed=%d loaded=%d unique=%d, want %d",
					len(consumed), len(loaded), len(seen), itemCount)
			}
		})
	}
}

func TestBuildPagesStopsAfterConsumerError(t *testing.T) {
	t.Parallel()

	items := buildPagesItems(4)
	consumerErr := errors.New("write failed")
	loaded := 0
	consumed := 0
	err := BuildPages(
		items,
		Options{MinSize: 1, MaxSize: 1, ForceSquare: true},
		func(string) (image.Image, error) {
			loaded++
			return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
		},
		func(pageIndex int, _ *Atlas) error {
			consumed++
			if pageIndex == 1 {
				return consumerErr
			}

			return nil
		},
	)

	if !errors.Is(err, consumerErr) || !strings.Contains(err.Error(), "page 1") {
		t.Fatalf("error = %v, want page 1 wrapping consumer error", err)
	}
	if loaded != 2 || consumed != 2 {
		t.Fatalf("loaded=%d consumed=%d, want 2 each", loaded, consumed)
	}
}

func TestBuildPagesMatchesPackPagesLayoutsAndPixels(t *testing.T) {
	t.Parallel()

	items := buildPagesItems(8)
	for i := range items {
		items[i].Width = 2
		items[i].Height = 2
	}
	sources := make(map[string]image.Image, len(items))
	sprites := make([]Sprite, 0, len(items))
	for i, item := range items {
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		colors := [...]color.RGBA{
			{R: 20, A: 255},
			{G: 80, A: 255},
			{B: 90, A: 255},
			{R: 40, G: 80, B: 90, A: 255},
		}
		for y := range 2 {
			for x := range 2 {
				img.SetRGBA(x, y, colors[(i+x+y)%len(colors)])
			}
		}
		sources[item.ID] = img
		sprites = append(sprites, Sprite{ID: item.ID, Image: img})
	}
	opts := Options{
		MinSize: 4, MaxSize: 4, ForceSquare: true,
		Heuristic: HeuristicFirstFit,
	}

	want, err := PackPages(sprites, opts)
	if err != nil {
		t.Fatalf("PackPages: %v", err)
	}
	var pageIndex int
	err = BuildPages(items, opts, func(id string) (image.Image, error) {
		return sources[id], nil
	}, func(index int, atlas *Atlas) error {
		if index != pageIndex {
			return fmt.Errorf("page index = %d, want %d", index, pageIndex)
		}
		if index >= len(want) {
			return fmt.Errorf("unexpected page index %d", index)
		}
		if !reflect.DeepEqual(atlas.Layout, want[index].Layout) {
			return fmt.Errorf("page %d layout differs from PackPages", index)
		}
		if !bytes.Equal(atlas.Image.(*image.RGBA).Pix, want[index].Image.(*image.RGBA).Pix) {
			return fmt.Errorf("page %d pixels differ from PackPages", index)
		}
		pageIndex++

		return nil
	})
	if err != nil {
		t.Fatalf("BuildPages: %v", err)
	}
	if pageIndex != len(want) {
		t.Fatalf("consumed %d pages, want %d", pageIndex, len(want))
	}
}

func TestBuildPagesStopsOnLoaderErrorOrSourceSizeMismatch(t *testing.T) {
	t.Parallel()

	loaderErr := errors.New("decode failed")
	tests := []struct {
		name string
		load ImageLoader
		want error
	}{
		{
			name: "loader error",
			load: func(_ string) (image.Image, error) {
				return nil, loaderErr
			},
			want: loaderErr,
		},
		{
			name: "source size mismatch",
			load: func(_ string) (image.Image, error) {
				return image.NewRGBA(image.Rect(0, 0, 2, 1)), nil
			},
			want: ErrSourceSizeMismatch,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			loaded := 0
			consumed := 0
			err := BuildPages(
				buildPagesItems(4),
				Options{MinSize: 1, MaxSize: 1, ForceSquare: true},
				func(id string) (image.Image, error) {
					loaded++
					if loaded == 1 {
						return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
					}
					if test.name == "loader error" {
						return nil, fmt.Errorf("%w for %s", test.want, id)
					}

					return test.load(id)
				},
				func(int, *Atlas) error {
					consumed++
					return nil
				},
			)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), "page 1") {
				t.Fatalf("error = %v, want page 1 wrapping %v", err, test.want)
			}
			if loaded != 2 || consumed != 1 {
				t.Fatalf("loaded=%d consumed=%d, want 2 and 1", loaded, consumed)
			}
		})
	}
}

func TestBuildPagesRejectsNilCallbacksBeforePlanning(t *testing.T) {
	t.Parallel()

	validConsume := func(int, *Atlas) error { return nil }
	if err := BuildPages(nil, Options{}, nil, validConsume); !errors.Is(err, ErrImageLoaderRequired) {
		t.Fatalf("nil loader error = %v, want ErrImageLoaderRequired", err)
	}
	validLoad := func(string) (image.Image, error) { return nil, nil }
	if err := BuildPages(nil, Options{}, validLoad, nil); !errors.Is(err, ErrAtlasConsumerRequired) {
		t.Fatalf("nil consumer error = %v, want ErrAtlasConsumerRequired", err)
	}

	called := false
	err := BuildPages(
		buildPagesItems(1),
		Options{MinSize: 0, MaxSize: 1},
		func(string) (image.Image, error) {
			called = true
			return nil, nil
		},
		func(int, *Atlas) error {
			called = true
			return nil
		},
	)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("invalid options error = %v, want ErrInvalidOptions", err)
	}
	if called {
		t.Fatal("loader or consumer called before option validation")
	}
}

// buildPagesItems creates one-pixel items for exact one-page-per-item tests.
func buildPagesItems(count int) []Item {
	items := make([]Item, count)
	for i := range items {
		items[i] = Item{ID: fmt.Sprintf("item-%02d", i), Width: 1, Height: 1}
	}

	return items
}
