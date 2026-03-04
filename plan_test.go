package atlasforge

import (
	"errors"
	"image"
	"testing"
)

func TestPlanValidateOptions(t *testing.T) {
	t.Parallel()

	items := []Item{{ID: "a", Width: 8, Height: 8}}

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name: "ok",
			opts: Options{
				MinSize:   16,
				MaxSize:   64,
				Padding:   1,
				Heuristic: HeuristicBestShortSideFit,
			},
		},
		{
			name: "invalid-min-max",
			opts: Options{
				MinSize: 64,
				MaxSize: 16,
				Padding: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid-padding",
			opts: Options{
				MinSize: 16,
				MaxSize: 64,
				Padding: -1,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Plan(items, tc.opts)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPlanRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	opts := Options{MinSize: 16, MaxSize: 64}
	items := []Item{
		{ID: "dup", Width: 8, Height: 8},
		{ID: "dup", Width: 4, Height: 4},
	}
	_, err := Plan(items, opts)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("error %v, want ErrInvalidItem", err)
	}
}

func TestPlanEmptyItemsReturnsMinSizeAtlas(t *testing.T) {
	t.Parallel()

	opts := Options{MinSize: 32, MaxSize: 64}
	layout, err := Plan(nil, opts)
	if err != nil {
		t.Fatalf("Plan(nil): %v", err)
	}
	if layout.Width != 32 || layout.Height != 32 {
		t.Fatalf("empty plan size = %dx%d, want 32x32", layout.Width, layout.Height)
	}
}

func TestPlanPlacementsWithinBoundsNoOverlap(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 10, Height: 12},
		{ID: "b", Width: 8, Height: 8},
		{ID: "c", Width: 5, Height: 14},
	}
	opts := Options{
		MinSize:   32,
		MaxSize:   64,
		Padding:   1,
		Heuristic: HeuristicBottomLeft,
	}

	layout, err := Plan(items, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(layout.Placements) != len(items) {
		t.Fatalf("placements=%d, want %d", len(layout.Placements), len(items))
	}

	for i := range layout.Placements {
		p := layout.Placements[i]
		if p.X < 0 || p.Y < 0 || p.X+p.Width > layout.Width || p.Y+p.Height > layout.Height {
			t.Fatalf("placement %q out of bounds: %+v atlas=%dx%d", p.ID, p, layout.Width, layout.Height)
		}
	}

	for i := 0; i < len(layout.Placements); i++ {
		for j := i + 1; j < len(layout.Placements); j++ {
			a := layout.Placements[i]
			b := layout.Placements[j]
			if overlaps(a.X, a.Y, a.Width, a.Height, b.X, b.Y, b.Width, b.Height) {
				t.Fatalf("placements overlap: %q and %q", a.ID, b.ID)
			}
		}
	}
}

func TestPlanAllHeuristics(t *testing.T) {
	t.Parallel()

	heuristics := []Heuristic{
		HeuristicBestShortSideFit,
		HeuristicBestLongSideFit,
		HeuristicBestAreaFit,
		HeuristicBottomLeft,
		HeuristicContactPoint,
		HeuristicFirstFit,
	}
	items := []Item{
		{ID: "a", Width: 10, Height: 12},
		{ID: "b", Width: 8, Height: 8},
		{ID: "c", Width: 5, Height: 14},
		{ID: "d", Width: 11, Height: 7},
	}

	for _, heuristic := range heuristics {
		heuristic := heuristic
		t.Run(heuristicName(heuristic), func(t *testing.T) {
			t.Parallel()

			opts := Options{
				MinSize:   32,
				MaxSize:   64,
				Padding:   1,
				Heuristic: heuristic,
			}
			layout, err := Plan(items, opts)
			if err != nil {
				t.Fatalf("Plan(%s): %v", heuristicName(heuristic), err)
			}
			if len(layout.Placements) != len(items) {
				t.Fatalf("placements=%d, want %d", len(layout.Placements), len(items))
			}
		})
	}
}

func TestPlanInvalidHeuristicFallsBack(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 10, Height: 12},
		{ID: "b", Width: 8, Height: 8},
	}
	opts := Options{
		MinSize:   32,
		MaxSize:   64,
		Padding:   1,
		Heuristic: Heuristic(999),
	}
	layout, err := Plan(items, opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(layout.Placements) != len(items) {
		t.Fatalf("placements=%d, want %d", len(layout.Placements), len(items))
	}
}

