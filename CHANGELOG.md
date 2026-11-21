<!-- markdownlint-disable MD024 -->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## Unreleased

### Added

* Caller-configurable RGBA output-buffer limits for rendering layouts.
* Optional edge extrusion into reserved sprite padding.
* Multi-page atlas planning and packing, including sequential callback builds
  and on-demand source loading for single- and multi-page rendering.
* Optional transparent-border and exact color-key trimming
  with reversible source geometry, plus key-to-alpha conversion
  across single- and multi-page packing and build APIs.
* Optional exact pixel deduplication with explicit aliases
  across single- and multi-page packing and build APIs.
* Optional exact RGBA8 edge overlap zones that pack multiple compatible sprites
  as one padded block, with grid and Guillotine local solvers.
* Image-independent atlas and multi-page utilization, alias,
  rotation and trim statistics reports.

### Changed

* Improve planning throughput by sharing fixed-bin MaxRects insertion
  and reducing redundant size checks and free-space pruning.
* Speed up clockwise RGBA sprite rotation while preserving source bounds.
* Lowered the minimum Go version to 1.22.

### Fixed

* Reject typed-nil images, invalid placements,
  and overflowing atlas geometry before rendering.
* Reject invalid non-finite or negative aspect penalties.
* Fix MaxRects best-area scoring for large free rectangles.
* Consider rectangular atlas candidates up to the configured bounds
  when `ForceSquare` is disabled.
* Preserve requested padding in layout metadata when extrusion is disabled.
* Preserve non-zero source image bounds when rendering sprites.

## [0.1.0][] - 2026-03-04

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/atlasforge/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
