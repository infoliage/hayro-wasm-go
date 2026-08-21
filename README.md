# hayro-wasm-go

An idiomatic Go package wrapping [`hayro-wasm-bridge`](../hayro-wasm-bridge)'s
compiled wasm module — itself a wasm build of
[`hayro`](https://crates.io/crates/hayro), a pure-Rust PDF rasterizer — via
[`wazero`](https://wazero.io), a pure-Go wasm runtime with no cgo and no
external interpreter to install.

Callers never see the underlying C-ABI (pointers, lengths, alloc/free
pairs — see `hayro-wasm-bridge/README.md` for that layer). This package's
public surface is `Engine` and `Document`: `Engine` wraps the one-time wasm
compilation step (`NewEngine`, `Open`, `Close`); `Document` is one open PDF
(`PageCount`/`Render`/`Close`).

## Usage

```go
engine, err := hayro.NewEngine(ctx) // do this once per process
if err != nil {
    // ...
}
defer engine.Close(ctx)

doc, err := engine.Open(ctx, pdfBytes)
if err != nil {
    // hayro.ErrInvalidPDF if pdfBytes didn't parse
}
defer doc.Close(ctx)

fmt.Println(doc.PageCount())

img, err := doc.Render(ctx, 1, nil, nil) // *image.NRGBA, page 1, hayro's defaults
```

Pass non-nil `*RenderSettings`/`*InterpreterSettings` to `Render` for
anything other than `hayro`'s defaults — see their doc comments in
`settings.go`. Every field in both types is a pointer; `nil` means "use the
default for that field", so it's fine to only set the fields you care
about — Go 1.26's `new(expr)` (not just the older `new(Type)`) builds a
literal inline, no helper function needed:

```go
img, err := doc.Render(ctx, 1, &hayro.RenderSettings{
    Width:  new(uint16(800)),
    Height: new(uint16(600)),
}, nil)
```

In particular, `RenderSettings`'s `Width`/`Height` default to "auto"
(derived from the page's own point size at the given scale) — pass `nil`
for `render` entirely, as above, unless you actually need to force
specific pixel dimensions.

Under the hood, `Render` JSON-encodes whichever settings you pass and
hands the bytes to `hayro-wasm-bridge`'s wasm module — see that crate's
README ("Why JSON for the settings blobs") for why, and
`schema/*.json` there for the wire format both sides agree on. This is an
implementation detail; callers only ever see `RenderSettings`/
`InterpreterSettings`.

## Example

`examples/topng` is a minimal end-to-end usage example: it renders one
page of a PDF to a PNG.

```sh
go run ./examples/topng input.pdf output.png        # page 1
go run ./examples/topng -page 2 input.pdf output.png
```

## The embedded wasm module

`wasm/hayro_wasm_bridge.wasm` is a **checked-in build artifact**, embedded
into this package's binary via `go:embed` (see `hayro.go`) — there's
nothing to fetch or load at runtime, and no separate distribution step
for it.

It's built from the sibling `hayro-wasm-bridge` checkout, not built from
source as part of this module (this is a Go module; it has no Rust
toolchain dependency). After pulling in changes to `hayro-wasm-bridge`,
regenerate it with:

```sh
make update-wasm
```

then commit the result. (`hayro-wasm-bridge` isn't versioned/published
anywhere yet, so for now this is the only way the two repos stay in sync —
worth reconsidering once it has real releases.)

## Concurrency

`Engine.Open` is safe to call concurrently from multiple goroutines —
compiling the wasm module happens once, in `NewEngine`, and wazero
explicitly supports instantiating the same compiled module from many
goroutines at once.

Each `Document` returned by `Open` owns its own wasm module instance (own
linear memory, own allocator state) and serializes calls against it
internally with a mutex — `hayro-wasm-bridge` itself has no interior
locking (see its README). A single `Document`'s methods are safe to call
from multiple goroutines, but those calls won't run in parallel. For
actual parallel rendering, open the same PDF bytes as separate
`Document`s (via the same `Engine`), one per goroutine.

## Why Engine/Document, not a pool of instances

Some similar wrappers around wasm PDF libraries (e.g.
[go-pdfium](https://github.com/klippa-app/go-pdfium)) keep a pool of
pre-warmed wasm runtimes/instances around. Measured against this module,
that's not worth it here: compiling the wasm module (`NewEngine`, ~5MB of
wasm) takes on the order of hundreds of milliseconds, but instantiating a
`Document` from an already-compiled module (`Engine.Open`) takes on the
order of 100 microseconds — roughly a 5,000x gap. `Engine` amortizes the
expensive part once per process; `Document`s are already cheap enough,
per-`Open`/`Close`, that pooling them wouldn't measurably help.

## Status

Early scaffold. Not yet handled: password-protected PDFs (blocked on
`hayro-wasm-bridge` exposing them first), and a panic/trap in the wasm
module currently returns a Go `error` from the failing call but leaves the
`Document`'s instance in an unknown state — there's no recycling policy
yet for "this instance took a trap, treat it as poisoned." See the
`hayro-wasm-bridge` repo's own README Status section for what's outstanding
one layer down.
