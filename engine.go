package hayro

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
)

// Engine wraps the wasm compilation step shared by every Document: a
// wazero.Runtime and a wazero.CompiledModule (the native-compiled wasm
// code) built once from the embedded module. Compiling is the expensive
// part — measured at roughly 500-1000x the cost of instantiating a
// Document from an already-compiled module — so an Engine is meant to be
// created once and reused for the life of a process, not per Document.
//
// Opening a Document from an Engine is correspondingly cheap: it doesn't
// recompile anything, just instantiates a fresh, isolated module instance
// (its own linear memory, its own allocator state) from the shared
// CompiledModule. There's no pool of pre-warmed instances here — none is
// needed at that cost.
//
// An Engine is safe for concurrent use, including concurrent Open calls:
// wazero explicitly supports instantiating the same CompiledModule from
// multiple goroutines at once.
type Engine struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	nextID   atomic.Uint64
}

// NewEngine compiles the embedded hayro-wasm-bridge module and returns an
// Engine ready to Open Documents from. The caller should Close it when
// done — typically once, at process shutdown.
func NewEngine(ctx context.Context) (*Engine, error) {
	rt := wazero.NewRuntime(ctx)
	compiled, err := rt.CompileModule(ctx, wasmBinary)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("hayro: compiling embedded wasm module: %w", err)
	}
	return &Engine{runtime: rt, compiled: compiled}, nil
}

// Open parses pdf — the raw bytes of a PDF file — and returns a Document
// backed by a fresh module instance. The caller must call Close on the
// returned Document when done with it; that's independent of, and doesn't
// require, closing the Engine.
//
// Open validates that pdf actually parses (returning ErrInvalidPDF if not)
// rather than deferring that failure to the first PageCount or Render
// call.
func (e *Engine) Open(ctx context.Context, pdf []byte) (*Document, error) {
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

// Close closes the Engine's underlying wazero.Runtime — and, per wazero's
// own semantics, every Document instantiated from it that hasn't already
// been closed. Call it once, at process shutdown; not after every Open.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
