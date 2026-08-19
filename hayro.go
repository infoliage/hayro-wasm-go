// Package hayro is a Go wrapper around hayro-wasm-bridge's compiled wasm
// module, which itself wraps the hayro PDF rasterizer
// (https://crates.io/crates/hayro). It hides that module's plain
// pointer/length C-ABI (see hayro-wasm-bridge's README) behind idiomatic
// Go types — callers use Engine and Document, not wasm memory offsets.
//
// The wasm module is embedded directly into this package's binary (see
// wasm/hayro_wasm_bridge.wasm and the Makefile's update-wasm target), so
// there's nothing to fetch or load separately at runtime.
package hayro

import (
	_ "embed"
)

//go:embed wasm/hayro_wasm_bridge.wasm
var wasmBinary []byte
