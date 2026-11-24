// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"bytes"
	"fmt"
	"hash/maphash"
	"image"
	"image/draw"
)

// pixelHashSeed randomizes hash buckets for image-driven pixel keys.
var pixelHashSeed = maphash.MakeSeed()

// imageHashFunc hashes the rendered RGBA pixels in a bounded-memory pass.
type imageHashFunc func(src image.Image) uint64

// pixelHashKey scopes a hash to one physical crop size.
type pixelHashKey struct {
	width  int    // crop width before rotation.
	height int    // crop height before rotation.
	hash   uint64 // process-local pixel hash.
}

// inMemoryCandidate keeps a borrowed view for Pack and PackPages comparisons.
type inMemoryCandidate struct {
	view image.Image // normalized source view without a raster copy.
	item Item        // physical canonical crop geometry.
}

// dedupRecord stores geometry and dimensions required by the loader pipeline.
type dedupRecord struct {
	id           string       // canonical source ID used for exact comparisons.
	item         Item         // physical canonical crop geometry.
	sourceWidth  int          // source width before trimming.
	sourceHeight int          // source height before trimming.
	trim         trimMetadata // crop geometry when trimming changed the bounds.
	hasTrim      bool         // whether trim contains an actual crop.
}

// pixelRows reuses bounded row buffers while hashing and comparing candidates.
type pixelRows struct {
	first  image.RGBA // normalized RGBA bytes for the current source row.
	second image.RGBA // normalized RGBA bytes for an exact-comparison candidate.
}

// hash hashes rendered rows using a reusable scratch row.
func (rows *pixelRows) hash(src image.Image) uint64 {
	bounds := src.Bounds()
	rows.ensure(&rows.first, bounds.Dx())
	rows.first.Rect = image.Rect(0, 0, bounds.Dx(), 1)
	rowPixels := rows.first.Pix[:bounds.Dx()*4]
	var hash maphash.Hash
	hash.SetSeed(pixelHashSeed)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		copyPixelRowRGBA(&rows.first, src, bounds.Min.X, y)
		_, _ = hash.Write(rowPixels)
	}

	return hash.Sum64()
}

// equal compares normalized rows using two reusable scratch buffers.
func (rows *pixelRows) equal(a, b image.Image) bool {
	aBounds := a.Bounds()
	bBounds := b.Bounds()
	if aBounds.Dx() != bBounds.Dx() || aBounds.Dy() != bBounds.Dy() {
		return false
	}

	rows.ensure(&rows.first, aBounds.Dx())
	rows.ensure(&rows.second, bBounds.Dx())
	rows.first.Rect = image.Rect(0, 0, aBounds.Dx(), 1)
	rows.second.Rect = image.Rect(0, 0, bBounds.Dx(), 1)
	aPixels := rows.first.Pix[:aBounds.Dx()*4]
	bPixels := rows.second.Pix[:bBounds.Dx()*4]

	for y := 0; y < aBounds.Dy(); y++ {
		copyPixelRowRGBA(&rows.first, a, aBounds.Min.X, aBounds.Min.Y+y)
		copyPixelRowRGBA(&rows.second, b, bBounds.Min.X, bBounds.Min.Y+y)

		if !bytes.Equal(aPixels, bPixels) {
			return false
		}
	}

	return true
}

// copyPixelRowRGBA keeps legacy draw paths and avoids interface allocations for key views.
func copyPixelRowRGBA(dst *image.RGBA, src image.Image, sourceX, sourceY int) {
	if view, ok := src.(colorKeyImageView); ok {
		pixels := dst.Pix[:view.Bounds().Dx()*4]
		for x := range view.Bounds().Dx() {
			pixel := view.RGBAAt(sourceX+x, sourceY)
			offset := x * 4
			pixels[offset] = pixel.R
			pixels[offset+1] = pixel.G
			pixels[offset+2] = pixel.B
			pixels[offset+3] = pixel.A
		}

		return
	}

	draw.Draw(dst, dst.Bounds(), src, image.Pt(sourceX, sourceY), draw.Src)
}

// ensure resizes one reusable buffer only when a wider source is encountered.
func (rows *pixelRows) ensure(row *image.RGBA, width int) {
	if row.Stride/4 < width {
		*row = *image.NewRGBA(image.Rect(0, 0, width, 1))
	}
}

// deduplicatePreparedSprites retains the first exact pixel match as canonical.
func deduplicatePreparedSprites(
	items []Item,
	sourceMap map[string]image.Image,
	trimByID map[string]trimMetadata,
	trim bool,
	hashImage imageHashFunc,
) ([]Item, []aliasRelation) {
	return deduplicatePreparedSpritesWithOptions(
		items,
		sourceMap,
		trimByID,
		Options{TrimTransparent: trim},
		hashImage,
	)
}

// deduplicatePreparedSpritesWithOptions compares pixels after requested image filters.
func deduplicatePreparedSpritesWithOptions(
	items []Item,
	sourceMap map[string]image.Image,
	trimByID map[string]trimMetadata,
	opts Options,
	hashImage imageHashFunc,
) ([]Item, []aliasRelation) {
	canonicalItems := make([]Item, 0, len(items))
	aliases := make([]aliasRelation, 0)
	buckets := make(map[pixelHashKey][]inMemoryCandidate, len(items))
	var rows pixelRows

	for i := range items {
		item := items[i]
		source := sourceMap[item.ID]
		view := sourceViewForItemWithOptions(item, source, opts, trimByID)
		var hash uint64
		if hashImage == nil {
			hash = rows.hash(view)
		} else {
			hash = hashImage(view)
		}

		key := pixelHashKey{
			width:  item.Width,
			height: item.Height,
			hash:   hash,
		}

		canonicalID := ""
		for _, candidate := range buckets[key] {
			if rows.equal(view, candidate.view) {
				canonicalID = candidate.item.ID
				break
			}
		}

		if canonicalID != "" {
			aliases = append(aliases, aliasRelation{
				id:          item.ID,
				canonicalID: canonicalID,
			})
			continue
		}

		canonicalItems = append(canonicalItems, item)
		buckets[key] = append(buckets[key], inMemoryCandidate{item: item, view: view})
	}

	return canonicalItems, aliases
}

