// Package server exposes the metadata engine over a small JSON HTTP API,
// bound to loopback only, for the embedded browser-based UI. No SSE/job
// machinery here — inspecting or stripping metadata is fast enough
// (in-memory byte parsing, not video/image encoding) that a plain
// request/response is enough; adding a progress-streaming layer for
// something that finishes in milliseconds would be complexity nothing
// here needs.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DavidMarsanic/photo-privacy-cleaner/web"
)

const idleTimeout = 30 * time.Minute
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
	DefaultOutputDir string
	ctx              context.Context

	mu      sync.Mutex
	batches map[string]*batch

	lastActivity atomic.Int64
}

func New(ctx context.Context, defaultOutputDir string) *Server {
	s := &Server{
		ctx:              ctx,
		DefaultOutputDir: defaultOutputDir,
		batches:          map[string]*batch{},
	}
	s.touch()
	return s
}

func (s *Server) Start(port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("starting local server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("POST /api/clean", s.handleClean)
	mux.HandleFunc("POST /api/reveal", s.handleReveal)
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.Handle("GET /", http.FileServer(http.FS(web.Static)))

	httpSrv := &http.Server{Handler: s.trackActivity(mux)}
	go func() {
		_ = httpSrv.Serve(ln)
	}()
	go s.watchIdle()

	return "http://" + ln.Addr().String(), nil
}

func (s *Server) trackActivity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.touch()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) touch() {
	s.lastActivity.Store(time.Now().Unix())
}

func (s *Server) watchIdle() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		idleFor := time.Now().Unix() - s.lastActivity.Load()
		if idleFor > int64(idleTimeout.Seconds()) {
			os.Exit(0)
		}
	}
}
