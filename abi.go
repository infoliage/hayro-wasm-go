package hayro

// Low-level calls into a Document's wasm module instance — the only place
// in this package that speaks in terms of pointers rather than Go values.
// Every exported method here has a 1:1 counterpart documented in
// hayro-wasm-bridge's README; see that doc for the exact wire format each
// one reads/writes.

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

func result_to_uint32(res []uint64, err error) (uint32, error) {
	if err != nil {
		return 0, err
	}
	return api.DecodeU32(res[0]), err
}

// result_to_int32 converts signed results
func result_to_int32(res []uint64, err error) (int32, error) {
	if err != nil {
		return 0, err
	}
	return api.DecodeI32(res[0]), err
}

// call invokes the named export and returns its raw uint64 results. wazero
// represents every wasm value type (i32/i64/f32/f64) as a bit-reinterpreted
// uint64, so callers are responsible for casting to/from the right width.
func (d *Document) call(ctx context.Context, name string, args ...uint64) ([]uint64, error) {
	fn := d.mod.ExportedFunction(name)
	if fn == nil {
		return nil, fmt.Errorf("hayro: wasm module has no export %q (built from an incompatible hayro-wasm-bridge version?)", name)
	}
	res, err := fn.Call(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("hayro: calling %q: %w", name, err)
	}
	return res, nil
}

func (d *Document) allocPDF(ctx context.Context, size uint32) (uint32, error) {
	return result_to_uint32(d.call(ctx, "alloc_pdf", uint64(size)))
}

func (d *Document) freePDF(ctx context.Context, ptr, size uint32) error {
	_, err := d.call(ctx, "free_pdf", uint64(ptr), uint64(size))
	return err
}

func (d *Document) allocRenderSettings(ctx context.Context, size uint32) (uint32, error) {
	return result_to_uint32(d.call(ctx, "alloc_render_settings", uint64(size)))
}

func (d *Document) freeRenderSettings(ctx context.Context, ptr, size uint32) error {
	_, err := d.call(ctx, "free_render_settings", uint64(ptr), uint64(size))
	return err
}

func (d *Document) allocInterpreterSettings(ctx context.Context, size uint32) (uint32, error) {
	return result_to_uint32(d.call(ctx, "alloc_interpreter_settings", uint64(size)))
}

func (d *Document) freeInterpreterSettings(ctx context.Context, ptr, size uint32) error {
	_, err := d.call(ctx, "free_interpreter_settings", uint64(ptr), uint64(size))
	return err
}

func (d *Document) allocU32(ctx context.Context) (uint32, error) {
	return result_to_uint32(d.call(ctx, "alloc_u32"))
}

func (d *Document) freeU32(ctx context.Context, ptr uint32) error {
	_, err := d.call(ctx, "free_u32", uint64(ptr))
	return err
}

func (d *Document) freePixels(ctx context.Context, ptr, width, height uint32) error {
	_, err := d.call(ctx, "free_pixels", uint64(ptr), uint64(width), uint64(height))
	return err
}

func (d *Document) pageCount(ctx context.Context, pdfPtr, pdfLen uint32) (int32, error) {
	return result_to_int32(d.call(ctx, "page_count", uint64(pdfPtr), uint64(pdfLen)))
}

func (d *Document) renderPage(ctx context.Context, pdfPtr, pdfLen, pageNumber, interpSettingsPtr, interpSettingsLen, renderSettingsPtr, renderSettingsLen, heightPtr, widthPtr uint32) (uint32, error) {
	return result_to_uint32(
		d.call(ctx, "render_page",
			uint64(pdfPtr), uint64(pdfLen),
			uint64(pageNumber),
			uint64(interpSettingsPtr), uint64(interpSettingsLen),
			uint64(renderSettingsPtr), uint64(renderSettingsLen),
			uint64(heightPtr), uint64(widthPtr),
		))
}
