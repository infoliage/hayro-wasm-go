// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

// Package hayro is a Go wrapper around hayro-wasm-bridge's compiled wasm
// module, which itself wraps the hayro PDF rasterizer
// (https://crates.io/crates/hayro).
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
