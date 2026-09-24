// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro_wasm_go

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
)

// Engine holds a compiled WASM module.  For each document you want to
// process, call OpenDocument.  This will create a lightweight WASM VM
// which starts up very rapidly.  Use this to process the document, and
// then call Document.Close().  Typically you can create one Engine for
// the lifetime of the process.
//
// Engine is safe for concurrent use, including concurrent Open calls.
type Engine struct {
	config   *EngineConfig
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	nextID   atomic.Uint64
}

// EngineConfig sets Engine parameters.  In particular this can be used to add
// additional configuration to the wazero VMs that the Engine creates.
// Initialize using `NewEngineConfig()`.
type EngineConfig struct {
	RuntimeConfig wazero.RuntimeConfig
}

// NewEngineConfig returns a default EngineConfig.
func NewEngineConfig() *EngineConfig {
	return &EngineConfig{
		RuntimeConfig: wazero.NewRuntimeConfig(),
	}
}

// NewEngine compiles the embedded hayro-wasm-bridge module and returns an
// Engine which is ready to process documents.  Each document is processed in
// its own WASM VM.  The caller should Close the Engine when done to free the
// compiled module.
func NewEngine(ctx context.Context) (*Engine, error) {
	cfg := NewEngineConfig()
	return NewEngineWithConfig(ctx, cfg)
}

// NewEngineWithConfig is the same as NewEngine, with additional configuration
// options exposed via EngineConfig. cfg should be generated using
// NewEngineConfig and then augmented.
func NewEngineWithConfig(ctx context.Context, cfg *EngineConfig) (*Engine, error) {
	if cfg == nil || cfg.RuntimeConfig == nil {
		panic("hayro: NewEngineWithConfig requires a non-nil EngineConfig and RuntimeConfig; use NewEngineConfig")
	}
	rt := wazero.NewRuntimeWithConfig(ctx, cfg.RuntimeConfig)
	compiled, err := rt.CompileModule(ctx, wasmBinary)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("hayro: compiling embedded wasm module: %w", err)
	}
	return &Engine{config: cfg, runtime: rt, compiled: compiled}, nil
}

// OpenDocument parses pdf and returns a Document backed by a fresh wazero
// Module instance. The caller must call `Document.Close()` when it is done
// with the document.
//
// OpenDocument validates that pdf actually parses (returning ErrInvalidPDF if
// not).
func (e *Engine) OpenDocument(ctx context.Context, pdf []byte) (*Document, error) {
	name := fmt.Sprintf("hayro-%d", e.nextID.Add(1))
	mod, err := e.runtime.InstantiateModule(ctx, e.compiled, wazero.NewModuleConfig().WithName(name))
	if err != nil {
		return nil, fmt.Errorf("hayro: instantiating wasm module: %w", err)
	}

	d := &Document{mod: mod}
	if err := d.init(ctx, pdf); err != nil {
		_ = mod.Close(ctx)
		return nil, err
	}
	return d, nil
}

// Close closes the Engine's underlying wazero.Runtime.  Any running vms are
// shut down.  Typically this is called only once, at process shutdown.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
