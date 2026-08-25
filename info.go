package hayro

import (
	"encoding/json"
	"fmt"
	"time"
)

// Rect is a rectangle in unrotated PDF user-space points, with X0<=X1 and
// Y0<=Y1.
type Rect struct {
	X0, Y0, X1, Y1 float64
}

// rectWire is Rect's JSON shape on the wire — see hayro-wasm-bridge's
// schema/page-info.schema.json.
type rectWire struct {
	X0 float64 `json:"x0"`
	Y0 float64 `json:"y0"`
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
}

func (w rectWire) toRect() Rect {
	return Rect(w)
}

// PageInfo is one page's geometry — see Document.PageInfo.
type PageInfo struct {
	// Width and Height are the page's render dimensions, in points: the
	// pixel size Render would produce at XScale/YScale of 1.0 and no
	// explicit Width/Height override. Already accounts for Rotation
	// (swapped for a 90/270 degree rotation), unlike MediaBox/CropBox
	// below.
	Width, Height float32

	// Rotation is the page's /Rotate entry, normalized to 0, 90, 180, or
	// 270.
	Rotation uint16

	// MediaBox and CropBox are the page's raw /MediaBox and /CropBox
	// (inherited from an ancestor /Pages node and defaulted per spec if
	// absent, same as hayro does internally). Unrotated, and CropBox is
	// not intersected with MediaBox, unlike Width/Height above.
	MediaBox, CropBox Rect
}

// pageInfoWire is PageInfo's JSON shape on the wire — see
// hayro-wasm-bridge's schema/page-info.schema.json.
type pageInfoWire struct {
	Width    float32  `json:"width"`
	Height   float32  `json:"height"`
	Rotation uint16   `json:"rotation"`
	MediaBox rectWire `json:"media_box"`
	CropBox  rectWire `json:"crop_box"`
}

func (w pageInfoWire) toPageInfo() PageInfo {
	return PageInfo{
		Width:    w.Width,
		Height:   w.Height,
		Rotation: w.Rotation,
		MediaBox: w.MediaBox.toRect(),
		CropBox:  w.CropBox.toRect(),
	}
}

// DocumentInfo is document-level metadata — see Document.Info.
type DocumentInfo struct {
	// PageCount is the document's page count — the same value
	// Document.PageCount returns.
	PageCount uint

	// Version is the effective PDF version, e.g. "1.7".
	Version string

	// Title, Author, Subject, Keywords, Creator, and Producer come from the
	// PDF's document information dictionary. nil means the field is absent
	// from the dictionary, or there is no document information dictionary
	// at all.
	Title, Author, Subject, Keywords, Creator, Producer *string

	// CreationDate and ModificationDate come from the document information
	// dictionary's /CreationDate and /ModDate. nil means absent. A bare
	// "Z" (explicit UTC) in the original PDF timestamp and no offset
	// suffix at all are indistinguishable here — both parse with a zero
	// UTC offset.
	CreationDate, ModificationDate *time.Time
}

// documentInfoWire is DocumentInfo's JSON shape on the wire — see
// hayro-wasm-bridge's schema/document-info.schema.json. A separate type
// from DocumentInfo itself because the wire's dates are ISO-8601 strings,
// not *time.Time, and its page_count is a plain (signed, but always
// non-negative in practice) integer, not DocumentInfo's uint.
type documentInfoWire struct {
	PageCount int32   `json:"page_count"`
	Version   string  `json:"version"`
	Title     *string `json:"title"`
	Author    *string `json:"author"`
	Subject   *string `json:"subject"`
	Keywords  *string `json:"keywords"`
	Creator   *string `json:"creator"`
	Producer  *string `json:"producer"`

	CreationDate     *string `json:"creation_date"`
	ModificationDate *string `json:"modification_date"`
}

func (w documentInfoWire) toDocumentInfo() (DocumentInfo, error) {
	creationDate, err := parseWireDate(w.CreationDate)
	if err != nil {
		return DocumentInfo{}, fmt.Errorf("creation_date: %w", err)
	}
	modificationDate, err := parseWireDate(w.ModificationDate)
	if err != nil {
		return DocumentInfo{}, fmt.Errorf("modification_date: %w", err)
	}

	return DocumentInfo{
		PageCount:        uint(w.PageCount),
		Version:          w.Version,
		Title:            w.Title,
		Author:           w.Author,
		Subject:          w.Subject,
		Keywords:         w.Keywords,
		Creator:          w.Creator,
		Producer:         w.Producer,
		CreationDate:     creationDate,
		ModificationDate: modificationDate,
	}, nil
}

// parseWireDate parses one of documentInfoWire's ISO-8601 date strings.
// hayro-wasm-bridge always emits an explicit numeric offset (never a bare
// "Z"), but time.RFC3339's "Z07:00" layout element accepts either form, so
// this only ever actually sees the numeric one in practice.
func parseWireDate(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// decodeJSON is a small helper shared by Document.PageInfo/fetchDocumentInfo:
// json.Unmarshal wrapped with a consistent error message.
func decodeJSON[T any](blob []byte) (T, error) {
	var v T
	err := json.Unmarshal(blob, &v)
	return v, err
}
