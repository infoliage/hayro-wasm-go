package hayro

import (
	"encoding/binary"
	"image/color"
	"math"
)

// Byte lengths of the settings blobs, matching hayro-wasm-bridge's
// RENDER_SETTINGS_LEN/INTERPRETER_SETTINGS_LEN.
const (
	renderSettingsLen      = 16
	interpreterSettingsLen = 1
)

// RenderSettings mirrors hayro::RenderSettings. The zero value means "use
// hayro's defaults" for every field — this matches hayro-wasm-bridge's
// wire format, where an all-zero blob is documented to behave exactly like
// RenderSettings::default(), so RenderSettings{} is always a valid,
// meaningful value here.
type RenderSettings struct {
	// XScale and YScale are the horizontal/vertical scale factors applied
	// to the page's own point size. 0 means "use hayro's default (1.0)" —
	// an explicit 0 would always produce a zero-area, and thus failing,
	// render anyway, so nothing is lost by giving 0 this meaning instead.
	XScale float32
	YScale float32

	// Width and Height override the rendered pixel dimensions directly.
	// 0 means "auto" — derived from the page size and XScale/YScale.
	Width  uint16
	Height uint16

	// BackgroundColor uses straight (non-premultiplied) alpha, matching
	// its own type (color.NRGBA, not color.RGBA). The zero value, fully
	// transparent black, is hayro's actual default.
	BackgroundColor color.NRGBA
}

func (s RenderSettings) encode() [renderSettingsLen]byte {
	var blob [renderSettingsLen]byte
	binary.LittleEndian.PutUint32(blob[0:4], math.Float32bits(s.XScale))
	binary.LittleEndian.PutUint32(blob[4:8], math.Float32bits(s.YScale))
	binary.LittleEndian.PutUint16(blob[8:10], s.Width)
	binary.LittleEndian.PutUint16(blob[10:12], s.Height)
	blob[12] = s.BackgroundColor.R
	blob[13] = s.BackgroundColor.G
	blob[14] = s.BackgroundColor.B
	blob[15] = s.BackgroundColor.A
	return blob
}

// InterpreterSettings mirrors the subset of
// hayro_interpret::InterpreterSettings that hayro-wasm-bridge exposes.
// Its font_resolver/cmap_resolver/warning_sink fields are Rust closures
// with no plain-bytes representation, so hayro-wasm-bridge can't expose
// them over this ABI, and neither does this package — see
// hayro-wasm-bridge's README.
type InterpreterSettings struct {
	// RenderAnnotations is a tri-state, matching the wire format's "0 =
	// default, 1 = enabled, anything else = disabled": nil means "use
	// hayro's default".
	RenderAnnotations *bool
}

func (s InterpreterSettings) encode() [interpreterSettingsLen]byte {
	var b byte
	if s.RenderAnnotations != nil {
		if *s.RenderAnnotations {
			b = 1
		} else {
			b = 2
		}
	}
	return [interpreterSettingsLen]byte{b}
}
