// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import "errors"

var (
	// ErrInvalidOptions reports invalid packing options.
	ErrInvalidOptions = errors.New("invalid options")

	// ErrInvalidItem reports invalid item dimensions or identifiers.
	ErrInvalidItem = errors.New("invalid item")

	// ErrPlacementFailed reports that an item cannot be placed into an atlas.
	ErrPlacementFailed = errors.New("placement failed")

	// ErrLayoutRequired reports a missing layout for rendering.
	ErrLayoutRequired = errors.New("layout is required")

	// ErrRenderLimitExceeded reports that the RGBA output buffer exceeds MaxBytes.
	ErrRenderLimitExceeded = errors.New("render output exceeds byte limit")

	// ErrInvalidLayout reports malformed layout dimensions.
	ErrInvalidLayout = errors.New("invalid layout")

	// ErrSourceNotFound reports missing image source for a placement ID.
	ErrSourceNotFound = errors.New("source not found")

	// ErrSourceImageMissing reports a nil image in provided sources.
	ErrSourceImageMissing = errors.New("source image is missing")

	// ErrSourceSizeMismatch reports mismatch between placement size and source image size.
	ErrSourceSizeMismatch = errors.New("source size mismatch")

	// ErrImageLoaderRequired reports a missing lazy image loader.
	ErrImageLoaderRequired = errors.New("image loader is required")

	// ErrAtlasConsumerRequired reports a missing page consumer callback.
	ErrAtlasConsumerRequired = errors.New("atlas consumer is required")
)
