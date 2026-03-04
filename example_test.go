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