func TestPlanExceedsMaxSize(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "huge", Width: 512, Height: 512},
	}
	opts := Options{
		MinSize:   64,
		MaxSize:   256,
		Padding:   0,
		Heuristic: HeuristicBestShortSideFit,
	}

	_, err := Plan(items, opts)
	if err == nil {
		t.Fatal("expected error when required atlas exceeds MaxSize")
	}
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error %v, want ErrInvalidOptions", err)
	}
}

func TestPlanRejectsItemPaddedSizeExceedsMax(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 250, Height: 32},
	}
	opts := Options{
		MinSize: 64,
		MaxSize: 256,
		Padding: 10,
	}

	_, err := Plan(items, opts)
	if err == nil {
		t.Fatal("expected error when padded item exceeds MaxSize")
	}
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error %v, want ErrInvalidOptions", err)
	}
}

func TestPlanRejectsTotalPaddedAreaExceedsMaxArea(t *testing.T) {
	t.Parallel()

	items := []Item{
		{ID: "a", Width: 40, Height: 40},
		{ID: "b", Width: 40, Height: 40},
		{ID: "c", Width: 40, Height: 40},
	}
	opts := Options{
		MinSize: 32,
		MaxSize: 64,
		Padding: 0,
	}

	_, err := Plan(items, opts)
	if err == nil {
		t.Fatal("expected error when total area exceeds max atlas area")
	}
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error %v, want ErrInvalidOptions", err)
	}
}

func TestPackSpriteSizeOverrideMismatch(t *testing.T) {
	t.Parallel()

	sprites := []Sprite{
		{
			ID:     "a",
			Image:  solid(10, 12),
			Width:  9,
			Height: 12,
		},
	}
	opts := Options{
		MinSize:   32,
		MaxSize:   64,
		Padding:   1,
		Heuristic: HeuristicBestShortSideFit,
	}
	_, err := Pack(sprites, opts)
	if err == nil {
		t.Fatal("expected size mismatch error")
	}
	if !errors.Is(err, ErrSourceSizeMismatch) {
		t.Fatalf("error %v, want ErrSourceSizeMismatch", err)
	}
}

func TestPackDuplicateSpriteID(t *testing.T) {
	t.Parallel()

	sprites := []Sprite{
		{ID: "dup", Image: solid(4, 4)},
		{ID: "dup", Image: solid(4, 4)},
	}
	opts := Options{
		MinSize:   16,
		MaxSize:   64,
		Padding:   0,
		Heuristic: HeuristicBestShortSideFit,
	}
	_, err := Pack(sprites, opts)
	if err == nil {
		t.Fatal("expected duplicate sprite id error")
	}
	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("error %v, want ErrInvalidItem", err)
	}
}

func TestPack(t *testing.T) {
	t.Parallel()

	sprites := []Sprite{
		{ID: "a", Image: solid(10, 12)},
		{ID: "b", Image: solid(8, 8)},
	}
	opts := Options{
		MinSize:   32,
		MaxSize:   64,
		Padding:   1,
		Heuristic: HeuristicBottomLeft,
	}
	atlas, err := Pack(sprites, opts)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if atlas.Image == nil {
		t.Fatal("expected atlas image")
	}
	if len(atlas.Layout.Placements) != len(sprites) {
		t.Fatalf("placements=%d, want %d", len(atlas.Layout.Placements), len(sprites))
	}
}

// solid returns an opaque RGBA image of the requested size.
func solid(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

// overlaps reports whether two axis-aligned rectangles intersect.
func overlaps(ax, ay, aw, ah, bx, by, bw, bh int) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}

// heuristicName returns a stable short name for test and benchmark labels.
func heuristicName(heuristic Heuristic) string {
	switch heuristic {
	case HeuristicBestShortSideFit:
		return "bssf"
	case HeuristicBestLongSideFit:
		return "blsf"
	case HeuristicBestAreaFit:
		return "baf"
	case HeuristicBottomLeft:
		return "bl"
	case HeuristicContactPoint:
		return "cp"
	case HeuristicFirstFit:
		return "ff"
	default:
		return "unknown"
	}
}
