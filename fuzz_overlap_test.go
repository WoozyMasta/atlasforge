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

func FuzzOverlapZones(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6}, uint8(4), uint8(2), uint8(2), true)
	f.Add([]byte{0, 255, 17}, uint8(7), uint8(4), uint8(1), false)
	f.Add([]byte{}, uint8(2), uint8(1), uint8(0), true)

	f.Fuzz(func(t *testing.T, pixels []byte, countSeed, sizeSeed, depthSeed uint8, allowRotate bool) {
		count := int(countSeed%15) + 2
		size := int(sizeSeed%31) + 2
		depth := int(depthSeed)%min(size/2, 3) + 1
		edge := color.RGBA{R: 31, G: 63, B: 127, A: 191}
		sprites := make([]Sprite, count)
		for i := range sprites {
			img := image.NewRGBA(image.Rect(0, 0, size, size))
			for y := range size {
				for x := range size {
					pixel := edge
					if x >= depth && x < size-depth && y >= depth && y < size-depth {
						index := (i*size*size + y*size + x) * 4
						pixel = fuzzOverlapPixel(pixels, index)
					}
					img.SetRGBA(x, y, pixel)
				}
			}
			sprites[i] = Sprite{ID: "sprite-" + itoa(i), Image: img}
		}
		deduplicate := len(pixels) > 0 && pixels[0]&1 != 0
		if deduplicate {
			sprites[len(sprites)-1].Image = sprites[0].Image
		}

		options := overlapTestOptions()
		options.MinSize = 8
		options.MaxSize = 256
		options.Padding = int(depthSeed) % (depth + 1)
		options.AllowRotate = allowRotate
		options.Deduplicate = deduplicate
		atlas, err := Pack(sprites, options)
		if err != nil {
			t.Fatalf("Pack: %v", err)
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

		sources := make([]Source, len(sprites))
		for i := range sprites {
			sources[i] = Source{ID: sprites[i].ID, Image: sprites[i].Image}
		}

		rendered, err := Render(&layout, sources)
		if err != nil || !sameRGBAImage(atlas.Image, rendered) {
			t.Fatalf("Render JSON parity err=%v", err)
		}

		for index := range layout.Placements {
			if layout.Placements[index].AliasOf == "" {
				continue
			}

			layout.Placements[index].AliasOf = "missing-canonical"
			if _, err := Render(&layout, sources); err == nil {
				t.Fatal("Render accepted a mutated alias reference")
			}
			break
		}

		if len(layout.OverlapBlocks) > 0 {
			layout.OverlapBlocks[0].Width++
			if _, err := Render(&layout, sources); err == nil {
				t.Fatal("Render accepted a mutated overlap block")
			}
		}
	})
}

// fuzzOverlapPixel maps arbitrary bytes to one deterministic interior color.
func fuzzOverlapPixel(data []byte, offset int) color.RGBA {
	var pixel color.RGBA
	if len(data) == 0 {
		return pixel
	}

	pixel.R = data[offset%len(data)]
	pixel.G = data[(offset+1)%len(data)]
	pixel.B = data[(offset+2)%len(data)]
	pixel.A = data[(offset+3)%len(data)]
	if pixel == (color.RGBA{R: 31, G: 63, B: 127, A: 191}) {
		pixel.R++
	}

	return pixel
}
