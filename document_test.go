// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro

import (
	"context"
	"errors"
	"image/color"
	"os"
	"testing"
	"time"
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

// Same page geometry as minimalPDF (a 200x100pt page), but with a
// /Rotate 90 entry and a full document information dictionary — ported
// from hayro-wasm-bridge's own PDF_ROTATED_WITH_METADATA test fixture
// (src/tests.rs). CreationDate carries an explicit +05'30' offset, ModDate
// a bare Z (both should round-trip as a zero-offset time.Time — see
// parseWireDate).
const rotatedWithMetadataPDF = `%PDF-1.5
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Rotate 90 /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
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
6 0 obj
<< /Title (My Title) /Author (Jane Doe) /Subject (A Subject) /Keywords (foo bar) /Creator (My Creator) /Producer (My Producer) /CreationDate (D:20200102030405+05'30') /ModDate (D:20210607080910Z) >>
endobj
xref
0 7
0000000000 65535 f
trailer
<< /Size 7 /Root 1 0 R /Info 6 0 R >>
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

	if got := doc.Info().PageCount; got != 1 {
		t.Fatalf("PageCount = %d, want 1", got)
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

	width := uint16(100)
	height := uint16(50)
	img, err := doc.Render(ctx, 1, &RenderSettings{Width: &width, Height: &height}, nil)
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

	xscale := float32(0)
	yscale := float32(0)
	_, err = doc.Render(ctx, 1, &RenderSettings{XScale: &xscale, YScale: &yscale}, nil)
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

func wantStrPtr(t *testing.T, field string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %q", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %q, want %q", field, *got, want)
	}
}

func TestDocumentInfoNoMetadataDict(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	info := doc.Info()
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if info.Version != "1.1" {
		t.Errorf("Version = %q, want %q", info.Version, "1.1")
	}
	for name, got := range map[string]*string{
		"Title": info.Title, "Author": info.Author, "Subject": info.Subject,
		"Keywords": info.Keywords, "Creator": info.Creator, "Producer": info.Producer,
	} {
		if got != nil {
			t.Errorf("%s = %q, want nil", name, *got)
		}
	}
	if info.CreationDate != nil {
		t.Errorf("CreationDate = %v, want nil", info.CreationDate)
	}
	if info.ModificationDate != nil {
		t.Errorf("ModificationDate = %v, want nil", info.ModificationDate)
	}
}

func TestDocumentInfoWithMetadataAndDates(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(rotatedWithMetadataPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	info := doc.Info()
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if info.Version != "1.5" {
		t.Errorf("Version = %q, want %q", info.Version, "1.5")
	}
	wantStrPtr(t, "Title", info.Title, "My Title")
	wantStrPtr(t, "Author", info.Author, "Jane Doe")
	wantStrPtr(t, "Subject", info.Subject, "A Subject")
	wantStrPtr(t, "Keywords", info.Keywords, "foo bar")
	wantStrPtr(t, "Creator", info.Creator, "My Creator")
	wantStrPtr(t, "Producer", info.Producer, "My Producer")

	wantCreation := time.Date(2020, 1, 2, 3, 4, 5, 0, time.FixedZone("", 5*3600+30*60))
	if info.CreationDate == nil || !info.CreationDate.Equal(wantCreation) {
		t.Errorf("CreationDate = %v, want %v", info.CreationDate, wantCreation)
	}

	// ModDate's bare "Z" round-trips through hayro-wasm-bridge as an
	// explicit "+00:00" (see DocumentInfo.CreationDate's doc comment) —
	// still the same instant either way.
	wantModification := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
	if info.ModificationDate == nil || !info.ModificationDate.Equal(wantModification) {
		t.Errorf("ModificationDate = %v, want %v", info.ModificationDate, wantModification)
	}
}

func TestPageInfoDefaults(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	info, err := doc.PageInfo(ctx, 1)
	if err != nil {
		t.Fatalf("PageInfo: %v", err)
	}
	if info.Width != 200 || info.Height != 100 {
		t.Errorf("Width/Height = %v/%v, want 200/100", info.Width, info.Height)
	}
	if info.Rotation != 0 {
		t.Errorf("Rotation = %d, want 0", info.Rotation)
	}
	wantBox := Rect{X0: 0, Y0: 0, X1: 200, Y1: 100}
	if info.MediaBox != wantBox {
		t.Errorf("MediaBox = %+v, want %+v", info.MediaBox, wantBox)
	}
	if info.CropBox != wantBox {
		t.Errorf("CropBox = %+v, want %+v", info.CropBox, wantBox)
	}
}

func TestPageInfoRotationSwapsRenderDimensionsButNotBoxes(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(rotatedWithMetadataPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	info, err := doc.PageInfo(ctx, 1)
	if err != nil {
		t.Fatalf("PageInfo: %v", err)
	}
	if info.Rotation != 90 {
		t.Errorf("Rotation = %d, want 90", info.Rotation)
	}
	// Width/Height (render dimensions) are swapped for a 90° rotation...
	if info.Width != 100 || info.Height != 200 {
		t.Errorf("Width/Height = %v/%v, want 100/200", info.Width, info.Height)
	}
	// ...but the raw media box is not.
	wantBox := Rect{X0: 0, Y0: 0, X1: 200, Y1: 100}
	if info.MediaBox != wantBox {
		t.Errorf("MediaBox = %+v, want %+v", info.MediaBox, wantBox)
	}
}

func TestPageInfoOutOfRange(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer doc.Close(ctx)

	if _, err := doc.PageInfo(ctx, 0); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("PageInfo(0) error = %v, want ErrPageOutOfRange", err)
	}
	if _, err := doc.PageInfo(ctx, 2); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("PageInfo(2) error = %v, want ErrPageOutOfRange", err)
	}
}

func TestPageInfoAfterCloseIsErrClosed(t *testing.T) {
	ctx := context.Background()
	doc, err := testEngine.Open(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := doc.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := doc.PageInfo(ctx, 1); !errors.Is(err, ErrClosed) {
		t.Errorf("PageInfo() after Close error = %v, want ErrClosed", err)
	}
}
