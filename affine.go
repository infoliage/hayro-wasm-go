// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro_wasm_go

import "math"

// Affine is a 2D affine transform, [a, b, c, d, e, f], mapping a point
// (x, y) to
//
//	x' = a*x + c*y + e
//	y' = b*x + d*y + f
//
// The coefficient order matches kurbo::Affine, the type hayro itself
// uses, and hayro-wasm-bridge's wire format — see PixmapSettings.Transform
// for the coordinate spaces it maps between.
type Affine [6]float64

// Identity is the transform that leaves every point where it is.
var Identity = Affine{1, 0, 0, 1, 0, 0}

// Scale returns a transform that scales by sx horizontally and sy
// vertically, about the origin.
func Scale(sx, sy float64) Affine {
	return Affine{sx, 0, 0, sy, 0, 0}
}

// Translate returns a transform that moves every point by (dx, dy).
func Translate(dx, dy float64) Affine {
	return Affine{1, 0, 0, 1, dx, dy}
}

// Rotate returns a transform that rotates by radians about the origin.
// With y pointing down, as it does in PixmapSettings.Transform's spaces, a
// positive angle is clockwise.
func Rotate(radians float64) Affine {
	sin, cos := math.Sincos(radians)
	return Affine{cos, sin, -sin, cos, 0, 0}
}

// Mul returns the transform that applies o first and then a — the same
// order as kurbo's a * o.
func (a Affine) Mul(o Affine) Affine {
	return Affine{
		a[0]*o[0] + a[2]*o[1],
		a[1]*o[0] + a[3]*o[1],
		a[0]*o[2] + a[2]*o[3],
		a[1]*o[2] + a[3]*o[3],
		a[0]*o[4] + a[2]*o[5] + a[4],
		a[1]*o[4] + a[3]*o[5] + a[5],
	}
}

// Then returns the transform that applies a first and then o.
func (a Affine) Then(o Affine) Affine {
	return o.Mul(a)
}
