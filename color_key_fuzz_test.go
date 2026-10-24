// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"encoding/json"
	"image"
	"image/color"
	"testing"
)

// FuzzColorKey checks packing, serialized rendering, and lazy rendering parity.
func FuzzColorKey(f *testing.F) {
	f.Add([]byte{255, 0, 255, 255, 20, 40, 60, 255}, uint8(255), uint8(0), uint8(255), uint8(255), uint8(0x13))
	f.Add([]byte{1, 2, 3, 0, 255, 255, 255, 255}, uint8(255), uint8(255), uint8(255), uint8(255), uint8(0x54))
	f.Add([]byte{}, uint8(0), uint8(0), uint8(0), uint8(0), uint8(0x20))

	f.Fuzz(func(t *testing.T, pixels []byte, keyR, keyG, keyB, keyA, flags uint8) {
		width := 1 + int(flags&3)
		height := 1 + int(flags>>2&3)
		bounds := image.Rect(-3, 5, -3+width, 5+height)
		key := color.NRGBA{R: keyR, G: keyG, B: keyB, A: keyA}

		var source image.Image
		if flags&0x08 == 0 {
			img := image.NewNRGBA(bounds)
			fillColorKeyNRGBA(img, pixels)
			source = img
		} else {
			img := image.NewRGBA(bounds)
			fillColorKeyRGBA(img, pixels)
			source = img
		}

		trim := flags&0x10 != 0
		toAlpha := flags&0x20 != 0
		if !trim && !toAlpha {
			toAlpha = true
		}
		options := Options{
			MinSize:         1,
			MaxSize:         8,
			TrimTransparent: flags&0x40 != 0,
			ColorKey:        &ColorKeyOptions{Color: key, Trim: trim, ToAlpha: toAlpha},
		}

		atlas, err := Pack([]Sprite{{ID: "fuzz", Image: source}}, options)
		if err != nil {
			t.Fatalf("Pack: %v", err)
		}

		encoded, err := json.Marshal(atlas.Layout)
		if err != nil {
			t.Fatalf("marshal layout: %v", err)
		}

		var layout Layout
		err = json.Unmarshal(encoded, &layout)
		if err != nil {
			t.Fatalf("unmarshal layout: %v", err)
		}

		sources := []Source{{ID: "fuzz", Image: source}}
		eager, err := Render(&layout, sources)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}

		lazy, err := RenderWithLoader(
			&layout, func(string) (image.Image, error) {
				return source, nil
			},
		)
		if err != nil {
			t.Fatalf("RenderWithLoader: %v", err)
		}
		if !imagesEqual(atlas.Image, eager) || !imagesEqual(atlas.Image, lazy) {
			t.Fatal("Pack, Render, and RenderWithLoader outputs differ")
		}

		placement := layout.Placements[0]
		if placement.OriginalWidth != 0 {
			if placement.OriginalWidth != width || placement.OriginalHeight != height ||
				placement.OffsetX+placement.Width > width || placement.OffsetY+placement.Height > height {
				t.Fatalf("crop metadata is outside source bounds: %+v", placement)
			}
		}
	})
}

// fillColorKeyNRGBA fills an NRGBA source from a reusable byte sequence.
func fillColorKeyNRGBA(img *image.NRGBA, pixels []byte) {
	index := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			var components [4]uint8
			for i := range components {
				if len(pixels) != 0 {
					components[i] = pixels[index%len(pixels)]
				}
				index++
			}

			img.SetNRGBA(x, y, color.NRGBA{
				R: components[0],
				G: components[1],
				B: components[2],
				A: components[3],
			})
		}
	}
}

// fillColorKeyRGBA fills an RGBA source from canonicalized NRGBA bytes.
func fillColorKeyRGBA(img *image.RGBA, pixels []byte) {
	index := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			var components [4]uint8
			for i := range components {
				if len(pixels) != 0 {
					components[i] = pixels[index%len(pixels)]
				}
				index++
			}

			img.SetRGBA(x, y, canonicalRGBA8(color.NRGBA{
				R: components[0],
				G: components[1],
				B: components[2],
				A: components[3],
			}))
		}
	}
}
