// Command topng renders one page of a PDF to a PNG, using hayro-wasm-go's
// defaults throughout — a minimal end-to-end usage example for the
// package, not a general-purpose PDF-to-image tool.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"

	hayro "github.com/infoliage/hayro-wasm-go"
)

func main() {
	page := flag.Uint("page", 1, "1-based page number to render")
	width := flag.Uint("width", 0, "override rendered pixel width")
	height := flag.Uint("height", 0, "override rendered pixel height")
	xScale := flag.Float64("xscale", 0, "override horizontal scale factor")
	yScale := flag.Float64("yscale", 0, "override vertical scale factor")
	bgColor := flag.String("bgcolor", "", "override background color, as RRGGBB or RRGGBBAA")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-page N] [-width W] [-height H] [-xscale X] [-yscale Y] [-bgcolor #RRGGBBAA] input.pdf output.png\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(2)
	}
	inPath, outPath := args[0], args[1]

	render, err := renderSettingsFromFlags(*width, *height, *xScale, *yScale, *bgColor)
	if err != nil {
		fmt.Fprintln(os.Stderr, "topng:", err)
		os.Exit(2)
	}

	if err := run(inPath, outPath, *page, render); err != nil {
		fmt.Fprintln(os.Stderr, "topng:", err)
		os.Exit(1)
	}
}

// renderSettingsFromFlags builds a *hayro.RenderSettings from whichever of
// -width/-height/-xscale/-yscale/-bgcolor were actually passed on the
// command line, or nil if none were. flag.Visit only visits flags
// explicitly set, which matters here: an unset flag's zero-valued default
// (0) must not be confused with an explicit "0" — hayro-wasm-bridge's
// JSON wire format honors an explicit 0 literally (e.g. a 0 scale
// produces a zero-area, failing render) rather than treating it as "use
// hayro's default", so RenderSettings's pointer fields need the same care
// here.
func renderSettingsFromFlags(width, height uint, xScale, yScale float64, bgColor string) (*hayro.RenderSettings, error) {
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if !set["width"] && !set["height"] && !set["xscale"] && !set["yscale"] && !set["bgcolor"] {
		return nil, nil
	}

	var render hayro.RenderSettings
	if set["width"] {
		if width > math.MaxUint16 {
			return nil, fmt.Errorf("-width %d out of range (must fit in a uint16)", width)
		}
		render.Width = new(uint16(width))
	}
	if set["height"] {
		if height > math.MaxUint16 {
			return nil, fmt.Errorf("-height %d out of range (must fit in a uint16)", height)
		}
		render.Height = new(uint16(height))
	}
	if set["xscale"] {
		render.XScale = new(float32(xScale))
	}
	if set["yscale"] {
		render.YScale = new(float32(yScale))
	}
	if set["bgcolor"] {
		c, err := parseHexColor(bgColor)
		if err != nil {
			return nil, fmt.Errorf("-bgcolor: %w", err)
		}
		render.BackgroundColor = &c
	}
	return &render, nil
}

// parseHexColor parses "#RRGGBB" or "#RRGGBBAA" into a color.NRGBA
// (straight, non-premultiplied alpha, matching RenderSettings.
// BackgroundColor's own type) — the same #RRGGBBAA form used throughout
// this project's own docs, e.g. hayro-wasm-bridge's
// schema/render-settings.schema.json documents hayro's own default as
// "#00000000". A 6-digit "#RRGGBB" is accepted too, as shorthand for
// fully opaque (alpha = 0xff).
func parseHexColor(s string) (color.NRGBA, error) {
	hexDigits, _ := strings.CutPrefix(s, "#")
	if len(hexDigits) == 6 {
		hexDigits += "ff"
	}
	if len(hexDigits) != 8 {
		return color.NRGBA{}, fmt.Errorf("%q must be #RRGGBB or #RRGGBBAA", s)
	}

	b, err := hex.DecodeString(hexDigits)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("%q is not valid hex: %w", s, err)
	}
	return color.NRGBA{R: b[0], G: b[1], B: b[2], A: b[3]}, nil
}

func run(inPath, outPath string, page uint, render *hayro.RenderSettings) error {
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

	// render is nil unless -width/-height/-xscale/-yscale were passed, in
	// which case hayro's defaults apply throughout: Width/Height default
	// to "auto", derived from the page's own point size at 1x scale.
	img, err := doc.Render(ctx, page, render, nil)
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
