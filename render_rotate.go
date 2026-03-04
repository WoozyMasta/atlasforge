// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/atlasforge

package atlasforge

import (
	"image"
	"image/color"
)

// rgbaAtImage is an image that can return non-allocating RGBA pixels.
type rgbaAtImage interface {
	Bounds() image.Rectangle
	RGBAAt(x, y int) color.RGBA
}

// drawRotated90Into draws src rotated 90 degrees clockwise directly into dst.
func drawRotated90Into(dst *image.RGBA, dstX, dstY int, src image.Image, srcW, srcH int) {
	if fast, ok := src.(rgbaAtImage); ok {
		drawRotated90RGBAAt(dst, dstX, dstY, fast, srcW, srcH)
		return
	}

	drawRotated90Generic(dst, dstX, dstY, src, srcW, srcH)
}

// drawRotated90RGBAAt draws rotated data using fast RGBAAt access.
func drawRotated90RGBAAt(dst *image.RGBA, dstX, dstY int, src rgbaAtImage, srcW, srcH int) {
	srcBounds := src.Bounds()
	dstMin := dst.Rect.Min
	dstStride := dst.Stride
	dstPix := dst.Pix

	for sy := range srcH {
		srcY := srcBounds.Min.Y + sy
		dstXPos := dstX + srcH - 1 - sy
		dstColOffset := (dstXPos - dstMin.X) * 4

		for sx := range srcW {
			c := src.RGBAAt(srcBounds.Min.X+sx, srcY)
			dstYPos := dstY + sx
			di := (dstYPos-dstMin.Y)*dstStride + dstColOffset
			dstPix[di+0] = c.R
			dstPix[di+1] = c.G
			dstPix[di+2] = c.B
			dstPix[di+3] = c.A
		}
	}
}

// drawRotated90Generic draws rotated data for generic image.Image sources.
func drawRotated90Generic(dst *image.RGBA, dstX, dstY int, src image.Image, srcW, srcH int) {
	srcBounds := src.Bounds()
	for sy := range srcH {
		for sx := range srcW {
			c := color.RGBAModel.Convert(src.At(srcBounds.Min.X+sx, srcBounds.Min.Y+sy)).(color.RGBA)
			dst.SetRGBA(dstX+srcH-1-sy, dstY+sx, c)
		}
	}
}
