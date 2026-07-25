// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

/*
Package atlasforge provides 2D texture atlas packing for sprite sheet generation.

It packs multiple small images into one larger atlas
and returns layout metadata with exact rectangle coordinates.
Packing uses the MaxRects algorithm and is deterministic:
repeated runs with the same input produce the same layout.

The package offers three main APIs:

  - [Plan]: compute placement from items and options, returns Layout.
  - [Render]: draw a Layout with sources into an atlas image.
  - [Pack]: one-shot flow that plans from sprites and renders the atlas.

See ExamplePack for a complete usage example.
*/
package atlasforge
