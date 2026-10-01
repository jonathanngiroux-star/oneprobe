// Package serve is Oneprobe's embedded HTTP surface: /healthz for
// liveness, /donate for the donation addresses, and (in later
// milestones) the UI. It opens the same SQLite file the CLI uses.
package serve

import (
	"net/http"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// Server owns the HTTP handler and the embedded store.
type Server struct {
	store *store.Store
}

// New opens (creating if needed) the SQLite database at dbPath and
// returns the server. Close must be called by the caller.
func New(dbPath string) (*Server, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &Server{store: st}, nil
}

// Close closes the underlying store.
func (s *Server) Close() error { return s.store.Close() }

// Handler returns the HTTP routes. The API surface beyond /healthz and
// /donate is deliberately absent in v0.1 — unknown paths must 404, not
// fall into a catch-all.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/donate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(donate.PlainText()))
	})

	return mux
}
