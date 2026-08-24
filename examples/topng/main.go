// Command topng renders one page of a PDF to a PNG, using hayro-wasm-go's
// defaults throughout — a minimal end-to-end usage example for the
// package, not a general-purpose PDF-to-image tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"os"

	hayro "hayro-wasm-go"
)

func main() {
	page := flag.Uint("page", 1, "1-based page number to render")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-page N] input.pdf output.png\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(2)
	}
	inPath, outPath := args[0], args[1]

	if err := run(inPath, outPath, *page); err != nil {
		fmt.Fprintln(os.Stderr, "topng:", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string, page uint) error {
	ctx := context.Background()

	pdfBytes, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inPath, err)
	}

	engine, err := hayro.NewEngine(ctx)
	if err != nil {
		return fmt.Errorf("creating engine: %w", err)
	}
	defer engine.Close(ctx)

	doc, err := engine.Open(ctx, pdfBytes)
	if err != nil {
		return fmt.Errorf("opening %s: %w", inPath, err)
	}
	defer doc.Close(ctx)

	if page < 1 || page > doc.PageCount() {
		return fmt.Errorf("page %d out of range (document has %d page(s))", page, doc.PageCount())
	}

	// Leave RenderSettings nil (hayro's defaults throughout): Width/Height
	// default to "auto", derived from the page's own point size at 1x
	// scale — there's no need to compute pixel dimensions ourselves.
	img, err := doc.Render(ctx, page, nil, nil)
	if err != nil {
		return fmt.Errorf("rendering page %d: %w", page, err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		return fmt.Errorf("encoding png: %w", err)
	}

	fmt.Printf("wrote %s (%dx%d, page %d of %d)\n", outPath, img.Rect.Dx(), img.Rect.Dy(), page, doc.PageCount())
	return nil
}
