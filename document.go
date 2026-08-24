package hayro

import (
	"context"
	"fmt"
	"image"
	"math/bits"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

// Document is an open PDF, backed by its own wasm module instance (its own
// linear memory, its own allocator state) obtained from an Engine's Open.
//
// hayro-wasm-bridge's module has no interior locking (see its README's
// Status section), so a Document serializes every call against its module
// instance itself — it's safe to call a single Document's methods from
// multiple goroutines, but those calls won't run in parallel. For actual
// parallel rendering, open the same PDF bytes as separate Documents (one
// per goroutine, via the same Engine); each gets its own module instance
// and can proceed independently.
type Document struct {
	mu       sync.Mutex
	mod      api.Module
	pdfPtr   uint32
	pdfLen   uint32
	numPages uint
	closed   bool
}

// init loads pdf into the module's memory and validates it parses. It
// runs during Engine.Open, before d is returned to the caller, so it
// doesn't need d.mu.
func (d *Document) init(ctx context.Context, pdf []byte) error {
	ptr, err := d.allocPDF(ctx, uint32(len(pdf)))
	if err != nil {
		return err
	}
	if ptr == 0 && len(pdf) != 0 {
		return fmt.Errorf("hayro: wasm module failed to allocate %d bytes for the PDF", len(pdf))
	}
	if len(pdf) > 0 && !d.mod.Memory().Write(ptr, pdf) {
		return fmt.Errorf("hayro: writing PDF bytes into wasm memory")
	}
	d.pdfPtr, d.pdfLen = ptr, uint32(len(pdf))

	pageCount, err := d.pageCount(ctx, d.pdfPtr, d.pdfLen)
	if err != nil {
		return err
	}
	if pageCount < 0 {
		return ErrInvalidPDF
	}
	d.numPages = uint(pageCount)
	return nil
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() uint {
	return d.numPages
}

// Render rasterizes one page (pageNumber is 1-based, in [1, PageCount()])
// to an RGBA image. render and interpreter each independently select a
// hayro settings struct for this render — pass nil for either to use
// hayro's defaults.
func (d *Document) Render(ctx context.Context, pageNumber uint, render *RenderSettings, interpreter *InterpreterSettings) (*image.NRGBA, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil, ErrClosed
	}
	if pageNumber < 1 || pageNumber > d.numPages {
		return nil, ErrPageOutOfRange
	}

	var renderPtr, renderLen, interpreterPtr, interpreterLen uint32
	if render != nil {
		blob, err := render.encode()
		if err != nil {
			return nil, fmt.Errorf("hayro: encoding render settings: %w", err)
		}
		ptr, err := d.allocRenderSettings(ctx, uint32(len(blob)))
		if err != nil {
			return nil, err
		}
		defer func() { _ = d.freeRenderSettings(ctx, ptr, uint32(len(blob))) }()
		if !d.mod.Memory().Write(ptr, blob) {
			return nil, fmt.Errorf("hayro: writing render settings into wasm memory")
		}
		renderPtr, renderLen = ptr, uint32(len(blob))
	}
	if interpreter != nil {
		blob, err := interpreter.encode()
		if err != nil {
			return nil, fmt.Errorf("hayro: encoding interpreter settings: %w", err)
		}
		ptr, err := d.allocInterpreterSettings(ctx, uint32(len(blob)))
		if err != nil {
			return nil, err
		}
		defer func() { _ = d.freeInterpreterSettings(ctx, ptr, uint32(len(blob))) }()
		if !d.mod.Memory().Write(ptr, blob) {
			return nil, fmt.Errorf("hayro: writing interpreter settings into wasm memory")
		}
		interpreterPtr, interpreterLen = ptr, uint32(len(blob))
	}

	widthPtr, err := d.allocU32(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.freeU32(ctx, widthPtr) }()
	heightPtr, err := d.allocU32(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.freeU32(ctx, heightPtr) }()
	resultPtr, err := d.renderPage(ctx, d.pdfPtr, d.pdfLen, uint32(pageNumber),
		interpreterPtr, interpreterLen,
		renderPtr, renderLen,
		widthPtr, heightPtr)

	if err != nil {
		return nil, err
	}
	if resultPtr == 0 {
		// pageNumber was already validated above, and this package's own
		// encode() always produces JSON matching hayro-wasm-bridge's
		// schema, so the only failure mode left in practice is a
		// zero-area render — but the wasm side treats malformed settings
		// JSON as the same null-pointer failure, so this covers that too
		// (it would indicate a hayro-wasm-go bug, not a caller mistake).
		return nil, ErrRenderFailed
	}

	width, ok := d.mod.Memory().ReadUint32Le(widthPtr)
	if !ok {
		return nil, fmt.Errorf("hayro: reading rendered width from wasm memory")
	}
	height, ok := d.mod.Memory().ReadUint32Le(heightPtr)
	if !ok {
		return nil, fmt.Errorf("hayro: reading rendered height from wasm memory")
	}
	defer func() { _ = d.freePixels(ctx, resultPtr, width, height) }()

	// width*height*4 as uint32 arithmetic can overflow.  bits.Mul32,
	// chained, catches an overflow at either step.
	hi, wh := bits.Mul32(width, height)
	hi2, pixLen := bits.Mul32(wh, 4)
	if hi != 0 || hi2 != 0 {
		return nil, fmt.Errorf("hayro: rendered pixel buffer too large to read (%dx%d)", width, height)
	}

	pix, ok := d.mod.Memory().Read(resultPtr, pixLen)
	if !ok {
		return nil, fmt.Errorf("hayro: reading rendered pixels from wasm memory")
	}
	// pix aliases the module's own linear memory, about to be freed above
	// (and liable to be overwritten by the module's allocator on the next
	// call regardless) — the returned image needs its own copy.
	owned := make([]byte, len(pix))
	copy(owned, pix)

	return &image.NRGBA{
		Pix:    owned,
		Stride: int(width) * 4,
		Rect:   image.Rect(0, 0, int(width), int(height)),
	}, nil
}

// Close releases the Document's wasm module instance. It is safe to call
// more than once.
func (d *Document) Close(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	// Free the pdf buffer explicitly rather than relying on mod.Close
	// discarding all of the instance's linear memory — that's true today,
	// but freeing what was allocated, symmetrically, doesn't depend on it
	// staying true (e.g. if Close ever pooled/reused instances instead of
	// always tearing them down). Best-effort: a failure here shouldn't
	// stop the module instance itself from being closed below.
	_ = d.freePDF(ctx, d.pdfPtr, d.pdfLen)
	return d.mod.Close(ctx)
}
