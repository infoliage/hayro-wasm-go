// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

// Command topng renders one page of a PDF to a PNG, as a
// minimal usage example for the package.
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
	width := flag.Uint("width", 0, "target pixel width; if -height is not also given, height is scaled to preserve the page's aspect ratio")
	height := flag.Uint("height", 0, "target pixel height; if -width is not also given, width is scaled to preserve the page's aspect ratio")
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

	// flag.Visit only visits flags explicitly set, which matters here: an
	// unset flag's zero-valued default (0) must not be confused with an
	// explicit "0", which is honored literally (e.g. a 0 scale produces a
	// zero-area canvas, and so a failing render).
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if (set["width"] || set["height"]) && (set["xscale"] || set["yscale"]) {
		fmt.Fprintln(os.Stderr, "topng: -xscale/-yscale can't be combined with -width/-height (width/height already compute their own scale factor)")
		os.Exit(2)
	}

	if err := run(inPath, outPath, *page, sizeFlags{*width, *height, set["width"], set["height"]}, *xScale, *yScale, set["xscale"], set["yscale"], *bgColor, set["bgcolor"]); err != nil {
		fmt.Fprintln(os.Stderr, "topng:", err)
		os.Exit(1)
	}
}

// sizeFlags bundles -width/-height's raw values with whether each was
// actually passed (see flag.Visit's use in main).
type sizeFlags struct {
	width, height       uint
	widthSet, heightSet bool
}

// buildPixmapSettings builds a *hayro.PixmapSettings from whichever flags
// were actually passed, or nil if none were.
//
// Any size or scale flag turns into three settings that have to agree:
// the canvas Width/Height in pixels, and a Transform that scales the
// page's content to fill it. (Width/Height on their own only size the
// canvas; the content would stay at one pixel per point in its corner.)
//
//   - -xscale/-yscale: the canvas is natural's size times the scale,
//     truncated to whole pixels.
//   - exactly one of -width/-height: the given axis is hit exactly, the
//     content is scaled uniformly to match, and the other pixel dimension
//     is derived from natural's aspect ratio at that same scale (see
//     aspectFill) — this is what makes "-width 800" alone produce a
//     proportionally scaled image instead of a stretched or unscaled one.
//   - both -width and -height: the canvas is exactly the requested size,
//     and the content is scaled independently per axis to fill it.
func buildPixmapSettings(natural hayro.PageInfo, size sizeFlags, xScale, yScale float64, xScaleSet, yScaleSet bool, bgColor string, bgColorSet bool) (*hayro.PixmapSettings, error) {
	if !size.widthSet && !size.heightSet && !xScaleSet && !yScaleSet && !bgColorSet {
		return nil, nil
	}

	var pixmap hayro.PixmapSettings
	if size.widthSet || size.heightSet || xScaleSet || yScaleSet {
		var width, height uint16
		xs, ys := 1.0, 1.0
		var err error
		switch {
		case size.widthSet && size.heightSet:
			if size.width > math.MaxUint16 {
				return nil, fmt.Errorf("-width %d out of range (must fit in a uint16)", size.width)
			}
			if size.height > math.MaxUint16 {
				return nil, fmt.Errorf("-height %d out of range (must fit in a uint16)", size.height)
			}
			xs, ys, err = fillScale(natural, size.width, size.height)
			if err != nil {
				return nil, err
			}
			width, height = uint16(size.width), uint16(size.height)
		case size.widthSet || size.heightSet:
			width, height, xs, err = aspectFill(natural, size)
			if err != nil {
				return nil, err
			}
			ys = xs
		default:
			if xScaleSet {
				xs = xScale
			}
			if yScaleSet {
				ys = yScale
			}
			width, err = truncToUint16(float64(natural.Width) * xs)
			if err != nil {
				return nil, fmt.Errorf("width at -xscale %v: %w", xs, err)
			}
			height, err = truncToUint16(float64(natural.Height) * ys)
			if err != nil {
				return nil, fmt.Errorf("height at -yscale %v: %w", ys, err)
			}
		}
		transform := hayro.Scale(xs, ys)
		pixmap.Width, pixmap.Height, pixmap.Transform = &width, &height, &transform
	}
	if bgColorSet {
		c, err := parseHexColor(bgColor)
		if err != nil {
			return nil, fmt.Errorf("-bgcolor: %w", err)
		}
		pixmap.BackgroundColor = &c
	}
	return &pixmap, nil
}

// truncToUint16 truncates x to a whole number of pixels, the same way
// hayro sizes its own canvas from a scale factor.
func truncToUint16(x float64) (uint16, error) {
	if !(x >= 0 && x <= math.MaxUint16) {
		return 0, fmt.Errorf("%v out of range (must fit in a uint16)", x)
	}
	return uint16(x), nil
}

