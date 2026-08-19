// Package engine reads and strips privacy-relevant metadata (GPS,
// camera/device, date, software, author) from JPEG and PNG files —
// entirely in memory, no external tool, no network access. Stripping is
// always a surgical removal of the relevant segments/chunks, never a
// decode-and-re-encode: that would recompress the image and lose quality
// on every single clean, which is the wrong trade for a tool whose whole
// job is "make this safe to share."
package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// PhotoInfo is whatever privacy-relevant metadata Inspect found.
type PhotoInfo struct {
	HasMetadata bool    `json:"hasMetadata"`
	HasGPS      bool    `json:"hasGps"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Make        string  `json:"make,omitempty"`
	Model       string  `json:"model,omitempty"`
	DateTime    string  `json:"dateTime,omitempty"`
	Software    string  `json:"software,omitempty"`
}

func isJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
}

func isPNG(data []byte) bool {
	return bytes.HasPrefix(data, pngSignature)
}

// Inspect reports whatever privacy-relevant metadata is discoverable in
// data, without modifying anything.
func Inspect(data []byte) (*PhotoInfo, error) {
	switch {
	case isJPEG(data):
		return inspectJPEG(data), nil
	case isPNG(data):
		return inspectPNG(data), nil
	default:
		return nil, fmt.Errorf("unsupported file type — only JPEG and PNG are supported")
	}
}

func inspectJPEG(data []byte) *PhotoInfo {
	info := &PhotoInfo{}
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		// No EXIF segment at all is common (a screenshot, an already-
		// cleaned photo) and not an error — just nothing to report.
		return info
	}
	info.HasMetadata = true

	if lat, long, err := x.LatLong(); err == nil {
		info.HasGPS = true
		info.Latitude, info.Longitude = lat, long
	}
	if v, err := stringTag(x, exif.Make); err == nil {
		info.Make = v
	}
	if v, err := stringTag(x, exif.Model); err == nil {
		info.Model = v
	}
	if v, err := stringTag(x, exif.Software); err == nil {
		info.Software = v
	}
	if t, err := x.DateTime(); err == nil {
		info.DateTime = t.Format(time.RFC3339)
	}
	return info
}

func stringTag(x *exif.Exif, name exif.FieldName) (string, error) {
	tag, err := x.Get(name)
	if err != nil {
		return "", err
	}
	return tag.StringVal()
}

// inspectPNG only reports whether a metadata chunk is present, not its
// contents — PNG text chunks are freeform key/value pairs with no fixed
// schema the way EXIF has, so there's no reliable "GPS field" to extract.
// Clean strips whatever's actually there regardless of whether Inspect
// could describe it.
func inspectPNG(data []byte) *PhotoInfo {
	info := &PhotoInfo{}
	pos := 8
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		chunkType := string(data[pos+4 : pos+8])
		if pngMetadataChunks[chunkType] {
			info.HasMetadata = true
		}
		if chunkType == "IEND" {
			break
		}
		pos += 8 + length + 4
		if pos > len(data) {
			break
		}
	}
	return info
}

// Clean returns data with all discoverable privacy-relevant metadata
// removed, preserving pixel data exactly.
func Clean(data []byte) ([]byte, error) {
	switch {
	case isJPEG(data):
		return stripJPEGMetadata(data)
	case isPNG(data):
		return stripPNGMetadata(data)
	default:
		return nil, fmt.Errorf("unsupported file type — only JPEG and PNG are supported")
	}
}
