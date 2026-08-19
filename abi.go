package hayro

// Low-level calls into a Document's wasm module instance — the only place
// in this package that speaks in terms of pointers rather than Go values.
// Every exported method here has a 1:1 counterpart documented in
// hayro-wasm-bridge's README; see that doc for the exact wire format each
// one reads/writes.

import (
	"context"
	"fmt"
)

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

func (d *Document) call1(ctx context.Context, name string, args ...uint64) (uint32, error) {
	res, err := d.call(ctx, name, args...)
	if err != nil {
		return 0, err
	}
	return uint32(res[0]), nil
}

func (d *Document) allocPDF(ctx context.Context, size uint32) (uint32, error) {
	return d.call1(ctx, "alloc_pdf", uint64(size))
}

func (d *Document) freePDF(ctx context.Context, ptr, size uint32) error {
	_, err := d.call(ctx, "free_pdf", uint64(ptr), uint64(size))
	return err
}

func (d *Document) allocRenderSettings(ctx context.Context) (uint32, error) {
	return d.call1(ctx, "alloc_render_settings")
}

func (d *Document) freeRenderSettings(ctx context.Context, ptr uint32) error {
	_, err := d.call(ctx, "free_render_settings", uint64(ptr))
	return err
}

func (d *Document) allocInterpreterSettings(ctx context.Context) (uint32, error) {
	return d.call1(ctx, "alloc_interpreter_settings")
}

func (d *Document) freeInterpreterSettings(ctx context.Context, ptr uint32) error {
	_, err := d.call(ctx, "free_interpreter_settings", uint64(ptr))
	return err
}

func (d *Document) allocU32(ctx context.Context) (uint32, error) {
	return d.call1(ctx, "alloc_u32")
}

func (d *Document) freeU32(ctx context.Context, ptr uint32) error {
	_, err := d.call(ctx, "free_u32", uint64(ptr))
	return err
}

func (d *Document) freePixels(ctx context.Context, ptr, width, height uint32) error {
	_, err := d.call(ctx, "free_pixels", uint64(ptr), uint64(width), uint64(height))
	return err
}
