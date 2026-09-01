```
Copyright 2026 Infoliage LLC. All rights reserved.
Use is subject to license terms.

SPDX-License-Identifier: Apache-2.0 OR MIT
```

# hayro-wasm-go

This package makes the Rust-based
[`Hayro PDF renderer`](https://crates.io/crates/hayro) available to Go
programs; it runs the hayro code in a [`wazero`](https://wazero.io) WASM
sandbox.

The WASM module is included as a binary blob inside of this package, but
it can be independently built from the
[`hayro-wasm-bridge`](https://github.com/infoliage/hayro-wasm-bridge)
project.

**Please note that API revisions are planned**; this should be considered Alpha
software with no published versions.


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

fmt.Println(doc.Info().PageCount)

img, err := doc.Render(ctx, 1, nil, nil) // *image.NRGBA, page 1, hayro's defaults
```

`Document.Info` (document title/author/etc, PDF version) and
`Document.PageInfo` (one page's size/rotation/media box) are cheap
alternatives to `Render` for callers who only need to know something about
a PDF, not draw it — `PageInfo`, in particular, gets you a page's point
dimensions (via its `Width`/`Height`, already rotation-adjusted) without
rasterizing anything:

```go
info := doc.Info() // no further wasm call — fetched during Open
fmt.Println(info.PageCount, info.Version)
if info.Title != nil {
    fmt.Println(*info.Title)
}

page, err := doc.PageInfo(ctx, 1)
fmt.Printf("page 1 is %vx%v points, rotated %d°\n", page.Width, page.Height, page.Rotation)
```

Pass non-nil `*RenderSettings`/`*InterpreterSettings` to `Render` for
anything other than `hayro`'s defaults — see their doc comments in
`settings.go`. Every field in both types is a pointer; `nil` means "use the
default for that field", so it's fine to only set the fields you care
about:

```go
width := uint16(800)
height := uint16(600)
img, err := doc.Render(ctx, 1, &hayro.RenderSettings{
    Width:  &width,
    Height: &height,
}, nil)
```

In particular, `RenderSettings`'s `Width`/`Height` default to "auto"
(derived from the page's own point size at the given scale) — pass `nil`
for `render` entirely, as above, unless you actually need to force
specific pixel dimensions of the canvas.

## Example

`examples/topng` is a minimal end-to-end usage example: it renders one
page of a PDF to a PNG.  In contrast to hayro's API, it allows you
specify desired output width and/or height, and then adjusts the
Hayro scale and canvas parameters to match.

```sh
go run ./examples/topng input.pdf output.png        # page 1
go run ./examples/topng -page 2 input.pdf output.png
go run ./examples/topng -width 800 input.pdf output.png   # 800px wide, height scaled to match
go run ./examples/topng -height 600 input.pdf output.png  # 600px tall, width scaled to match
go run ./examples/topng -width 800 -height 600 input.pdf output.png # exact, may distort
```

`-width`/`-height` alone use `Document.PageInfo` to scale the other
dimension proportionally (uniform scaling, no distortion); passing both
overrides each exactly instead.

## The embedded wasm module

`wasm/hayro_wasm_bridge.wasm` is a checked-in build artifact, embedded
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
