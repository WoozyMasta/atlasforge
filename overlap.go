// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
)

const (
	maxOverlapZoneMembers = 128
	maxGuillotineBins     = 32
	zoneAreaSlackPercent  = 15
)

// overlapSide identifies one of the four full sprite edges.
type overlapSide uint8

const (
	overlapLeft overlapSide = iota
	overlapRight
	overlapTop
	overlapBottom
)

// overlapProfile stores one eligible sprite's shared edge color and depths.
type overlapProfile struct {
	item   Item   // prepared sprite geometry and stable ID.
	color  uint32 // exact canonical RGBA8 color shared by all edges.
	depths [4]int // solid-band depths ordered by overlapSide.
}

// overlapMember stores one full sprite at its local zone coordinates.
type overlapMember struct {
	item Item // original prepared sprite geometry.
	x    int  // local X position, also the full placement's X offset.
	y    int  // local Y position, also the full placement's Y offset.
}

// overlapZone describes one N-member virtual item for the global planner.
type overlapZone struct {
	virtualID string          // planner-only item ID.
	members   []overlapMember // physical canonical sprites in the zone.
	color     uint32          // shared uniform edge color.
	shareX    int             // shared width before any global rotation.
	shareY    int             // shared height before any global rotation.
	width     int             // full member union width before rotation.
	height    int             // full member union height before rotation.
	id        int             // stable public overlap block ID.
}

// overlapGrouping replaces each accepted zone with one global planner item.
type overlapGrouping struct {
	zonesByID  map[string]overlapZone // virtual planner ID to its zone.
	memberZone map[string]string      // canonical sprite ID to virtual zone ID.
	items      []Item                 // ordinary sprites and virtual zones.
}

// gridCandidate stores one legal row/column arrangement for equal work sizes.
type gridCandidate struct {
	columns int   // number of cells in each full row.
	rows    int   // number of rows including a partial final row.
	width   int   // bounding width of full sprite rectangles.
	height  int   // bounding height of full sprite rectangles.
	area    int64 // bounding area used for candidate selection.
}

// freeRect describes one non-overlapping Guillotine remainder.
type freeRect struct {
	x      int // local X origin.
	y      int // local Y origin.
	width  int // free width.
	height int // free height.
}

// guillotinePlacement stores one work rectangle's coordinates.
type guillotinePlacement struct {
	item Item // original prepared sprite.
	x    int  // work rectangle X.
	y    int  // work rectangle Y.
}

// scanOverlapProfile returns the exact common edge color and four band depths.
func scanOverlapProfile(
	source image.Image,
	item Item,
	metadata trimMetadata,
	options Options,
) (overlapProfile, bool, error) {
	placement := Placement{ID: item.ID, Width: item.Width, Height: item.Height}
	if metadata.originalWidth > 0 {
		placement.OriginalWidth = metadata.originalWidth
		placement.OriginalHeight = metadata.originalHeight
		placement.OffsetX = metadata.offsetX
		placement.OffsetY = metadata.offsetY
	}
	view := sourceForPlacementWithColorKey(
		placement,
		source,
		options.TrimTransparent,
		options.ColorKey,
	)
	if view.Bounds().Dx() != item.Width || view.Bounds().Dy() != item.Height {
		return overlapProfile{}, false, ErrSourceSizeMismatch
	}

	profile := overlapProfile{item: item}
	for side := overlapLeft; side <= overlapBottom; side++ {
		depth, edgeColor := scanUniformBand(view, side)
		if depth == 0 || side > overlapLeft && edgeColor != profile.color {
			return overlapProfile{}, false, nil
		}

		if side == overlapLeft {
			profile.color = edgeColor
		}
		profile.depths[side] = depth
	}

	return profile, true, nil
}

// scanUniformBand counts consecutive full RGBA8 rows or columns from one edge.
func scanUniformBand(source image.Image, side overlapSide) (int, uint32) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	if rgba, ok := source.(*image.RGBA); ok {
		return scanUniformRGBABand(rgba, side)
	}

	lineLength, maxDepth := width, height
	if side == overlapLeft || side == overlapRight {
		lineLength, maxDepth = height, width
	}
	first := edgePixel(source, bounds, side, 0, 0)
	for position := 1; position < lineLength; position++ {
		if edgePixel(source, bounds, side, 0, position) != first {
			return 0, packRGBA8(first)
		}
	}

	depth := 1
	for ; depth < maxDepth; depth++ {
		for position := 0; position < lineLength; position++ {
			if edgePixel(source, bounds, side, depth, position) != first {
				return depth, packRGBA8(first)
			}
		}
	}

	return depth, packRGBA8(first)
}

