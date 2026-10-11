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

// pixelHashSeed is process-local and never becomes part of the public layout.
var pixelHashSeed = maphash.MakeSeed()

// imageHashFunc hashes the rendered RGBA pixels in a bounded-memory pass.
type imageHashFunc func(src image.Image) uint64

// pixelHashKey scopes a hash to one physical crop size.
type pixelHashKey struct {
	// width is the crop width before rotation.
	width int
	// height is the crop height before rotation.
	height int
	// hash is the process-local pixel hash.
	hash uint64
}

// inMemoryCandidate keeps a borrowed view for Pack and PackPages comparisons.
type inMemoryCandidate struct {
	// view reads normalized source pixels without copying the raster.
	view image.Image
	// item describes the physical canonical crop.
	item Item
}

// dedupRecord stores geometry and dimensions required by the loader pipeline.
type dedupRecord struct {
	// id identifies the canonical source to load for exact comparisons.
	id string
	// item stores the canonical physical crop dimensions.
	item Item
	// sourceWidth is the source width before trimming.
	sourceWidth int
	// sourceHeight is the source height before trimming.
	sourceHeight int
	// trim stores the canonical source crop geometry when trimming changed bounds.
	trim trimMetadata
	// hasTrim reports whether trim metadata represents an actual crop.
	hasTrim bool
}

// pixelRows reuses bounded row buffers while hashing and comparing candidates.
type pixelRows struct {
	// first holds normalized RGBA bytes for the current source row.
	first image.RGBA
	// second holds normalized RGBA bytes for an exact-comparison candidate.
	second image.RGBA
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
		draw.Draw(&rows.first, rows.first.Bounds(), src, image.Pt(bounds.Min.X, y), draw.Src)
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
		draw.Draw(
			&rows.first,
			rows.first.Bounds(),
			a,
			image.Pt(aBounds.Min.X, aBounds.Min.Y+y),
			draw.Src,
		)
		draw.Draw(
			&rows.second,
			rows.second.Bounds(),
			b,
			image.Pt(bBounds.Min.X, bBounds.Min.Y+y),
			draw.Src,
		)

		if !bytes.Equal(aPixels, bPixels) {
			return false
		}
	}

	return true
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
	canonicalItems := make([]Item, 0, len(items))
	aliases := make([]aliasRelation, 0)
	buckets := make(map[pixelHashKey][]inMemoryCandidate, len(items))
	var rows pixelRows

	for i := range items {
		item := items[i]
		source := sourceMap[item.ID]
		view := sourceViewForItem(item, source, trim, trimByID)
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
			aliases = append(aliases, aliasRelation{id: item.ID, canonicalID: canonicalID})
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
		if opts.TrimTransparent {
			width, height, metadata, hasMetadata = trimmedSourceGeometry(source)
			if hasMetadata {
				trimByID[item.ID] = metadata
			}
		}

		physicalItem := Item{ID: item.ID, Width: width, Height: height}
		view := sourceViewForGeometry(physicalItem, source, opts.TrimTransparent, metadata, hasMetadata)
		var hash uint64
		if hashImage == nil {
			hash = rows.hash(view)
		} else {
			hash = hashImage(view)
		}

		key := pixelHashKey{width: width, height: height, hash: hash}

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

			canonicalView := sourceViewForRecord(canonicalSource, candidate, opts.TrimTransparent)
			if rows.equal(view, canonicalView) {
				canonicalID = candidate.id
				break
			}
		}

		if canonicalID != "" {
			aliases = append(aliases, aliasRelation{id: item.ID, canonicalID: canonicalID})
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
func sourceViewForItem(
	item Item,
	source image.Image,
	trim bool,
	trimByID map[string]trimMetadata,
) image.Image {
	if metadata, ok := trimByID[item.ID]; ok {
		return sourceViewForGeometry(item, source, trim, metadata, true)
	}

	return sourceViewForGeometry(item, source, trim, trimMetadata{}, false)
}

// sourceViewForGeometry applies one crop record without allocating metadata maps.
func sourceViewForGeometry(item Item, source image.Image, trim bool, metadata trimMetadata, hasMetadata bool) image.Image {
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

	return sourceForPlacement(placement, source, trim)
}

// sourceViewForRecord restores a candidate's crop from its original source.
func sourceViewForRecord(source image.Image, record dedupRecord, trim bool) image.Image {
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

	return sourceForPlacement(placement, source, trim)
}
