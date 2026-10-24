// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"fmt"
	"image"
	"image/color"
)

// colorKeyImageView applies exact key-to-alpha conversion without copying pixels.
type colorKeyImageView struct {
	source image.Image // source provides the original, unmodified pixels.
	key    color.RGBA  // key is the key color in the canonical premultiplied RGBA8 representation.
}

// validateColorKeyOptions rejects a present but inactive color-key configuration.
func validateColorKeyOptions(options *ColorKeyOptions) error {
	if options != nil && !options.Trim && !options.ToAlpha {
		return fmt.Errorf("%w: ColorKey requires Trim or ToAlpha", ErrInvalidOptions)
	}

	return nil
}

// canonicalRGBA8 converts one source pixel exactly as the RGBA renderer does.
func canonicalRGBA8(pixel color.Color) color.RGBA {
	return color.RGBAModel.Convert(pixel).(color.RGBA)
}

// canonicalColorKey converts the configured straight-alpha key to render bytes.
func canonicalColorKey(key color.NRGBA) color.RGBA {
	return canonicalRGBA8(key)
}

// canonicalImageRGBA8 reads one rendered RGBA8 pixel without interface allocations.
func canonicalImageRGBA8(source image.Image, x, y int) color.RGBA {
	if fast, ok := source.(rgbaAtImage); ok {
		return fast.RGBAAt(x, y)
	}

	switch img := source.(type) {
	case *image.NRGBA:
		return rgba8FromNRGBA(img.NRGBAAt(x, y))

	case *image.RGBA64:
		return rgba8FromRGBA64(img.RGBA64At(x, y))

	case *image.NRGBA64:
		return rgba8FromNRGBA64(img.NRGBA64At(x, y))

	case *image.Paletted:
		if len(img.Palette) == 0 {
			return color.RGBA{}
		}
		if !image.Pt(x, y).In(img.Rect) {
			return rgba8FromColor(img.Palette[0])
		}
		return rgba8FromColor(img.Palette[img.Pix[img.PixOffset(x, y)]])

	case *image.Gray:
		return rgba8FromGray(img.GrayAt(x, y))

	case *image.Gray16:
		return rgba8FromGray16(img.Gray16At(x, y))

	case *image.Alpha:
		return rgba8FromAlpha(img.AlphaAt(x, y))

	case *image.Alpha16:
		return rgba8FromAlpha16(img.Alpha16At(x, y))

	case *image.Uniform:
		return rgba8FromColor(img.C)

	case sourceImageView:
		return canonicalImageRGBA8(img.source, x, y)

	default:
		return rgba8FromColor(source.At(x, y))
	}
}

// rgba8FromNRGBA converts straight-alpha 8-bit pixels to canonical RGBA8.
func rgba8FromNRGBA(pixel color.NRGBA) color.RGBA {
	r, g, b, a := pixel.RGBA()
	return color.RGBA{
		R: uint8((r >> 8) & 0xff),
		G: uint8((g >> 8) & 0xff),
		B: uint8((b >> 8) & 0xff),
		A: uint8((a >> 8) & 0xff),
	}
}

// rgba8FromRGBA64 converts premultiplied 16-bit pixels to canonical RGBA8.
func rgba8FromRGBA64(pixel color.RGBA64) color.RGBA {
	return color.RGBA{
		R: uint8(pixel.R >> 8),
		G: uint8(pixel.G >> 8),
		B: uint8(pixel.B >> 8),
		A: uint8(pixel.A >> 8),
	}
}

// rgba8FromNRGBA64 converts straight-alpha 16-bit pixels to canonical RGBA8.
func rgba8FromNRGBA64(pixel color.NRGBA64) color.RGBA {
	r, g, b, a := pixel.RGBA()
	return color.RGBA{
		R: uint8((r >> 8) & 0xff),
		G: uint8((g >> 8) & 0xff),
		B: uint8((b >> 8) & 0xff),
		A: uint8((a >> 8) & 0xff),
	}
}

// rgba8FromGray converts an 8-bit gray pixel to opaque RGBA8.
func rgba8FromGray(pixel color.Gray) color.RGBA {
	return color.RGBA{
		R: pixel.Y,
		G: pixel.Y,
		B: pixel.Y,
		A: 255,
	}
}

// rgba8FromGray16 converts a 16-bit gray pixel to opaque RGBA8.
func rgba8FromGray16(pixel color.Gray16) color.RGBA {
	y := uint8(pixel.Y >> 8)
	return color.RGBA{
		R: y,
		G: y,
		B: y,
		A: 255,
	}
}

// rgba8FromAlpha converts an 8-bit alpha mask to the renderer's RGBA8 form.
func rgba8FromAlpha(pixel color.Alpha) color.RGBA {
	return color.RGBA{
		R: pixel.A,
		G: pixel.A,
		B: pixel.A,
		A: pixel.A,
	}
}

// rgba8FromAlpha16 converts a 16-bit alpha mask to the renderer's RGBA8 form.
func rgba8FromAlpha16(pixel color.Alpha16) color.RGBA {
	a := uint8(pixel.A >> 8)
	return color.RGBA{
		R: a,
		G: a,
		B: a,
		A: a,
	}
}

// rgba8FromColor converts one color through the renderer's RGBA component values.
func rgba8FromColor(pixel color.Color) color.RGBA {
	r, g, b, a := pixel.RGBA()
	return color.RGBA{
		R: uint8((r >> 8) & 0xff),
		G: uint8((g >> 8) & 0xff),
		B: uint8((b >> 8) & 0xff),
		A: uint8((a >> 8) & 0xff),
	}
}

// colorKeyView returns a lazy pixel transform when alpha conversion is enabled.
func colorKeyView(source image.Image, options *ColorKeyOptions) image.Image {
	if options == nil || !options.ToAlpha {
		return source
	}

	return colorKeyImageView{source: source, key: canonicalColorKey(options.Color)}
}

// Bounds reports the unchanged source bounds.
func (view colorKeyImageView) Bounds() image.Rectangle {
	return view.source.Bounds()
}

// ColorModel reports canonical premultiplied RGBA pixels.
func (colorKeyImageView) ColorModel() color.Model {
	return color.RGBAModel
}

// At returns a canonical pixel with exact key matches replaced by transparent zero.
func (view colorKeyImageView) At(x, y int) color.Color {
	return view.RGBAAt(x, y)
}

// RGBAAt applies the key transform without allocating a color conversion object.
func (view colorKeyImageView) RGBAAt(x, y int) color.RGBA {
	pixel := canonicalImageRGBA8(view.source, x, y)
	if pixel == view.key {
		return color.RGBA{}
	}

	return pixel
}