// scanUniformRGBABand scans standard RGBA storage without per-pixel At calls.
func scanUniformRGBABand(source *image.RGBA, side overlapSide) (int, uint32) {
	bounds := source.Rect
	lineLength, maxDepth := bounds.Dx(), bounds.Dy()
	// SubImage offsets Pix to Rect.Min while retaining the parent's Stride.
	start := 0
	depthStep, lineStep := source.Stride, 4

	switch side {
	case overlapLeft:
		lineLength, maxDepth = bounds.Dy(), bounds.Dx()
		depthStep, lineStep = 4, source.Stride

	case overlapRight:
		lineLength, maxDepth = bounds.Dy(), bounds.Dx()
		start += (bounds.Dx() - 1) * 4
		depthStep, lineStep = -4, source.Stride

	case overlapTop:
		depthStep, lineStep = source.Stride, 4

	case overlapBottom:
		start += (bounds.Dy() - 1) * source.Stride
		depthStep, lineStep = -source.Stride, 4
	}

	first := color.RGBA{
		R: source.Pix[start], G: source.Pix[start+1],
		B: source.Pix[start+2], A: source.Pix[start+3],
	}

	for position := 1; position < lineLength; position++ {
		offset := start + position*lineStep
		if source.Pix[offset] != first.R ||
			source.Pix[offset+1] != first.G ||
			source.Pix[offset+2] != first.B ||
			source.Pix[offset+3] != first.A {
			return 0, packRGBA8(first)
		}
	}

	for depth := 1; depth < maxDepth; depth++ {
		lineOffset := start + depth*depthStep
		for position := range lineLength {
			offset := lineOffset + position*lineStep
			if source.Pix[offset] != first.R ||
				source.Pix[offset+1] != first.G ||
				source.Pix[offset+2] != first.B ||
				source.Pix[offset+3] != first.A {
				return depth, packRGBA8(first)
			}
		}
	}

	return maxDepth, packRGBA8(first)
}

// edgePixel reads one pixel by depth and position from a requested full edge.
func edgePixel(source image.Image, bounds image.Rectangle, side overlapSide, depth, position int) color.RGBA {
	x, y := bounds.Min.X, bounds.Min.Y
	switch side {
	case overlapLeft:
		x += depth
		y += position

	case overlapRight:
		x += bounds.Dx() - 1 - depth
		y += position

	case overlapTop:
		x += position
		y += depth

	case overlapBottom:
		x += position
		y += bounds.Dy() - 1 - depth
	}

	return canonicalImageRGBA8(source, x, y)
}

// packRGBA8 encodes canonical RGBA channel bytes in stable network order.
func packRGBA8(pixel color.RGBA) uint32 {
	return uint32(pixel.R)<<24 | uint32(pixel.G)<<16 | uint32(pixel.B)<<8 | uint32(pixel.A)
}

// unpackRGBA8 restores canonical RGBA channel bytes from a packed color.
func unpackRGBA8(packed uint32) color.RGBA {
	return color.RGBA{
		R: uint8(packed >> 24),
		G: uint8((packed >> 16) & 0xff),
		B: uint8((packed >> 8) & 0xff),
		A: uint8(packed & 0xff),
	}
}

// findOverlapGrouping scans each source once and builds deterministic color zones.
func findOverlapGrouping(
	items []Item,
	trimByID map[string]trimMetadata,
	options Options,
	load ImageLoader,
) (overlapGrouping, error) {
	buckets := make(map[uint32][]overlapProfile)
	for _, item := range items {
		source, err := load(item.ID)
		if err != nil {
			return overlapGrouping{}, fmt.Errorf("load source %q for overlap scan: %w", item.ID, err)
		}
		if isNilImage(source) {
			return overlapGrouping{}, fmt.Errorf("%w: ID=%q", ErrSourceImageMissing, item.ID)
		}

		profile, eligible, err := scanOverlapProfile(source, item, trimByID[item.ID], options)
		if err != nil {
			return overlapGrouping{}, fmt.Errorf("scan source %q edges: %w", item.ID, err)
		}

		if eligible {
			buckets[profile.color] = append(buckets[profile.color], profile)
		}
	}

	colors := make([]uint32, 0, len(buckets))
	for edgeColor := range buckets {
		colors = append(colors, edgeColor)
	}

	slices.Sort(colors)
	zones := make([]overlapZone, 0)
	for _, edgeColor := range colors {
		profiles := buckets[edgeColor]
		slices.SortFunc(profiles, compareOverlapProfiles)
		zones = append(zones, splitOverlapBucket(profiles, edgeColor, options)...)
	}

	usedIDs := make(map[string]struct{}, len(items)+len(zones))
	for _, item := range items {
		usedIDs[item.ID] = struct{}{}
	}

	grouping := overlapGrouping{
		zonesByID:  make(map[string]overlapZone, len(zones)),
		memberZone: make(map[string]string),
	}

	for i := range zones {
		zones[i].id = i + 1
		zones[i].virtualID = overlapVirtualID(i+1, usedIDs)
		usedIDs[zones[i].virtualID] = struct{}{}
		grouping.zonesByID[zones[i].virtualID] = zones[i]
		for _, member := range zones[i].members {
			grouping.memberZone[member.item.ID] = zones[i].virtualID
		}
	}

	addedZones := make(map[string]struct{}, len(zones))
	grouping.items = make([]Item, 0, len(items)-len(grouping.memberZone)+len(zones))
	for _, item := range items {
		virtualID, grouped := grouping.memberZone[item.ID]
		if !grouped {
			grouping.items = append(grouping.items, item)
			continue
		}
		if _, added := addedZones[virtualID]; added {
			continue
		}

		zone := grouping.zonesByID[virtualID]
		grouping.items = append(grouping.items, Item{
			ID:     virtualID,
			Width:  zone.width,
			Height: zone.height,
		})
		addedZones[virtualID] = struct{}{}
	}

	return grouping, nil
}

