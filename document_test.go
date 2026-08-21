package hayro

import (
	"context"
	"errors"
	"image/color"
	"os"
	"testing"
)

// testEngine is shared across every test in this package — compiling the
// wasm module is the expensive part of an Engine (see Engine's doc
// comment), so tests share one rather than paying that per test.
var testEngine *Engine

func TestMain(m *testing.M) {
	ctx := context.Background()
	e, err := NewEngine(ctx)
	if err != nil {
		panic(err)
	}
	testEngine = e

	code := m.Run()

	if err := testEngine.Close(ctx); err != nil {
		panic(err)
	}
	os.Exit(code)
}

// A single 200x100pt page with "Hello World" in Helvetica, ported from
// hayro-wasm-bridge's own MINIMAL_PDF test fixture (src/tests.rs) — see
// that file for why the xref table is a deliberate stub.
const minimalPDF = `%PDF-1.1
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 58 >>
stream
BT /F1 24 Tf 20 40 Td (Hello World) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f
trailer
<< /Size 6 /Root 1 0 R >>
startxref
0
%%EOF`

func TestOpenAndPageCount(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	if got := doc.PageCount(); got != 1 {
		t.Fatalf("PageCount() = %d, want 1", got)
	}
}

func TestOpenInvalidPDF(t *testing.T) {
	ctx := context.Background()
	_, err := testEngine.Open(ctx, []byte("not a pdf"))
	if !errors.Is(err, ErrInvalidPDF) {
		t.Fatalf("Open() error = %v, want ErrInvalidPDF", err)
	}
}

func TestRenderDefaults(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	img, err := doc.Render(ctx, 1, nil, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if img.Rect.Dx() != 200 || img.Rect.Dy() != 100 {
		t.Fatalf("Render() size = %dx%d, want 200x100", img.Rect.Dx(), img.Rect.Dy())
	}

	// Corners are reliably background — MINIMAL_PDF's text sits away from
	// every edge (see hayro-wasm-bridge/src/tests.rs).
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner pixel not transparent: alpha = %d", a)
	}
}

func TestRenderPageOutOfRange(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	if _, err := doc.Render(ctx, 0, nil, nil); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("Render(0) error = %v, want ErrPageOutOfRange", err)
	}
	if _, err := doc.Render(ctx, 2, nil, nil); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("Render(2) error = %v, want ErrPageOutOfRange", err)
	}
}

func TestRenderWithSettings(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	img, err := doc.Render(ctx, 1, &RenderSettings{Width: new(uint16(100)), Height: new(uint16(50))}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if img.Rect.Dx() != 100 || img.Rect.Dy() != 50 {
		t.Fatalf("Render() size = %dx%d, want 100x50", img.Rect.Dx(), img.Rect.Dy())
	}
}

func TestRenderExplicitZeroScaleIsZeroAreaNotDefault(t *testing.T) {
	// Confirms the semantic this whole JSON wire format exists for: an
	// explicit 0 must not be reinterpreted as "use hayro's default", the
	// way it was in an earlier byte-packed version of this wire format —
	// see hayro-wasm-bridge's schema/render-settings.schema.json.
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	_, err = doc.Render(ctx, 1, &RenderSettings{XScale: new(float32(0)), YScale: new(float32(0))}, nil)
	if !errors.Is(err, ErrRenderFailed) {
		t.Fatalf("Render() error = %v, want ErrRenderFailed", err)
	}
}

func TestRenderBackgroundColorOverride(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	bg := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	img, err := doc.Render(ctx, 1, &RenderSettings{BackgroundColor: &bg}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Top-left corner is well outside MINIMAL_PDF's text (see its doc
	// comment), so it's guaranteed to be pure background.
	if got := img.NRGBAAt(0, 0); got != bg {
		t.Errorf("corner pixel = %+v, want %+v", got, bg)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := doc.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := doc.Close(ctx); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := doc.Render(ctx, 1, nil, nil); !errors.Is(err, ErrClosed) {
		t.Errorf("Render() after Close error = %v, want ErrClosed", err)
	}
}
