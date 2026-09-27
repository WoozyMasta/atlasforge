// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
)

func ExamplePack() {
	sprites := []Sprite{
		{ID: "a", Image: image.NewRGBA(image.Rect(0, 0, 2, 2))},
		{ID: "b", Image: image.NewRGBA(image.Rect(0, 0, 3, 2))},
	}

	opts := Options{
		MinSize:     8,
		MaxSize:     8,
		Padding:     0,
		Heuristic:   HeuristicBestShortSideFit,
		AllowRotate: false,
	}

	atlas, err := Pack(sprites, opts)
	if err != nil {
		fmt.Println("pack error")
		return
	}

	fmt.Printf(
		"%dx%d %d\n",
		atlas.Layout.Width,
		atlas.Layout.Height,
		len(atlas.Layout.Placements),
	)

	// Output:
	// 8x8 2
}

func ExampleRenderWithLoader() {
	layout := &Layout{
		Width:  2,
		Height: 2,
		Placements: []Placement{
			{ID: "icon", Width: 2, Height: 2},
		},
	}

	img, err := RenderWithLoader(layout, func(id string) (image.Image, error) {
		if id != "icon" {
			return nil, fmt.Errorf("unknown source %q", id)
		}

		return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
	})
	if err != nil {
		fmt.Println("render error")
		return
	}

	fmt.Printf("%dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())

	// Output:
	// 2x2
}
