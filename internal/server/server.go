// Package server exposes the metadata engine over a small JSON HTTP API,
// bound to loopback only, for the embedded browser-based UI. No SSE/job
// machinery here — inspecting or stripping metadata is fast enough
// (in-memory byte parsing, not video/image encoding) that a plain
// request/response is enough; adding a progress-streaming layer for
// something that finishes in milliseconds would be complexity nothing
// here needs. Shared server plumbing (loopback bind, idle-timeout
// shutdown, reveal/open routes) comes from brightencode-appkit
// unmodified — this app adds no extra validation on top of them.
package server

import (
	"context"
	"net/http"
	"sync"

	appkit "github.com/DavidMarsanic/brightencode-appkit/server"
	"github.com/DavidMarsanic/photo-privacy-cleaner/web"
)

const maxUploadBytes = 200 << 20 // 200MB — generous for a batch of photos

// photo is one uploaded file held in memory for the lifetime of its
// batch — never written to disk until Clean actually produces an output
// file, so an inspected-but-not-cleaned original never sits in a temp
// directory leaking exactly the metadata this app exists to remove.
type photo struct {
	filename string
	data     []byte
}

type batch struct {
	photos map[string]*photo // photo id -> photo
}

type Server struct {
	*appkit.Server
	DefaultOutputDir string

	mu      sync.Mutex
	batches map[string]*batch
}

func New(ctx context.Context, defaultOutputDir string) *Server {
	return &Server{
		Server:           appkit.New(ctx, 0),
		DefaultOutputDir: defaultOutputDir,
		batches:          map[string]*batch{},
	}
}

func (s *Server) Start(port int) (string, error) {
	return s.Server.Start(port, web.Static, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/upload", s.handleUpload)
		mux.HandleFunc("POST /api/clean", s.handleClean)
	})
}
