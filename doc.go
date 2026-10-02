// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

/*
Package atlasforge provides 2D texture atlas packing for sprite sheet generation.

It packs multiple small images into one or more atlases
and returns layout metadata with exact rectangle coordinates.
Packing uses the MaxRects algorithm and is deterministic:
repeated runs with the same input produce the same layout.

The package offers planning, rendering, and one-shot APIs:

  - [Plan]: compute placement from items and options, returns Layout.
  - [PlanPages]: split items across multiple atlas layouts.
  - [Render]: draw a Layout with sources into an atlas image.
  - [RenderWithLoader]: draw a Layout by loading each source on demand.
  - [Pack]: one-shot flow that plans from sprites and renders the atlas.
  - [PackPages]: one-shot flow that returns all rendered atlas pages.
  - [BuildPages]: render planned pages sequentially through a consumer callback.

See ExamplePack, ExamplePlanPages, and ExampleBuildPages for usage examples.
*/
package atlasforge