// prepareDeduplicatedItems scans sources without retaining images between loads.
func prepareDeduplicatedItems(
	items []Item,
	opts Options,
	load ImageLoader,
	hashImage imageHashFunc,
) ([]Item, map[string]trimMetadata, []aliasRelation, error) {
	canonicalItems := make([]Item, 0, len(items))
	trimByID := make(map[string]trimMetadata, len(items))
	aliases := make([]aliasRelation, 0)
	buckets := make(map[pixelHashKey][]dedupRecord, len(items))
	var rows pixelRows

	for i := range items {
		item := items[i]
		source, err := load(item.ID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("scan source %q for deduplication: %w", item.ID, err)
		}

		if err := validatePlacementSource(Placement{
			ID:     item.ID,
			Width:  item.Width,
			Height: item.Height,
		}, source); err != nil {
			return nil, nil, nil, fmt.Errorf("scan source %q for deduplication: %w", item.ID, err)
		}

		width, height := item.Width, item.Height
		metadata := trimMetadata{}
		hasMetadata := false
		if opts.TrimTransparent || opts.ColorKey != nil && opts.ColorKey.Trim {
			width, height, metadata, hasMetadata = trimmedSourceGeometryWithOptions(source, opts)
			if hasMetadata {
				trimByID[item.ID] = metadata
			}
		}

		physicalItem := Item{ID: item.ID, Width: width, Height: height}
		view := sourceViewForGeometryWithOptions(physicalItem, source, opts, metadata, hasMetadata)
		var hash uint64
		if hashImage == nil {
			hash = rows.hash(view)
		} else {
			hash = hashImage(view)
		}

		key := pixelHashKey{
			width:  width,
			height: height,
			hash:   hash,
		}

		canonicalID := ""
		for _, candidate := range buckets[key] {
			canonicalSource, err := load(candidate.id)
			if err != nil {
				return nil, nil, nil, fmt.Errorf(
					"load canonical source %q for exact comparison: %w",
					candidate.id,
					err,
				)
			}

			if err := validatePlacementSource(Placement{
				ID:             candidate.id,
				Width:          candidate.item.Width,
				Height:         candidate.item.Height,
				OriginalWidth:  candidate.sourceWidth,
				OriginalHeight: candidate.sourceHeight,
			}, canonicalSource); err != nil {
				return nil, nil, nil, fmt.Errorf(
					"load canonical source %q for exact comparison: %w",
					candidate.id, err,
				)
			}

			canonicalView := sourceViewForRecordWithOptions(canonicalSource, candidate, opts)
			if rows.equal(view, canonicalView) {
				canonicalID = candidate.id
				break
			}
		}

		if canonicalID != "" {
			aliases = append(aliases, aliasRelation{
				id:          item.ID,
				canonicalID: canonicalID,
			})
			continue
		}

		canonicalItems = append(canonicalItems, physicalItem)
		buckets[key] = append(buckets[key], dedupRecord{
			id:           item.ID,
			item:         physicalItem,
			sourceWidth:  item.Width,
			sourceHeight: item.Height,
			trim:         metadata,
			hasTrim:      hasMetadata,
		})
	}

	return canonicalItems, trimByID, aliases, nil
}

// sourceViewForItem returns the exact source view used to compare one item.
func sourceViewForItemWithOptions(
	item Item,
	source image.Image,
	opts Options,
	trimByID map[string]trimMetadata,
) image.Image {
	if metadata, ok := trimByID[item.ID]; ok {
		return sourceViewForGeometryWithOptions(item, source, opts, metadata, true)
	}

	return sourceViewForGeometryWithOptions(item, source, opts, trimMetadata{}, false)
}

// sourceViewForGeometryWithOptions applies one crop and pixel transform without map allocations.
func sourceViewForGeometryWithOptions(
	item Item,
	source image.Image,
	opts Options,
	metadata trimMetadata,
	hasMetadata bool,
) image.Image {
	placement := Placement{
		ID:     item.ID,
		Width:  item.Width,
		Height: item.Height,
	}
	if hasMetadata {
		placement.OriginalWidth = metadata.originalWidth
		placement.OriginalHeight = metadata.originalHeight
		placement.OffsetX = metadata.offsetX
		placement.OffsetY = metadata.offsetY
	}

	return sourceForPlacementWithColorKey(placement, source, opts.TrimTransparent, opts.ColorKey)
}

// sourceViewForRecordWithOptions restores a candidate crop and pixel transform.
func sourceViewForRecordWithOptions(source image.Image, record dedupRecord, opts Options) image.Image {
	placement := Placement{
		ID:     record.id,
		Width:  record.item.Width,
		Height: record.item.Height,
	}
	if record.hasTrim {
		placement.OriginalWidth = record.trim.originalWidth
		placement.OriginalHeight = record.trim.originalHeight
		placement.OffsetX = record.trim.offsetX
		placement.OffsetY = record.trim.offsetY
	}

	return sourceForPlacementWithColorKey(placement, source, opts.TrimTransparent, opts.ColorKey)
}
