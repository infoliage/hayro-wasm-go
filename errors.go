// Copyright 2026 Infoliage LLC. All rights reserved.
// Use is subject to license terms.
//
// SPDX-License-Identifier: Apache-2.0 OR MIT

package hayro

import "errors"

var (
	// ErrInvalidPDF is returned when a document's bytes could not be parsed
	// as a PDF.
	ErrInvalidPDF = errors.New("hayro: could not parse PDF")

	// ErrPageOutOfRange is returned when a page number passed to Render is
	// outside [1, Document.PageCount()].
	ErrPageOutOfRange = errors.New("hayro: page number out of range")

	// ErrRenderFailed is returned when Render fails for a reason other
	// than an out-of-range page number — currently only a zero-area
	// render (e.g. an explicit zero RenderSettings.Width/Height combined
	// with a zero XScale/YScale).
	ErrRenderFailed = errors.New("hayro: render failed")

	// ErrClosed is returned by any Document method called after Close.
	ErrClosed = errors.New("hayro: document is closed")
)