// compareOverlapProfiles prioritizes thick strips, then larger items and stable IDs.
func compareOverlapProfiles(a, b overlapProfile) int {
	depthA := min(a.depths[overlapLeft], a.depths[overlapRight], a.depths[overlapTop], a.depths[overlapBottom])
	depthB := min(b.depths[overlapLeft], b.depths[overlapRight], b.depths[overlapTop], b.depths[overlapBottom])
	if depthA != depthB {
		return depthB - depthA
	}

	areaA, _ := checkedArea(int64(a.item.Width), int64(a.item.Height))
	areaB, _ := checkedArea(int64(b.item.Width), int64(b.item.Height))
	if areaA != areaB {
		if areaA > areaB {
			return -1
		}
		return 1
	}
	if a.item.ID < b.item.ID {
		return -1
	}
	if a.item.ID > b.item.ID {
		return 1
	}

	return 0
}

// splitOverlapBucket recursively isolates compatible subsets that fit MaxSize.
func splitOverlapBucket(profiles []overlapProfile, edgeColor uint32, options Options) []overlapZone {
	if len(profiles) < 2 {
		return nil
	}

	if len(profiles) > maxOverlapZoneMembers {
		zones := splitOverlapBucket(profiles[:maxOverlapZoneMembers], edgeColor, options)
		return append(zones, splitOverlapBucket(profiles[maxOverlapZoneMembers:], edgeColor, options)...)
	}

	if zone, ok := makeOverlapZone(profiles, edgeColor, options); ok {
		return []overlapZone{zone}
	}

	if len(profiles) <= 2 {
		return nil
	}

	middle := len(profiles) / 2
	zones := splitOverlapBucket(profiles[:middle], edgeColor, options)

	return append(zones, splitOverlapBucket(profiles[middle:], edgeColor, options)...)
}

// makeOverlapZone computes conservative shares and accepts only a local area saving.
func makeOverlapZone(profiles []overlapProfile, edgeColor uint32, options Options) (overlapZone, bool) {
	minWidth, minHeight := profiles[0].item.Width, profiles[0].item.Height
	minLeft, minRight := profiles[0].depths[overlapLeft], profiles[0].depths[overlapRight]
	minTop, minBottom := profiles[0].depths[overlapTop], profiles[0].depths[overlapBottom]

	for _, profile := range profiles[1:] {
		minWidth = min(minWidth, profile.item.Width)
		minHeight = min(minHeight, profile.item.Height)
		minLeft = min(minLeft, profile.depths[overlapLeft])
		minRight = min(minRight, profile.depths[overlapRight])
		minTop = min(minTop, profile.depths[overlapTop])
		minBottom = min(minBottom, profile.depths[overlapBottom])
	}

	shareX := max(0, min(minLeft, minRight)-options.Padding)
	shareY := max(0, min(minTop, minBottom)-options.Padding)
	shareX = min(shareX, minWidth-1)
	shareY = min(shareY, minHeight-1)

	if shareX == 0 && shareY == 0 {
		return overlapZone{}, false
	}

	members := make([]overlapMember, len(profiles))
	for i, profile := range profiles {
		members[i] = overlapMember{item: profile.item}
	}

	slices.SortFunc(members, func(a, b overlapMember) int {
		if a.item.ID < b.item.ID {
			return -1
		}
		if a.item.ID > b.item.ID {
			return 1
		}

		return 0
	})

	packed, ok := solveOverlapZone(members, shareX, shareY, options.MaxSize-2*options.Padding)
	if !ok {
		return overlapZone{}, false
	}

	zone, ok := zoneFromPlacements(packed, edgeColor, shareX, shareY)
	if !ok || !zoneFitsMaxSize(zone, options) || !zoneSavesArea(zone, members, options.Padding) {
		return overlapZone{}, false
	}

	return zone, true
}

