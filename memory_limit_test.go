// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro_wasm_go

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

// newTestEngineWithMemoryLimit returns an Engine whose wasm linear memory
// is hard-capped at limitPages (each page is 64KiB — see wazero's
// RuntimeConfig.WithMemoryLimitPages), for exercising the module's
// out-of-memory paths below. It's a fresh Engine rather than the package's
// shared testEngine, since the memory limit is exactly what these tests
// need to vary.
func newTestEngineWithMemoryLimit(t *testing.T, limitPages uint32) (*Engine, error) {
	t.Helper()
	ctx := context.Background()
	cfg := NewEngineConfig()
	cfg.RuntimeConfig = cfg.RuntimeConfig.WithMemoryLimitPages(limitPages)
	e, err := NewEngineWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if err := e.Close(ctx); err != nil {
			t.Errorf("Engine.Close: %v", err)
		}
	})
	return e, nil
}

// TestEngineCreationMemoryLimit tests that the engine fails to create
// cleanly when the memory limit is too low.
func TestEngineCreationMemoryLimit(t *testing.T) {
	if _, err := newTestEngineWithMemoryLimit(t, 1); err == nil {
		t.Fatal("NewEngineWithConfig with a 1-page memory limit succeeded, want an error")
	}
}

// TestPDFExceedsMemoryBudget tests that an allocation failure inside the
// wasm module fails cleanly.
func TestPDFExceedsMemoryBudget(t *testing.T) {
	ctx := context.Background()
	const limitPages = 64 // 4MiB: plenty to instantiate the module itself.
	e, err := newTestEngineWithMemoryLimit(t, limitPages)
	if err != nil {
		t.Fatalf("newTestEngineWithMemoryLimit: %v", err)
	}

	const wasmPageSize = 64 * 1024 // wazero's fixed wasm page size.
	budget := limitPages * wasmPageSize
	junk := bytes.Repeat([]byte{0}, budget*2) // twice the whole memory budget.

	_, err = e.OpenDocument(ctx, junk)
	if err == nil {
		t.Fatal("OpenDocument with an oversized PDF succeeded, want an error")
	}
	if errors.Is(err, ErrInvalidPDF) {
		t.Errorf("OpenDocument error = %v, want an allocation failure, not ErrInvalidPDF", err)
	}
	// alloc_pdf reports failure by returning null, not by trapping.
	if errors.Is(err, ErrCrashed) {
		t.Errorf("OpenDocument error = %v, want a clean allocation failure, not ErrCrashed", err)
	}
}

// TestRenderExceedsMemoryBudget tests the same for Render: try rendering
// to an obviously too large pixel size.
func TestRenderExceedsMemoryBudget(t *testing.T) {
	ctx := context.Background()
	const limitPages = 64 // 4MiB.
	e, err := newTestEngineWithMemoryLimit(t, limitPages)
	if err != nil {
		t.Fatalf("newTestEngineWithMemoryLimit: %v", err)
	}

	doc, err := e.OpenDocument(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	defer doc.Close(ctx)

	// A 4000x4000 RGBA output buffer alone is 64MiB
	width := uint16(4000)
	height := uint16(4000)
	if _, err := doc.Render(ctx, 1, &RenderSettings{Width: &width, Height: &height}, nil); !errors.Is(err, ErrCrashed) {
		t.Fatalf("Render with an oversized output error = %v, want ErrCrashed", err)
	}

	// Later calls fail cleanly rather than running on a module instance in
	// an unknown state.
	if _, err := doc.Render(ctx, 1, nil, nil); !errors.Is(err, ErrCrashed) {
		t.Errorf("Render after a crash error = %v, want ErrCrashed", err)
	}
	if _, err := doc.PageInfo(ctx, 1); !errors.Is(err, ErrCrashed) {
		t.Errorf("PageInfo after a crash error = %v, want ErrCrashed", err)
	}
	if err := doc.Close(ctx); err != nil {
		t.Errorf("Close after a crash: %v", err)
	}
}

func TestWasmMemorySize(t *testing.T) {
	ctx := context.Background()
	const limitPages = 64 // 4MiB.
	const limitBytes = limitPages * 65536
	e, err := newTestEngineWithMemoryLimit(t, limitPages)
	if err != nil {
		t.Fatalf("newTestEngineWithMemoryLimit: %v", err)
	}

	doc, err := e.OpenDocument(ctx, []byte(minimalPDF))
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	defer doc.Close(ctx)

	memSize := doc.WasmMemorySize()
	if memSize < 65536 || memSize > limitBytes {
		t.Fatalf("expected memSize 0 < memSize (%d) < %d", memSize, limitBytes)
	}
	t.Logf("memSize is %d bytes", memSize)
}
