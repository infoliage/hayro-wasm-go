// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro_wasm_go

import (
	"math"
	"testing"
)

// apply maps the point (x, y) through a.
func apply(a Affine, x, y float64) (float64, float64) {
	return a[0]*x + a[2]*y + a[4], a[1]*x + a[3]*y + a[5]
}

func wantPoint(t *testing.T, name string, a Affine, x, y, wantX, wantY float64) {
	t.Helper()
	gotX, gotY := apply(a, x, y)
	if math.Abs(gotX-wantX) > 1e-9 || math.Abs(gotY-wantY) > 1e-9 {
		t.Errorf("%s maps (%v, %v) to (%v, %v), want (%v, %v)", name, x, y, gotX, gotY, wantX, wantY)
	}
}

func TestAffineConstructors(t *testing.T) {
	wantPoint(t, "Identity", Identity, 3, 4, 3, 4)
	wantPoint(t, "Scale(2, 3)", Scale(2, 3), 3, 4, 6, 12)
	wantPoint(t, "Translate(10, 20)", Translate(10, 20), 3, 4, 13, 24)
	// A quarter turn takes the x axis onto the y axis, which with y
	// pointing down is clockwise.
	wantPoint(t, "Rotate(pi/2)", Rotate(math.Pi/2), 1, 0, 0, 1)
	wantPoint(t, "Rotate(pi/2)", Rotate(math.Pi/2), 0, 1, -1, 0)
}

func TestAffineMulAppliesRightOperandFirst(t *testing.T) {
	// Scale then translate: (3, 4) -> (6, 8) -> (16, 28).
	wantPoint(t, "Translate.Mul(Scale)", Translate(10, 20).Mul(Scale(2, 2)), 3, 4, 16, 28)
	// Translate then scale: (3, 4) -> (13, 24) -> (26, 48).
	wantPoint(t, "Scale.Mul(Translate)", Scale(2, 2).Mul(Translate(10, 20)), 3, 4, 26, 48)
}

func TestAffineThenAppliesReceiverFirst(t *testing.T) {
	wantPoint(t, "Scale.Then(Translate)", Scale(2, 2).Then(Translate(10, 20)), 3, 4, 16, 28)
	if got, want := Scale(2, 2).Then(Translate(10, 20)), Translate(10, 20).Mul(Scale(2, 2)); got != want {
		t.Errorf("a.Then(b) = %v, want b.Mul(a) = %v", got, want)
	}
}

func TestAffineEncodesInWireOrder(t *testing.T) {
	width, height := uint16(1), uint16(1)
	transform := Affine{1, 2, 3, 4, 5, 6}
	blob, err := PixmapSettings{Width: &width, Height: &height, Transform: &transform}.encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got, want := string(blob), `{"width":1,"height":1,"transform":[1,2,3,4,5,6]}`; got != want {
		t.Errorf("encode() = %s, want %s", got, want)
	}
}
