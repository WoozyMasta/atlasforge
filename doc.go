// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

/*
Package atlasforge provides 2D texture atlas packing for sprite sheet generation.

It packs multiple small images into one or more atlases
and returns layout metadata with exact rectangle coordinates.
Packing uses the MaxRects algorithm and is deterministic:
repeated runs with the same input produce the same layout.

Image-based [Pack] calls can trim transparent outer borders.
Cropped placements keep original dimensions and offsets;
render those layouts with the original source images to reproduce the atlas.

The package offers planning, rendering, and one-shot APIs:

  - [Plan]: compute placement from items and options, returns Layout.
  - [PlanPages]: split items across multiple atlas layouts.
  - [Render]: draw a Layout with sources into an atlas image.
  - [RenderWithOptions]: draw a Layout with a caller-supplied output byte limit.
  - [RenderWithLoader]: draw a Layout by loading each source on demand.
  - [RenderWithLoaderOptions]: draw with lazy sources and a caller-supplied limit.
  - [Pack]: one-shot flow that plans from sprites and renders the atlas.
  - [PackPages]: one-shot flow that returns all rendered atlas pages.
  - [BuildPages]: render planned pages sequentially through a consumer callback.

See ExamplePack, ExamplePlanPages, and ExampleBuildPages for usage examples.

When rendering an untrusted Layout, use [RenderWithOptions]
or [RenderWithLoaderOptions] with a nonzero [RenderOptions.MaxBytes].
This limit covers only the output RGBA buffer;
it does not bound total process memory or memory used by source image decoders.
*/
package atlasforge
