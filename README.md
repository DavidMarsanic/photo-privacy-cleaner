# Photo Privacy Cleaner

Strip GPS location, camera/device, date, and software metadata from
photos — entirely on this machine. Opens as its own window.

Drop a photo, see exactly what's hiding in it (including a map link if
there's a location), then remove it in one click. Originals are never
modified — a clean copy is saved alongside, and nothing is ever uploaded
anywhere.

Metadata removal is a lossless, surgical strip of the relevant JPEG
segments / PNG chunks — never a decode-and-re-encode, so pixel data is
untouched and there's no quality loss. Verified byte-for-byte identical
image data before and after, and cross-checked with `exiftool`.

## Requirements

**A Chromium-based browser already installed**: Google Chrome, Chromium,
Brave, Microsoft Edge, or Arc — renders the app's own UI window.

## Use

1. Open Photo Privacy Cleaner — it opens its own window.
2. Drop one or more JPEG/PNG photos.
3. See what was found: location (with a "view on map" link), camera
   model, date, and software — or "No metadata found" if there's nothing
   to remove.
4. **Remove private information.** Clean copies are saved to Downloads as
   `<name>-clean.<ext>`.

## Format support

JPEG and PNG only for now — no HEIC (iPhone's default format), since
decoding it needs a native library that would break this app's
cross-platform build. Convert to JPEG first if needed.

## License

MIT — see [LICENSE](LICENSE).
