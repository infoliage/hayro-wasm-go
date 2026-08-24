# hayro-wasm-go

An Go package wrapping [`hayro-wasm-bridge`](../hayro-wasm-bridge)'s
compiled wasm module — itself a wasm build of
[`hayro`](https://crates.io/crates/hayro), a pure-Rust PDF rasterizer.  Uses
[`wazero`](https://wazero.io).

This package's public surface is `Engine` and `Document`: `Engine` wraps the
one-time wasm compilation step (`NewEngine`, `Open`, `Close`); `Document` is
one open PDF (`PageCount`/`Render`/`Close`).

**Please note the API revisions are planned in the near future**; this should
be considered Alpha software with no published versions (yet).


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
about:

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
for it.  In the future we will try to provide precise provenance information
about the wasm module's Hayro version.

## Concurrency

`Engine.Open` is safe to call concurrently from multiple goroutines —
compiling the wasm module happens once, in `NewEngine`, and wazero
explicitly supports instantiating the same compiled module from many
goroutines at once.

Each `Document` returned by `Open` owns its own wasm module instance (own
linear memory, own allocator state) and serializes calls against it internally
with a mutex.  If for some reason you needed actual parallel rendering, open
the same PDF bytes as separate `Document`s (via the same `Engine`), one per
goroutine.

## Status

Pre-Alpha software.

Not yet handled: password-protected PDFs (blocked on `hayro-wasm-bridge`
exposing them first), and a panic/trap in the wasm module currently returns a
Go `error` from the failing call but leaves the `Document`'s instance in an
unknown state — there's no recycling policy yet for "this instance took a trap,
treat it as poisoned." See the `hayro-wasm-bridge` repo's own README Status
section for what's outstanding one layer down.