// solveOverlapZone selects Grid for uniform work sizes and Guillotine otherwise.
func solveOverlapZone(
	members []overlapMember,
	shareX, shareY, maxContentSide int,
) ([]overlapMember, bool) {
	if len(members) < 2 || shareX < 0 || shareY < 0 || maxContentSide <= 0 {
		return nil, false
	}

	workWidth, workHeight := members[0].item.Width-shareX, members[0].item.Height-shareY
	if workWidth <= 0 || workHeight <= 0 {
		return nil, false
	}

	uniform := true
	for _, member := range members[1:] {
		if member.item.Width-shareX != workWidth || member.item.Height-shareY != workHeight {
			uniform = false
			break
		}
	}

	if uniform {
		return solveOverlapGrid(members, shareX, shareY, maxContentSide)
	}

	return solveOverlapGuillotine(members, shareX, shareY, maxContentSide)
}

// solveOverlapGrid enumerates compact row-major layouts for equal work rectangles.
func solveOverlapGrid(members []overlapMember, shareX, shareY, maxContentSide int) ([]overlapMember, bool) {
	count := len(members)
	workWidth := members[0].item.Width - shareX
	workHeight := members[0].item.Height - shareY
	candidates := make([]gridCandidate, 0, count)
	minimumArea := int64(math.MaxInt64)

	for columns := 1; columns <= count; columns++ {
		rows := (count + columns - 1) / columns
		width := (columns-1)*workWidth + members[0].item.Width
		height := (rows-1)*workHeight + members[0].item.Height
		if width > maxContentSide || height > maxContentSide {
			continue
		}

		area, err := checkedArea(int64(width), int64(height))
		if err != nil {
			continue
		}

		candidate := gridCandidate{
			columns: columns,
			rows:    rows,
			width:   width,
			height:  height,
			area:    area,
		}
		candidates = append(candidates, candidate)
		minimumArea = min(minimumArea, area)
	}

	if len(candidates) == 0 {
		return nil, false
	}

	bestIndex := -1
	for i, candidate := range candidates {
		if !withinAreaWindow(candidate.area, minimumArea) {
			continue
		}

		if bestIndex < 0 || compareGridCandidates(candidate, candidates[bestIndex]) < 0 {
			bestIndex = i
		}
	}

	if bestIndex < 0 {
		return nil, false
	}

	best := candidates[bestIndex]
	placements := make([]overlapMember, count)
	for index, member := range members {
		member.x = index % best.columns * workWidth
		member.y = index / best.columns * workHeight
		placements[index] = member
	}

	return placements, true
}

// compareGridCandidates prefers square-like, then smaller and wider grids.
func compareGridCandidates(a, b gridCandidate) int {
	maxA, minA := max(a.width, a.height), min(a.width, a.height)
	maxB, minB := max(b.width, b.height), min(b.width, b.height)
	ratioA := int64(maxA) * int64(minB)
	ratioB := int64(maxB) * int64(minA)

	if ratioA != ratioB {
		if ratioA < ratioB {
			return -1
		}
		return 1
	}

	if a.area != b.area {
		if a.area < b.area {
			return -1
		}
		return 1
	}

	if a.width != b.width {
		if a.width > b.width {
			return -1
		}
		return 1
	}

	if a.columns < b.columns {
		return -1
	}
	if a.columns > b.columns {
		return 1
	}

	return 0
}

// withinAreaWindow reports whether area is at most 15 percent above minimumArea.
func withinAreaWindow(area, minimumArea int64) bool {
	if area <= minimumArea {
		return true
	}

	window := minimumArea/100*zoneAreaSlackPercent + minimumArea%100*zoneAreaSlackPercent/100
	return area-minimumArea <= window
}

