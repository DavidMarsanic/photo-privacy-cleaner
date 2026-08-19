package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}

// pngMetadataChunks are dropped: tEXt/zTXt/iTXt carry arbitrary free-text
// key/value pairs (often including author, description, software, and
// sometimes GPS via an embedded XMP payload in iTXt), eXIf carries actual
// EXIF (modern PNGs can have it), tIME carries the last-modified
// timestamp. Everything else — IHDR, PLTE, IDAT, IEND, gAMA/cHRM/sRGB/
// iCCP (color), bKGD, pHYs (physical size) — isn't personal data and is
// left untouched.
var pngMetadataChunks = map[string]bool{
	"tEXt": true, "zTXt": true, "iTXt": true, "eXIf": true, "tIME": true,
}

// stripPNGMetadata rewrites a PNG dropping metadata chunks while leaving
// every other chunk's bytes — including all image data — untouched, for
// the same reason stripJPEGMetadata avoids a decode/re-encode: PNG
// compression is lossless, but re-encoding is still unnecessary work and
// unnecessary risk when the chunk format makes a surgical removal this
// simple.
func stripPNGMetadata(data []byte) ([]byte, error) {
	if len(data) < 8 || !bytes.Equal(data[:8], pngSignature) {
		return nil, fmt.Errorf("not a PNG file")
	}

	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:8])
	pos := 8

	for pos < len(data) {
		if pos+8 > len(data) {
			return nil, fmt.Errorf("malformed PNG: truncated chunk header at byte %d", pos)
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		chunkType := string(data[pos+4 : pos+8])
		chunkEnd := pos + 8 + length + 4 // length + type + data + CRC
		if chunkEnd > len(data) {
			return nil, fmt.Errorf("malformed PNG: chunk %q overruns file at byte %d", chunkType, pos)
		}

		if !pngMetadataChunks[chunkType] {
			out.Write(data[pos:chunkEnd])
		}

		if chunkType == "IEND" {
			return out.Bytes(), nil
		}
		pos = chunkEnd
	}

	return nil, fmt.Errorf("malformed PNG: reached end of file before IEND")
}
