// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"testing"
)

// benchmarkHeuristics returns all supported packing heuristics.
func benchmarkHeuristics() []Heuristic {
	return []Heuristic{
		HeuristicBestShortSideFit,
		HeuristicBestLongSideFit,
		HeuristicBestAreaFit,
		HeuristicBottomLeft,
		HeuristicContactPoint,
		HeuristicFirstFit,
	}
}

// benchmarkItems builds deterministic rectangular items for planning benchmarks.
func benchmarkItems(n int) []Item {
	items := make([]Item, 0, n)
	for i := range n {
		w := 8 + ((i * 7) % 47)
		h := 8 + ((i * 11) % 53)
		items = append(items, Item{
			ID:     fmt.Sprintf("item_%03d", i),
			Width:  w,
			Height: h,
		})
	}

	return items
}

// benchmarkSprites converts benchmark items into sprites with matching images.
func benchmarkSprites(items []Item) []Sprite {
	sprites := make([]Sprite, 0, len(items))
	for i := range items {
		item := items[i]
		sprites = append(sprites, Sprite{
			ID:    item.ID,
			Image: image.NewRGBA(image.Rect(0, 0, item.Width, item.Height)),
		})
	}

	return sprites
}

// benchmarkSources converts benchmark items into render sources.
func benchmarkSources(items []Item) []Source {
	sources := make([]Source, 0, len(items))
	for i := range items {
		item := items[i]
		sources = append(sources, Source{
			ID:    item.ID,
			Image: image.NewRGBA(image.Rect(0, 0, item.Width, item.Height)),
		})
	}

	return sources
}

func BenchmarkPlanHeuristics(b *testing.B) {
	items := benchmarkItems(256)
	base := DefaultOptions()
	base.MinSize = 256
	base.MaxSize = 4096
	base.Padding = 1
	base.AllowRotate = true

	for _, heuristic := range benchmarkHeuristics() {
		b.Run(heuristicName(heuristic), func(b *testing.B) {
			opts := base
			opts.Heuristic = heuristic
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				layout, err := Plan(items, opts)
				if err != nil {
					b.Fatalf("Plan(%s): %v", heuristicName(heuristic), err)
				}
				if len(layout.Placements) != len(items) {
					b.Fatalf("placements=%d, want %d", len(layout.Placements), len(items))
				}
			}
		})
	}
}

func BenchmarkRenderHeuristics(b *testing.B) {
	items := benchmarkItems(256)
	sources := benchmarkSources(items)
	base := DefaultOptions()
	base.MinSize = 256
	base.MaxSize = 4096
	base.Padding = 1
	base.AllowRotate = true

	for _, heuristic := range benchmarkHeuristics() {
		b.Run(heuristicName(heuristic), func(b *testing.B) {
			opts := base
			opts.Heuristic = heuristic
			layout, err := Plan(items, opts)
			if err != nil {
				b.Fatalf("Plan(%s): %v", heuristicName(heuristic), err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				img, err := Render(layout, sources)
				if err != nil {
					b.Fatalf("Render(%s): %v", heuristicName(heuristic), err)
				}
				if img.Bounds().Dx() != layout.Width || img.Bounds().Dy() != layout.Height {
					b.Fatalf("render size=%dx%d, want=%dx%d", img.Bounds().Dx(), img.Bounds().Dy(), layout.Width, layout.Height)
				}
			}
		})
	}
}

func BenchmarkPackHeuristics(b *testing.B) {
	items := benchmarkItems(256)
	sprites := benchmarkSprites(items)
	base := DefaultOptions()
	base.MinSize = 256
	base.MaxSize = 4096
	base.Padding = 1
	base.AllowRotate = true

	for _, heuristic := range benchmarkHeuristics() {
		b.Run(heuristicName(heuristic), func(b *testing.B) {
			opts := base
			opts.Heuristic = heuristic
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				atlas, err := Pack(sprites, opts)
				if err != nil {
					b.Fatalf("Pack(%s): %v", heuristicName(heuristic), err)
				}
				if len(atlas.Layout.Placements) != len(sprites) {
					b.Fatalf("placements=%d, want %d", len(atlas.Layout.Placements), len(sprites))
				}
			}
		})
	}
}
