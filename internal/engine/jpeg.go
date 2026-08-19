package engine

import (
	"bytes"
	"fmt"
)

// jpeg markers relevant here. See ITU-T T.81 Annex B.
const (
	markerSOI = 0xD8 // start of image — no length field
	markerEOI = 0xD9 // end of image — no length field
	markerSOS = 0xDA // start of scan — has a length-prefixed header, then raw entropy-coded data follows with no further length prefixes until EOI
	markerAPP1 = 0xE1 // EXIF (and, separately, XMP) always live here
	markerAPP13 = 0xED // Photoshop IRB, which can carry IPTC (author/caption/keywords)
	markerCOM = 0xFE // free-form comment segment
)

// stripJPEGMetadata rewrites a JPEG dropping APP1 (EXIF/XMP), APP13
// (Photoshop IPTC), and COM (comment) segments, while leaving every other
// byte — including the actual compressed image data — untouched. This is
// deliberately not a decode-and-re-encode: that would recompress the
// image and lose quality on every clean, which is the wrong trade for a
// tool whose entire job is "make this safe to share," not "make this
// slightly worse." Unknown/malformed input is returned as an error rather
// than guessed at.
func stripJPEGMetadata(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != markerSOI {
		return nil, fmt.Errorf("not a JPEG file")
	}

	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:2]) // SOI
	pos := 2

	for pos < len(data) {
		if data[pos] != 0xFF {
			return nil, fmt.Errorf("malformed JPEG: expected marker at byte %d", pos)
		}
		// A run of 0xFF fill bytes can precede a real marker.
		marker := data[pos+1]
		for marker == 0xFF && pos+2 < len(data) {
			pos++
			marker = data[pos+1]
		}

		if marker == markerEOI {
			out.Write(data[pos : pos+2])
			pos += 2
			continue
		}

		// Segments with no length field: TEM, RST0-RST7.
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			out.Write(data[pos : pos+2])
			pos += 2
			continue
		}

		if pos+4 > len(data) {
			return nil, fmt.Errorf("malformed JPEG: truncated segment header at byte %d", pos)
		}
		length := int(data[pos+2])<<8 | int(data[pos+3])
		segEnd := pos + 2 + length
		if segEnd > len(data) {
			return nil, fmt.Errorf("malformed JPEG: segment length overruns file at byte %d", pos)
		}

		if marker != markerAPP1 && marker != markerAPP13 && marker != markerCOM {
			out.Write(data[pos:segEnd])
		}

		if marker == markerSOS {
			// Everything from here to EOI is entropy-coded scan data —
			// stray 0xFF bytes in it are followed by 0x00 stuffing or a
			// restart marker, never a real segment marker, so there's
			// nothing left to parse. Copy it through verbatim.
			out.Write(data[segEnd:])
			return out.Bytes(), nil
		}

		pos = segEnd
	}

	return nil, fmt.Errorf("malformed JPEG: reached end of file before SOS/EOI")
}