// fillScale computes the independent horizontal/vertical scale factors
// that stretch natural's point-size dimensions (from Document.PageInfo) to
// exactly width x height — used for the "-width and -height both given"
// case, where the caller has explicitly accepted that the result may be
// distorted if the ratio doesn't match natural's own.
func fillScale(natural hayro.PageInfo, width, height uint) (xScale, yScale float64, err error) {
	if natural.Width <= 0 || natural.Height <= 0 {
		return 0, 0, fmt.Errorf("page has a zero-area natural size (%vx%v)", natural.Width, natural.Height)
	}
	return float64(width) / float64(natural.Width), float64(height) / float64(natural.Height), nil
}

// aspectFill computes a uniform scale factor from whichever of
// size.width/size.height was actually requested, plus the explicit pixel
// width/height pair that scale produces: the requested axis hit exactly,
// and the other derived from natural's aspect ratio (natural being the
// page's own point-size dimensions, from Document.PageInfo) — so scaling
// by one axis doesn't distort the image.
func aspectFill(natural hayro.PageInfo, size sizeFlags) (width, height uint16, scale float64, err error) {
	if natural.Width <= 0 || natural.Height <= 0 {
		return 0, 0, 0, fmt.Errorf("page has a zero-area natural size (%vx%v)", natural.Width, natural.Height)
	}

	if size.widthSet {
		if size.width > math.MaxUint16 {
			return 0, 0, 0, fmt.Errorf("-width %d out of range (must fit in a uint16)", size.width)
		}
		s := float64(size.width) / float64(natural.Width)
		derivedHeight, err := roundToUint16(float64(natural.Height) * s)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("height derived from -width %d: %w", size.width, err)
		}
		return uint16(size.width), derivedHeight, s, nil
	}

	if size.height > math.MaxUint16 {
		return 0, 0, 0, fmt.Errorf("-height %d out of range (must fit in a uint16)", size.height)
	}
	s := float64(size.height) / float64(natural.Height)
	derivedWidth, err := roundToUint16(float64(natural.Width) * s)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("width derived from -height %d: %w", size.height, err)
	}
	return derivedWidth, uint16(size.height), s, nil
}

// roundToUint16 rounds x to the nearest integer and clamps it into
// [1, math.MaxUint16] — 1 rather than 0, since a derived dimension of 0
// would silently produce a zero-area, failing render for a request that
// otherwise looks perfectly reasonable (e.g. a very wide, short page
// scaled down to a small width).
func roundToUint16(x float64) (uint16, error) {
	rounded := math.Round(x)
	if rounded > math.MaxUint16 {
		return 0, fmt.Errorf("%v out of range (must fit in a uint16)", rounded)
	}
	return uint16(max(rounded, 1)), nil
}

// parseHexColor parses "#RRGGBB" or "#RRGGBBAA" into a color.NRGBA
// (straight, non-premultiplied alpha, matching PixmapSettings.
// BackgroundColor's own type) — the same #RRGGBBAA form used throughout
// this project's own docs, e.g. hayro-wasm-bridge's
// schema/pixmap-settings.schema.json documents the default as
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

func run(inPath, outPath string, page uint, size sizeFlags, xScale, yScale float64, xScaleSet, yScaleSet bool, bgColor string, bgColorSet bool) error {
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

	doc, err := engine.OpenDocument(ctx, pdfBytes)
	if err != nil {
		return fmt.Errorf("opening %s: %w", inPath, err)
	}
	defer doc.Close(ctx)
	fmt.Printf("Module memory size after Open: %dMiB\n", doc.WasmMemorySize()/(1024*1024))

	pageCount := doc.Info().PageCount
	if page < 1 || page > pageCount {
		return fmt.Errorf("page %d out of range (document has %d page(s))", page, pageCount)
	}

	// Only fetch the page's natural size (an extra, if cheap, wasm call)
	// when it's actually needed: working out the canvas size and content
	// scale for a requested size or scale factor (see
	// buildPixmapSettings).
	var natural hayro.PageInfo
	if size.widthSet || size.heightSet || xScaleSet || yScaleSet {
		natural, err = doc.PageInfo(ctx, page)
		if err != nil {
			return fmt.Errorf("getting page %d info: %w", page, err)
		}
	}

	pixmap, err := buildPixmapSettings(natural, size, xScale, yScale, xScaleSet, yScaleSet, bgColor, bgColorSet)
	if err != nil {
		return err
	}

	// pixmap is nil unless any of the flags above were passed, in which
	// case the defaults apply throughout: a canvas of the page's own
	// point size, at one pixel per point.
	img, err := doc.Render(ctx, page, nil, nil, pixmap)
	if err != nil {
		return fmt.Errorf("rendering page %d: %w", page, err)
	}

	fmt.Printf("Module memory size after Render: %dMiB\n", doc.WasmMemorySize()/(1024*1024))

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		return fmt.Errorf("encoding png: %w", err)
	}

	fmt.Printf("wrote %s (%dx%d, page %d of %d)\n", outPath, img.Rect.Dx(), img.Rect.Dy(), page, pageCount)
	return nil
}