// solveOverlapGuillotine searches a bounded set of bins using BAF/BSSF scoring.
func solveOverlapGuillotine(
	members []overlapMember,
	shareX, shareY, maxContentSide int,
) ([]overlapMember, bool) {
	work := make([]Item, len(members))
	var totalArea int64
	maxWorkWidth, maxWorkHeight := 0, 0

	for i, member := range members {
		width, height := member.item.Width-shareX, member.item.Height-shareY
		if width <= 0 || height <= 0 {
			return nil, false
		}

		area, err := checkedArea(int64(width), int64(height))
		if err != nil || totalArea > math.MaxInt64-area {
			return nil, false
		}

		totalArea += area
		maxWorkWidth = max(maxWorkWidth, width)
		maxWorkHeight = max(maxWorkHeight, height)
		work[i] = Item{ID: member.item.ID, Width: width, Height: height}
	}

	slices.SortFunc(work, compareWorkItems)
	widths := guillotineWidthCandidates(work, totalArea, maxWorkWidth, maxWorkHeight, maxContentSide)
	type packedZone struct {
		members []overlapMember
		width   int
		height  int
		area    int64
	}

	candidates := make([]packedZone, 0, maxGuillotineBins)
	minimumArea := int64(math.MaxInt64)
	probes := 0

	for _, width := range widths {
		minHeight := max(maxWorkHeight, ceilDiv64(totalArea, int64(width)))
		heights := guillotineHeightCandidates(minHeight, maxWorkHeight, maxContentSide)
		for _, height := range heights {
			if probes == maxGuillotineBins {
				break
			}

			probes++
			packed, ok := guillotinePack(work, width, height)
			if !ok {
				continue
			}

			fullMembers := restoreWorkPlacements(members, packed)
			zone, ok := zoneFromPlacements(fullMembers, 0, shareX, shareY)
			if !ok {
				continue
			}

			area, err := checkedArea(int64(zone.width), int64(zone.height))
			if err != nil {
				continue
			}

			candidates = append(candidates, packedZone{
				members: fullMembers,
				width:   zone.width,
				height:  zone.height,
				area:    area,
			})
			minimumArea = min(minimumArea, area)
		}
	}

	if len(candidates) == 0 {
		return nil, false
	}

	bestIndex := -1
	for index, candidate := range candidates {
		if !withinAreaWindow(candidate.area, minimumArea) {
			continue
		}

		if bestIndex < 0 || compareGridCandidates(
			gridCandidate{
				width:  candidate.width,
				height: candidate.height,
				area:   candidate.area,
			},
			gridCandidate{
				width:  candidates[bestIndex].width,
				height: candidates[bestIndex].height,
				area:   candidates[bestIndex].area,
			},
		) < 0 {
			bestIndex = index
		}
	}

	return candidates[bestIndex].members, true
}

// compareWorkItems defines stable Guillotine placement order.
func compareWorkItems(a, b Item) int {
	if a.Height != b.Height {
		return b.Height - a.Height
	}
	if a.Width != b.Width {
		return b.Width - a.Width
	}

	areaA, _ := checkedArea(int64(a.Width), int64(a.Height))
	areaB, _ := checkedArea(int64(b.Width), int64(b.Height))
	if areaA != areaB {
		if areaA > areaB {
			return -1
		}
		return 1
	}

	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}

	return 0
}

// guillotineWidthCandidates builds a small stable set around useful dimensions.
func guillotineWidthCandidates(
	work []Item,
	totalArea int64,
	maxWorkWidth, maxWorkHeight, limit int,
) []int {
	values := []int{maxWorkWidth, int(math.Ceil(math.Sqrt(float64(totalArea))))}
	areaWidth := ceilDiv64(totalArea, int64(maxWorkHeight))
	if areaWidth <= limit {
		values = append(values, areaWidth)
	}

	for i := 0; i < min(3, len(work)); i++ {
		sum := 0
		for j := 0; j <= i; j++ {
			sum += work[j].Width
		}
		values = append(values, sum)
	}

	values = append(values, limit)
	requiredSet := make(map[int]struct{}, len(values))

	for _, value := range values {
		if value >= maxWorkWidth && value <= limit {
			requiredSet[value] = struct{}{}
		}
	}

	candidateSet := make(map[int]struct{}, len(requiredSet)*3)
	for candidate := range requiredSet {
		candidateSet[candidate] = struct{}{}
		for _, nearby := range []int{candidate - maxWorkWidth/2, candidate + maxWorkWidth/2} {
			if nearby >= maxWorkWidth && nearby <= limit {
				candidateSet[nearby] = struct{}{}
			}
		}
	}

	var required, optional []int
	for candidate := range candidateSet {
		if _, exists := requiredSet[candidate]; exists {
			required = append(required, candidate)
		} else {
			optional = append(optional, candidate)
		}
	}

	slices.Sort(required)
	slices.Sort(optional)
	widthLimit := maxGuillotineBins / 2
	if len(required) > widthLimit {
		required = required[:widthLimit]
	}

	candidates := append([]int(nil), required...)
	for _, candidate := range optional {
		if len(candidates) == widthLimit {
			break
		}

		candidates = append(candidates, candidate)
	}
	slices.Sort(candidates)

	return candidates
}

