// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro

import (
	"encoding/json"
	"image/color"
)

// RenderSettings mirrors hayro::RenderSettings. Every field is a pointer;
// nil means "use hayro's default for it" — matching hayro-wasm-bridge's
// JSON wire format (see its schema/render-settings.schema.json), where an
// absent field means exactly the same thing. Unlike an earlier
// byte-packed version of this wire format, an explicit zero value (e.g.
// XScale pointing at 0.0) is no longer indistinguishable from "unset" — it
// is honored literally.
//
// Example:
//
//	hayro.RenderSettings{Width: new(uint16(800))}
type RenderSettings struct {
	// XScale and YScale are the horizontal/vertical scale factors applied
	// to the page's own point size. nil means "use hayro's default
	// (1.0)". An explicit 0 is honored literally, and (like any other
	// zero scale) produces a zero-area, and thus failing, render.
	XScale *float32
	YScale *float32

	// Width and Height override the rendered pixel dimensions directly.
	// nil means "auto" — derived from the page size and XScale/YScale.
	Width  *uint16
	Height *uint16

	// BackgroundColor uses straight (non-premultiplied) alpha, matching
	// its own type (color.NRGBA, not color.RGBA). nil means "use hayro's
	// default" (i.e. #00000000 — fully transparent black).
	BackgroundColor *color.NRGBA
}

// renderSettingsWire is RenderSettings' JSON shape on the wire — see
// hayro-wasm-bridge's schema/render-settings.schema.json for the
// authoritative description. A separate type from RenderSettings itself
// because color.NRGBA has no json tags of its own, and its field names
// (R, G, B, A) don't match the wire's lowercase r/g/b/a.
type renderSettingsWire struct {
	XScale  *float32  `json:"x_scale,omitempty"`
	YScale  *float32  `json:"y_scale,omitempty"`
	Width   *uint16   `json:"width,omitempty"`
	Height  *uint16   `json:"height,omitempty"`
	BgColor *rgbaWire `json:"bg_color,omitempty"`
}

type rgbaWire struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

func (s RenderSettings) encode() ([]byte, error) {
	wire := renderSettingsWire{
		XScale: s.XScale,
		YScale: s.YScale,
		Width:  s.Width,
		Height: s.Height,
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
