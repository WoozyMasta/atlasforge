// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"image"
	"image/color"
)

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
//
// [BuildPages] may load sources for trimming, exact pixel comparisons, and drawing.
// When [BuildPages] is used, the loader
// must return stable source dimensions and pixels across calls.
//
// The library releases each loaded image after its scan
// or draw rather than retaining the full source set.
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
	// ColorKey optionally trims exact color matches and/or converts them to alpha.
	// It is applied by image-based APIs, not geometry-only planning.
	ColorKey *ColorKeyOptions `json:"color_key,omitempty" yaml:"color_key,omitempty"`

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

	// TrimTransparent removes fully transparent pixels outside the image content.
	// It is applied by image-based packing APIs, not geometry-only planning.
	TrimTransparent bool `json:"trim_transparent,omitempty" yaml:"trim_transparent,omitempty"`

	// Deduplicate merges sprites with identical rendered pixels and records aliases.
	// It is applied by image-based packing APIs, not geometry-only planning.
	Deduplicate bool `json:"deduplicate,omitempty" yaml:"deduplicate,omitempty"`

	// Overlap packs compatible uniform-edge sprites into multi-member zones.
	// Geometry-only planning does not inspect pixels or form overlap groups.
	Overlap bool `json:"overlap,omitempty" yaml:"overlap,omitempty"`
}

// ColorKeyOptions configures exact 8-bit color matching in source images.
type ColorKeyOptions struct {
	// Color is compared after conversion to the renderer's canonical RGBA8 pixel.
	Color color.NRGBA `json:"color" yaml:"color"`

	// Trim excludes exact matches from the content bounds without changing pixels.
	Trim bool `json:"trim,omitempty" yaml:"trim,omitempty"`

	// ToAlpha replaces exact matches with canonical transparent RGBA zero.
	ToAlpha bool `json:"to_alpha,omitempty" yaml:"to_alpha,omitempty"`
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

	// OverlapBlocks describes intentional N-member pixel-sharing zones.
	OverlapBlocks []OverlapBlock `json:"overlap_blocks,omitempty" yaml:"overlap_blocks,omitempty"`

	// Width is the final atlas width in pixels.
	Width int `json:"width" yaml:"width"`

	// Height is the final atlas height in pixels.
	Height int `json:"height" yaml:"height"`
}

// OverlapBlock describes one independently placed N-member overlap zone.
type OverlapBlock struct {
	// ID links canonical placements through Placement.OverlapGroup.
	ID int `json:"id" yaml:"id"`

	// X is the physical block's atlas X coordinate after optional rotation.
	X int `json:"x" yaml:"x"`

	// Y is the physical block's atlas Y coordinate after optional rotation.
	Y int `json:"y" yaml:"y"`

	// Width is the physical block width after optional rotation.
	Width int `json:"width" yaml:"width"`

	// Height is the physical block height after optional rotation.
	Height int `json:"height" yaml:"height"`

	// ShareX is the shared width before global rotation.
	ShareX int `json:"share_x" yaml:"share_x"`

	// ShareY is the shared height before global rotation.
	ShareY int `json:"share_y" yaml:"share_y"`

	// Color is the packed canonical RGBA8 color used for empty zone pixels.
	Color uint32 `json:"color" yaml:"color"`

	// Rotated reports clockwise rotation of the entire zone.
	Rotated bool `json:"rotated,omitempty" yaml:"rotated,omitempty"`
}

// RenderSettings contains pixel operations recorded for reproducible rendering.
type RenderSettings struct {
	// ColorKey records image-based color-key operations for reproducible rendering.
	ColorKey *ColorKeyOptions `json:"color_key,omitempty" yaml:"color_key,omitempty"`

	// Padding is the reserved margin around placements.
	Padding int `json:"padding,omitempty" yaml:"padding,omitempty"`

	// Extrude is the number of edge-pixel layers copied into the padding.
	Extrude int `json:"extrude,omitempty" yaml:"extrude,omitempty"`

	// TrimTransparent records whether source bounds were trimmed.
	TrimTransparent bool `json:"trim_transparent,omitempty" yaml:"trim_transparent,omitempty"`
}

// RenderOptions contains caller-side limits that are not stored in Layout.
type RenderOptions struct {
	// MaxBytes caps the RGBA output buffer size; zero disables this limit.
	MaxBytes int64 `json:"max_bytes,omitempty" yaml:"max_bytes,omitempty"`
}

// Placement describes final coordinates of one packed item.
type Placement struct {
	// ID matches Item.ID / Sprite.ID used for this placement.
	ID string `json:"id" yaml:"id"`

	// AliasOf identifies the canonical placement sharing this physical rectangle.
	AliasOf string `json:"alias_of,omitempty" yaml:"alias_of,omitempty"`

	// X is the top-left X coordinate in atlas pixels.
	X int `json:"x" yaml:"x"`

	// Y is the top-left Y coordinate in atlas pixels.
	Y int `json:"y" yaml:"y"`

	// Width is the packed content width before rotation.
	Width int `json:"width" yaml:"width"`

	// Height is the packed content height before rotation.
	Height int `json:"height" yaml:"height"`

	// OriginalWidth is the source width before transparent trimming.
	OriginalWidth int `json:"original_width,omitempty" yaml:"original_width,omitempty"`

	// OriginalHeight is the source height before transparent trimming.
	OriginalHeight int `json:"original_height,omitempty" yaml:"original_height,omitempty"`

	// OffsetX is the crop's horizontal offset in the original source.
	OffsetX int `json:"offset_x,omitempty" yaml:"offset_x,omitempty"`

	// OffsetY is the crop's vertical offset in the original source.
	OffsetY int `json:"offset_y,omitempty" yaml:"offset_y,omitempty"`

	// Rotated reports clockwise 90-degree placement during rendering.
	Rotated bool `json:"rotated" yaml:"rotated"`

	// OverlapGroup identifies a canonical placement's explicit overlap zone.
	OverlapGroup int `json:"overlap_group,omitempty" yaml:"overlap_group,omitempty"`
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
	// Image provides the original pixels for rendering.
	// Its bounds must match the placement's recorded original dimensions,
	// or its content dimensions otherwise.
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
