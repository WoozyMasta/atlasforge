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

* Optional edge extrusion into reserved sprite padding.

### Changed

* Share fixed-bin MaxRects insertion between single- and multi-page planning.
* Speed up clockwise RGBA sprite rotation while preserving source bounds.
* Reduce redundant atlas size feasibility checks during planning.
* Reduce MaxRects free-space pruning overhead on large atlas plans.
* Lowered the minimum Go version to 1.22.

### Fixed

* Reject typed-nil source images instead of panicking.
* Reject invalid non-finite or negative aspect penalties.
* Fix MaxRects best-area scoring for large free rectangles.
* Reject overflowing atlas dimensions and padded area calculations.
* Consider valid atlas width/height candidates independently
  and respect maximum bounds.
* Preserve non-zero source image bounds when rendering sprites.
* Reject invalid atlas placements before allocating or rendering.

## [0.1.0][] - 2026-03-04

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/atlasforge/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
