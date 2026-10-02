// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "image"

const (
	// HeuristicBestShortSideFit minimizes the smaller leftover edge first.
	HeuristicBestShortSideFit Heuristic = iota
	// HeuristicBestLongSideFit minimizes the larger leftover edge first.
	HeuristicBestLongSideFit
	// HeuristicBestAreaFit minimizes wasted free rectangle area first.
	HeuristicBestAreaFit
	// HeuristicBottomLeft prefers lower Y, then smaller X placements.
	HeuristicBottomLeft
	// HeuristicContactPoint maximizes contact with borders/used rectangles.
	HeuristicContactPoint
	// HeuristicFirstFit places into the first matching free rectangle.
	// It favors planning speed over packing density.
	HeuristicFirstFit
)

// Heuristic is the MaxRects scoring heuristic used for placement.
type Heuristic int

// ImageLoader loads one image by placement ID during rendering.
// The renderer does not retain images after drawing each placement.
type ImageLoader func(id string) (image.Image, error)

// AtlasConsumer receives one rendered page synchronously.
// Page indexes start at zero and increase in output order.
type AtlasConsumer func(pageIndex int, atlas *Atlas) error

// Sprite is a high-level input used by Pack.
type Sprite struct {
	// Image is the source pixels used by Pack.
	// It is excluded from json/yaml serialization.
	Image image.Image `json:"-" yaml:"-"`

	// ID is a unique stable identifier propagated into Placement.ID.
	ID string `json:"id" yaml:"id"`

	// Width overrides Image bounds when > 0.
	// Use with care: mismatch with Image bounds causes render errors.
	Width int `json:"width,omitempty" yaml:"width,omitempty"`

	// Height overrides Image bounds when > 0.
	// Use with care: mismatch with Image bounds causes render errors.
	Height int `json:"height,omitempty" yaml:"height,omitempty"`
}

// Options controls atlas planning behavior.
type Options struct {
	// MinSize is the lower bound for each atlas side candidate.
	// Plan and compacted PlanPages layouts do not produce smaller dimensions.
	MinSize int `json:"min_size" yaml:"min_size"`

	// MaxSize is the upper bound for each atlas side candidate.
	// Plan/Pack fail when items require a larger side;
	// page APIs fail only when an individual item cannot fit.
	// Its square must fit in int for planner area calculations.
	MaxSize int `json:"max_size" yaml:"max_size"`

	// Padding reserves empty pixels around each placed item.
	// Higher values reduce packing density and increase atlas size.
	Padding int `json:"padding" yaml:"padding"`

	// Extrude copies edge pixels into the reserved padding around each item.
	// It must be between zero and Padding.
	Extrude int `json:"extrude,omitempty" yaml:"extrude,omitempty"`

	// AspectPenalty adds a penalty for non-square atlas shapes.
	// Use 0 to disable shape bias and prioritize fit only.
	AspectPenalty float64 `json:"aspect_penalty" yaml:"aspect_penalty"`

	// Heuristic selects MaxRects placement scoring policy.
	// It affects placement order quality and final atlas utilization.
	Heuristic Heuristic `json:"heuristic" yaml:"heuristic"`

	// PreferHeight controls tie-breaking for equally scored candidates.
	// When true, ties prefer taller atlases over wider ones.
	PreferHeight bool `json:"prefer_height" yaml:"prefer_height"`

	// ForceSquare limits size search to square candidates only (w == h).
	// Rectangular candidates are skipped even when they have lower area.
	ForceSquare bool `json:"force_square" yaml:"force_square"`

	// AllowRotate enables 90-degree clockwise item rotation.
	// Rotated placements are reported via Placement.Rotated.
	AllowRotate bool `json:"allow_rotate" yaml:"allow_rotate"`
}

// Atlas combines rendered image and the layout used to produce it.
type Atlas struct {
	// Image is the rendered atlas pixels.
	Image image.Image `json:"-" yaml:"-"`

	// Layout describes where each source item was placed in Image.
	Layout Layout `json:"layout" yaml:"layout"`
}

// Layout contains atlas dimensions and all placements.
type Layout struct {
	// Render stores optional settings needed to reproduce rendered pixels.
	Render *RenderSettings `json:"render,omitempty" yaml:"render,omitempty"`

	// Placements contains one entry per input item.
	// Order follows internal packing workflow, not input order guarantees.
	Placements []Placement `json:"placements" yaml:"placements"`

	// Width is the final atlas width in pixels.
	Width int `json:"width" yaml:"width"`

	// Height is the final atlas height in pixels.
	Height int `json:"height" yaml:"height"`
}

// RenderSettings contains pixel operations recorded by Plan for Render.
type RenderSettings struct {
	// Padding is the reserved margin around placements.
	Padding int `json:"padding,omitempty" yaml:"padding,omitempty"`

	// Extrude is the number of edge-pixel layers copied into the padding.
	Extrude int `json:"extrude,omitempty" yaml:"extrude,omitempty"`

	// TrimTransparent records whether source bounds were trimmed.
	TrimTransparent bool `json:"trim_transparent,omitempty" yaml:"trim_transparent,omitempty"`
}

// Placement describes final coordinates of one packed item.
type Placement struct {
	// ID matches Item.ID / Sprite.ID used for this placement.
	ID string `json:"id" yaml:"id"`

	// X is the top-left X coordinate in atlas pixels.
	X int `json:"x" yaml:"x"`

	// Y is the top-left Y coordinate in atlas pixels.
	Y int `json:"y" yaml:"y"`

	// Width is the original source width before rotation.
	Width int `json:"width" yaml:"width"`

	// Height is the original source height before rotation.
	Height int `json:"height" yaml:"height"`

	// Rotated reports clockwise 90-degree placement during rendering.
	Rotated bool `json:"rotated" yaml:"rotated"`
}

// Item describes a rectangular payload for layout planning.
type Item struct {
	// ID is a unique stable identifier used in resulting Placement.ID.
	ID string `json:"id" yaml:"id"`

	// Width is the payload width in pixels and must be > 0.
	Width int `json:"width" yaml:"width"`

	// Height is the payload height in pixels and must be > 0.
	Height int `json:"height" yaml:"height"`
}

// Source binds an image payload to an item ID for rendering.
type Source struct {
	// Image provides pixels for rendering.
	// Bounds must match Placement width/height for the same ID.
	Image image.Image `json:"-" yaml:"-"`

	// ID must match Placement.ID from a planned layout.
	ID string `json:"id" yaml:"id"`
}

// DefaultOptions returns practical defaults for UI/sprite atlas packing.
func DefaultOptions() Options {
	return Options{
		MinSize:       256,
		MaxSize:       4096,
		Padding:       0,
		AspectPenalty: 0.25,
		Heuristic:     HeuristicBestShortSideFit,
	}
}
