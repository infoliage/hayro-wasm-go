// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro_wasm_go

import (
	"encoding/json"
	"errors"
	"image/color"
)

// RenderSettings mirrors hayro::RenderSettings — see hayro-wasm-bridge's
// schema/render-settings.schema.json.
type RenderSettings struct {
	// ForceImageInterpolation: true draws every image with bilinear
	// interpolation, whatever the PDF asks for. nil means "use hayro's
	// default" (false).
	ForceImageInterpolation *bool `json:"force_image_interpolation,omitempty"`
}

func (s RenderSettings) encode() ([]byte, error) {
	return json.Marshal(s)
}

// PixmapSettings describes the canvas a page is drawn onto: what hayro
// splits between its own PixmapSettings and the context/transform
// arguments of hayro::render_into. Every field is a pointer; nil means
// "use the default for it" — matching hayro-wasm-bridge's JSON wire format
// (see its schema/pixmap-settings.schema.json), where an absent field
// means exactly the same thing.
//
// Example, rendering a page at twice its natural size (page being its
// PageInfo):
//
//	hayro.PixmapSettings{
//		Width:     new(uint16(page.Width * 2)),
//		Height:    new(uint16(page.Height * 2)),
//		Transform: new(hayro.Scale(2, 2)),
//	}
type PixmapSettings struct {
	// Width and Height are the canvas size in pixels. nil means the
	// page's own size (PageInfo's Width/Height, truncated to whole
	// pixels), which is only allowed when Transform is nil too. A zero
	// Width or Height fails the render.
	Width  *uint16
	Height *uint16

	// Transform maps upright page space — points, origin at the page's
	// top-left corner, y pointing down, with the page's rotation and crop
	// box already applied — to canvas pixels. nil means Identity, i.e.
	// one pixel per point. Width and Height must both be set alongside
	// it: a transform has no canvas size that is obviously right for it.
	// Whatever the transform, nothing outside the page's crop box is
	// drawn.
	Transform *Affine

	// BackgroundColor is what the canvas is cleared to before the page
	// is drawn. It uses straight (non-premultiplied) alpha, matching its
	// own type (color.NRGBA, not color.RGBA). nil means #00000000 —
	// fully transparent black.
	BackgroundColor *color.NRGBA
}

// pixmapSettingsWire is PixmapSettings' JSON shape on the wire — see
// hayro-wasm-bridge's schema/pixmap-settings.schema.json for the
// authoritative description. A separate type from PixmapSettings itself
// because color.NRGBA has no json tags of its own, and its field names
// (R, G, B, A) don't match the wire's lowercase r/g/b/a.
type pixmapSettingsWire struct {
	Width     *uint16   `json:"width,omitempty"`
	Height    *uint16   `json:"height,omitempty"`
	Transform *Affine   `json:"transform,omitempty"`
	BgColor   *rgbaWire `json:"bg_color,omitempty"`
}

type rgbaWire struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

func (s PixmapSettings) encode() ([]byte, error) {
	if s.Transform != nil && (s.Width == nil || s.Height == nil) {
		return nil, errors.New("transform set without both width and height")
	}
	wire := pixmapSettingsWire{
		Width:     s.Width,
		Height:    s.Height,
		Transform: s.Transform,
	}
	if s.BackgroundColor != nil {
		wire.BgColor = &rgbaWire{
			R: s.BackgroundColor.R,
			G: s.BackgroundColor.G,
			B: s.BackgroundColor.B,
			A: s.BackgroundColor.A,
		}
	}
	return json.Marshal(wire)
}

// InterpreterSettings mirrors the subset of
// hayro_interpret::InterpreterSettings that hayro-wasm-bridge exposes. Its
// font_resolver/cmap_resolver/warning_sink fields are Rust closures with
// no JSON representation, so hayro-wasm-bridge can't expose them over this
// ABI, and neither does this package — see hayro-wasm-bridge's README.
type InterpreterSettings struct {
	// RenderAnnotations: nil means "use hayro's default".
	RenderAnnotations *bool `json:"render_annotations,omitempty"`
}

func (s InterpreterSettings) encode() ([]byte, error) {
	return json.Marshal(s)
}