// guillotineHeightCandidates returns a bounded set of required and slack heights.
func guillotineHeightCandidates(minHeight, maxWorkHeight, limit int) []int {
	values := []int{minHeight, minHeight + maxWorkHeight/2, minHeight + maxWorkHeight, limit}
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))

	for _, value := range values {
		if value < minHeight || value > limit {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	slices.Sort(result)
	return result
}

// guillotinePack packs every work rectangle or rejects the candidate bin.
func guillotinePack(items []Item, width, height int) ([]guillotinePlacement, bool) {
	free := []freeRect{{width: width, height: height}}
	placements := make([]guillotinePlacement, 0, len(items))

	for _, item := range items {
		bestIndex, bestShortWaste := -1, math.MaxInt
		bestAreaWaste := int64(math.MaxInt64)
		for index, rect := range free {
			if item.Width > rect.width || item.Height > rect.height {
				continue
			}

			areaWaste := int64(rect.width)*int64(rect.height) - int64(item.Width)*int64(item.Height)
			shortWaste := min(rect.width-item.Width, rect.height-item.Height)
			if bestIndex < 0 ||
				areaWaste < bestAreaWaste || areaWaste == bestAreaWaste &&
				(shortWaste < bestShortWaste || shortWaste == bestShortWaste &&
					(rect.y < free[bestIndex].y || rect.y == free[bestIndex].y &&
						rect.x < free[bestIndex].x)) {
				bestIndex, bestAreaWaste, bestShortWaste = index, areaWaste, shortWaste
			}
		}
		if bestIndex < 0 {
			return nil, false
		}

		rect := free[bestIndex]
		placements = append(placements, guillotinePlacement{item: item, x: rect.x, y: rect.y})
		free = append(free[:bestIndex], free[bestIndex+1:]...)
		free = append(free, chooseGuillotineSplit(rect, item.Width, item.Height)...)
	}

	return placements, true
}

// chooseGuillotineSplit selects the split with the stronger largest remainder.
func chooseGuillotineSplit(rect freeRect, width, height int) []freeRect {
	dx, dy := rect.width-width, rect.height-height

	vertical := make([]freeRect, 0, 2)
	if dx > 0 {
		vertical = append(vertical, freeRect{x: rect.x + width, y: rect.y, width: dx, height: rect.height})
	}
	if dy > 0 {
		vertical = append(vertical, freeRect{x: rect.x, y: rect.y + height, width: width, height: dy})
	}

	horizontal := make([]freeRect, 0, 2)
	if dx > 0 {
		horizontal = append(horizontal, freeRect{x: rect.x + width, y: rect.y, width: dx, height: height})
	}
	if dy > 0 {
		horizontal = append(horizontal, freeRect{x: rect.x, y: rect.y + height, width: rect.width, height: dy})
	}

	if compareFreeSplits(vertical, horizontal) <= 0 {
		return vertical
	}

	return horizontal
}

// compareFreeSplits scores the largest usable remainder and its short side.
func compareFreeSplits(a, b []freeRect) int {
	areaA, shortA := largestFreeRectScore(a)
	areaB, shortB := largestFreeRectScore(b)
	if areaA != areaB {
		if areaA > areaB {
			return -1
		}
		return 1
	}

	if shortA != shortB {
		if shortA > shortB {
			return -1
		}
		return 1
	}

	return 0
}

// largestFreeRectScore returns the area and short side of the strongest remainder.
func largestFreeRectScore(rects []freeRect) (int64, int) {
	var bestArea int64
	bestShort := 0
	for _, rect := range rects {
		area := int64(rect.width) * int64(rect.height)
		short := min(rect.width, rect.height)
		if area > bestArea || area == bestArea && short > bestShort {
			bestArea, bestShort = area, short
		}
	}

	return bestArea, bestShort
}

// restoreWorkPlacements maps sorted work coordinates back to original members.
func restoreWorkPlacements(members []overlapMember, packed []guillotinePlacement) []overlapMember {
	memberByID := make(map[string]overlapMember, len(members))
	for _, member := range members {
		memberByID[member.item.ID] = member
	}

	for _, placement := range packed {
		member := memberByID[placement.item.ID]
		member.x, member.y = placement.x, placement.y
		memberByID[placement.item.ID] = member
	}

	result := make([]overlapMember, len(members))
	for i, member := range members {
		result[i] = memberByID[member.item.ID]
	}

	return result
}

// zoneFromPlacements derives the full content bounds from member rectangles.
func zoneFromPlacements(
	members []overlapMember,
	edgeColor uint32,
	shareX, shareY int,
) (overlapZone, bool) {
	if len(members) < 2 {
		return overlapZone{}, false
	}

	minX, minY := math.MaxInt, math.MaxInt
	maxX, maxY := 0, 0
	for _, member := range members {
		if member.x < 0 || member.y < 0 ||
			member.item.Width <= 0 || member.item.Height <= 0 ||
			member.x > math.MaxInt-member.item.Width || member.y > math.MaxInt-member.item.Height {
			return overlapZone{}, false
		}

		minX, minY = min(minX, member.x), min(minY, member.y)
		maxX = max(maxX, member.x+member.item.Width)
		maxY = max(maxY, member.y+member.item.Height)
	}

	for i := range members {
		members[i].x -= minX
		members[i].y -= minY
	}

	return overlapZone{
		members: members,
		color:   edgeColor,
		shareX:  shareX,
		shareY:  shareY,
		width:   maxX - minX,
		height:  maxY - minY,
	}, true
}

// zoneFitsMaxSize includes only the block's external padding in global limits.
func zoneFitsMaxSize(zone overlapZone, options Options) bool {
	if options.Padding > math.MaxInt/2 {
		return false
	}

	reservedWidth, reservedHeight := zone.width+2*options.Padding, zone.height+2*options.Padding
	return reservedWidth >= zone.width && reservedHeight >= zone.height &&
		reservedWidth <= options.MaxSize && reservedHeight <= options.MaxSize
}

// zoneSavesArea compares one padded zone with its separate padded members.
func zoneSavesArea(zone overlapZone, members []overlapMember, padding int) bool {
	zoneArea, ok := paddedArea(zone.width, zone.height, padding)
	if !ok {
		return false
	}

	var individualArea int64
	for _, member := range members {
		area, valid := paddedArea(member.item.Width, member.item.Height, padding)
		if !valid || individualArea > math.MaxInt64-area {
			return false
		}

		individualArea += area
	}

	return zoneArea < individualArea
}

// paddedArea returns one checked content area including both padding edges.
func paddedArea(width, height, padding int) (int64, bool) {
	if width <= 0 || height <= 0 ||
		padding < 0 || padding > math.MaxInt/2 ||
		width > math.MaxInt-2*padding || height > math.MaxInt-2*padding {
		return 0, false
	}

	area, err := checkedArea(int64(width+2*padding), int64(height+2*padding))
	return area, err == nil
}

// ceilDiv64 returns the ceiling of a positive integer division.
func ceilDiv64(value int64, divisor int64) int {
	if value <= 0 || divisor <= 0 {
		return 0
	}

	return int(value/divisor + int64(boolInt(value%divisor != 0)))
}

// boolInt converts a boolean rounding indicator to zero or one.
func boolInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

// placementContentBounds returns physical atlas bounds after optional rotation.
func placementContentBounds(placement Placement) image.Rectangle {
	width, height := placement.Width, placement.Height
	if placement.Rotated {
		width, height = height, width
	}

	return image.Rect(placement.X, placement.Y, placement.X+width, placement.Y+height)
}

// findOverlapGrouping-compatible ID allocation avoids collisions with user IDs.
func overlapVirtualID(index int, used map[string]struct{}) string {
	for suffix := 0; ; suffix++ {
		id := fmt.Sprintf("\x00atlasforge-overlap-zone-%d-%d", index, suffix)
		if _, exists := used[id]; !exists {
			return id
		}
	}
}

// expandOverlapLayout restores full member placements and records physical blocks.
func expandOverlapLayout(
	layout *Layout,
	grouping overlapGrouping,
	trimByID map[string]trimMetadata,
	options Options,
) {
	placements := make([]Placement, 0, len(layout.Placements)+len(grouping.memberZone))
	blocks := make([]OverlapBlock, 0, len(grouping.zonesByID))

	for _, placement := range layout.Placements {
		zone, isZone := grouping.zonesByID[placement.ID]
		if !isZone {
			placements = append(placements, placement)
			continue
		}

		blockWidth, blockHeight := zone.width, zone.height
		if placement.Rotated {
			blockWidth, blockHeight = blockHeight, blockWidth
		}

		blockID := zone.id
		blocks = append(blocks, OverlapBlock{
			ID:      blockID,
			X:       placement.X,
			Y:       placement.Y,
			Width:   blockWidth,
			Height:  blockHeight,
			Color:   zone.color,
			ShareX:  zone.shareX,
			ShareY:  zone.shareY,
			Rotated: placement.Rotated,
		})

		for _, member := range zone.members {
			x, y := member.x, member.y
			if placement.Rotated {
				x, y = zone.height-member.y-member.item.Height, member.x
			}
			child := Placement{
				ID:           member.item.ID,
				X:            placement.X + x,
				Y:            placement.Y + y,
				Width:        member.item.Width,
				Height:       member.item.Height,
				Rotated:      placement.Rotated,
				OverlapGroup: blockID,
			}

			if metadata, exists := trimByID[member.item.ID]; exists {
				child.OriginalWidth = metadata.originalWidth
				child.OriginalHeight = metadata.originalHeight
				child.OffsetX = metadata.offsetX
				child.OffsetY = metadata.offsetY
			}

			placements = append(placements, child)
		}
	}

	layout.Placements = placements
	layout.OverlapBlocks = blocks
	if len(blocks) > 0 {
		if layout.Render == nil {
			layout.Render = &RenderSettings{
				Padding: options.Padding,
				Extrude: options.Extrude,
			}
		} else {
			layout.Render.Padding = options.Padding
			layout.Render.Extrude = options.Extrude
		}
	}
}

// planImageItems uses the overlap candidate only when global metrics do not worsen.
func planImageItems(
	items []Item,
	trimByID map[string]trimMetadata,
	options Options,
	load ImageLoader,
) (*Layout, error) {
	if !options.Overlap {
		return Plan(items, options)
	}

	grouping, err := findOverlapGrouping(items, trimByID, options, load)
	if err != nil {
		return nil, err
	}

	if len(grouping.zonesByID) == 0 {
		return Plan(items, options)
	}

	baseline, baselineErr := Plan(items, options)
	candidate, candidateErr := Plan(grouping.items, options)
	if candidateErr == nil {
		expandOverlapLayout(candidate, grouping, trimByID, options)
		if overlapLayoutIsBetter(candidate, baseline, grouping.items, items, options) {
			return candidate, nil
		}
	}

	if baselineErr == nil {
		return baseline, nil
	}

	if candidateErr != nil && !errors.Is(candidateErr, ErrPlacementFailed) {
		return nil, candidateErr
	}

	return nil, baselineErr
}

// planImagePages preserves the baseline unless zone pages improve the documented score.
func planImagePages(
	items []Item,
	trimByID map[string]trimMetadata,
	options Options,
	load ImageLoader,
) ([]Layout, error) {
	if !options.Overlap {
		return PlanPages(items, options)
	}

	grouping, err := findOverlapGrouping(items, trimByID, options, load)
	if err != nil {
		return nil, err
	}

	if len(grouping.zonesByID) == 0 {
		return PlanPages(items, options)
	}

	baseline, baselineErr := PlanPages(items, options)
	candidate, candidateErr := PlanPages(grouping.items, options)
	if candidateErr == nil {
		for pageIndex := range candidate {
			expandOverlapLayout(&candidate[pageIndex], grouping, trimByID, options)
		}

		better, compareErr := overlapPagesAreBetter(candidate, baseline, grouping.items, items)
		if compareErr == nil && better {
			return candidate, nil
		}
	}

	if baselineErr == nil {
		return baseline, nil
	}

	if candidateErr != nil && !errors.Is(candidateErr, ErrPlacementFailed) {
		return nil, candidateErr
	}

	return nil, baselineErr
}

// overlapLayoutIsBetter applies MaxRects scoring, then a local-area tie-break.
func overlapLayoutIsBetter(
	candidate, baseline *Layout,
	candidateItems, baselineItems []Item,
	options Options,
) bool {
	if candidate == nil {
		return false
	}
	if baseline == nil {
		return true
	}

	candidateScoreValue := candidateScore(candidate.Width, candidate.Height, options.AspectPenalty)
	baselineScoreValue := candidateScore(baseline.Width, baseline.Height, options.AspectPenalty)
	if isBetterCandidate(
		candidate.Width,
		candidate.Height,
		candidateScoreValue,
		baseline.Width,
		baseline.Height,
		baselineScoreValue,
		options.PreferHeight,
	) {
		return true
	}

	if candidate.Width != baseline.Width || candidate.Height != baseline.Height ||
		candidateScoreValue != baselineScoreValue {
		return false
	}

	candidateArea, candidateOK := itemAreaSum(candidateItems)
	baselineArea, baselineOK := itemAreaSum(baselineItems)
	return candidateOK && baselineOK && candidateArea < baselineArea
}

// overlapPagesAreBetter compares page count, atlas area, then zone-item area.
func overlapPagesAreBetter(
	candidate, baseline []Layout,
	candidateItems, baselineItems []Item,
) (bool, error) {
	if len(baseline) == 0 && len(candidate) > 0 {
		return true, nil
	}

	if len(candidate) != len(baseline) {
		return len(candidate) < len(baseline), nil
	}

	area := func(layouts []Layout) (int64, error) {
		var total int64
		for i := range layouts {
			pageArea, err := checkedArea(int64(layouts[i].Width), int64(layouts[i].Height))
			if err != nil {
				return 0, err
			}

			total, err = checkedAdd(total, pageArea)
			if err != nil {
				return 0, err
			}
		}

		return total, nil
	}

	candidateArea, err := area(candidate)
	if err != nil {
		return false, fmt.Errorf("candidate page area: %w", err)
	}

	baselineArea, err := area(baseline)
	if err != nil {
		return false, fmt.Errorf("baseline page area: %w", err)
	}
	if candidateArea != baselineArea {
		return candidateArea < baselineArea, nil
	}

	candidateItemArea, candidateOK := itemAreaSum(candidateItems)
	baselineItemArea, baselineOK := itemAreaSum(baselineItems)
	return candidateOK && baselineOK && candidateItemArea < baselineItemArea, nil
}

// itemAreaSum returns the checked sum of item content areas.
func itemAreaSum(items []Item) (int64, bool) {
	var total int64
	for _, item := range items {
		area, err := checkedArea(int64(item.Width), int64(item.Height))
		if err != nil || total > math.MaxInt64-area {
			return 0, false
		}

		total += area
	}

	return total, true
}
