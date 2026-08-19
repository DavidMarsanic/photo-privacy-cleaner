package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/DavidMarsanic/photo-privacy-cleaner/internal/browser"
	"github.com/DavidMarsanic/photo-privacy-cleaner/internal/engine"
)

type uploadedPhoto struct {
	ID       string            `json:"id"`
	Filename string            `json:"filename"`
	Info     *engine.PhotoInfo `json:"info,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// handleUpload accepts one or more photos, inspects each for
// privacy-relevant metadata, and holds the original bytes in memory
// (never written to disk) under a new batch id so a later /api/clean
// call can act on the same files without a second upload.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid upload", "code": "bad-request"})
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no files uploaded", "code": "bad-request"})
		return
	}

	b := &batch{photos: map[string]*photo{}}
	var results []uploadedPhoto

	for _, header := range files {
		f, err := header.Open()
		if err != nil {
			results = append(results, uploadedPhoto{Filename: header.Filename, Error: "couldn't read upload"})
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			results = append(results, uploadedPhoto{Filename: header.Filename, Error: "couldn't read upload"})
			continue
		}

		id := newID()
		name := sanitizeFilename(header.Filename)
		b.photos[id] = &photo{filename: name, data: data}

		info, err := engine.Inspect(data)
		if err != nil {
			results = append(results, uploadedPhoto{ID: id, Filename: name, Error: err.Error()})
			continue
		}
		results = append(results, uploadedPhoto{ID: id, Filename: name, Info: info})
	}

	batchID := newID()
	s.mu.Lock()
	s.batches[batchID] = b
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"batchId": batchID, "photos": results})
}

type cleanRequest struct {
	BatchID string   `json:"batchId"`
	IDs     []string `json:"ids"`
}

type cleanResult struct {
	ID       string `json:"id"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleClean strips metadata from the requested photos in a batch and
// writes each clean copy to the output directory, named "<stem>-clean.ext"
// — the original upload (still only in memory) is left completely alone.
// Once every photo in the batch has been either cleaned or explicitly
// errored, the batch's in-memory bytes are dropped.
func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	var req cleanRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	s.mu.Lock()
	b, ok := s.batches[req.BatchID]
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown batch — upload again", "code": "bad-request"})
		return
	}

	var results []cleanResult
	for _, id := range req.IDs {
		p, ok := b.photos[id]
		if !ok {
			results = append(results, cleanResult{ID: id, Error: "unknown photo in this batch"})
			continue
		}
		cleaned, err := engine.Clean(p.data)
		if err != nil {
			results = append(results, cleanResult{ID: id, Error: err.Error()})
			continue
		}
		outPath, err := writeClean(s.DefaultOutputDir, p.filename, cleaned)
		if err != nil {
			results = append(results, cleanResult{ID: id, Error: err.Error()})
			continue
		}
		results = append(results, cleanResult{ID: id, Filename: filepath.Base(outPath), Path: outPath})
	}

	s.mu.Lock()
	delete(s.batches, req.BatchID)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func writeClean(outputDir, originalName string, data []byte) (string, error) {
	ext := filepath.Ext(originalName)
	stem := strings.TrimSuffix(originalName, ext)
	if stem == "" {
		stem = "photo"
	}
	outPath := uniquePath(outputDir, stem+"-clean", ext)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", fmt.Errorf("saving cleaned photo: %w", err)
	}
	return outPath, nil
}

func uniquePath(dir, stem, ext string) string {
	candidate := filepath.Join(dir, stem+ext)
	for i := 2; fileExists(candidate); i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
	}
	return candidate
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "" {
		return "photo"
	}
	return name
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := browser.Reveal(req.Path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := browser.Open(req.Path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "bad-request"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
