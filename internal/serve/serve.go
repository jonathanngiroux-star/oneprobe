// Package serve is Oneprobe's embedded HTTP surface: target
// registration and listing, sample queries, recent scrapes, /healthz
// for liveness, /donate for the donation addresses, and a minimal UI.
// The scrape loop runs in-process against the same SQLite file.
package serve

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/donate"
	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
	"github.com/jonathanngiroux-star/oneprobe/internal/ui"
)

// Server owns the HTTP handler, the embedded store, and the scrape loop.
type Server struct {
	store    *store.Store
	loop     *scrape.Loop
	stopLoop chan struct{}

	tickEvery time.Duration
}

// New opens (creating if needed) the SQLite database at dbPath, starts
// the scrape loop, and returns the server. Close stops the loop and
// closes the store.
func New(dbPath string) (*Server, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:     st,
		loop:      scrape.NewLoop(st, scrape.NewFetcher(10*time.Second)),
		stopLoop:  make(chan struct{}),
		tickEvery: 1 * time.Second,
	}
	go s.loop.Run(s.tickEvery, s.stopLoop)
	return s, nil
}

// SetScrapeTickInterval changes how often the loop wakes to re-check
// target due-ness (NOT the per-target scrape interval). Takes effect on
// the next Run; tests call it before the first scrape is due.
func (s *Server) SetScrapeTickInterval(d time.Duration) {
	s.tickEvery = d
}

// Close stops the scrape loop and closes the store.
func (s *Server) Close() error {
	close(s.stopLoop)
	return s.store.Close()
}

// Handler returns the HTTP routes. Unknown paths must 404, not fall
// into a catch-all.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/donate", s.handleDonate)
	mux.HandleFunc("/api/targets", s.handleTargets)
	mux.HandleFunc("/api/targets/", s.handleTargetSubresources)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/scrapes", s.handleRecentScrapes)
	mux.HandleFunc("/", s.handleUI)

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleDonate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(donate.PlainText()))
}

// handleTargets: GET lists all targets; POST registers one.
func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		targets, err := s.store.ListTargets()
		if err != nil {
			http.Error(w, "list targets: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, targets)
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			URL      string `json:"url"`
			Interval int    `json:"interval"` // milliseconds
			Enabled  *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		tg, err := s.store.AddTarget(store.Target{
			Name:     req.Name,
			URL:      req.URL,
			Interval: scrape.Millis(time.Duration(req.Interval) * time.Millisecond),
			Enabled:  enabled,
		})
		if err != nil {
			http.Error(w, "add target: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, tg)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTargetSubresources: DELETE /api/targets/{id} and GET
// /api/targets/{id}/scrapes.
func (s *Server) handleTargetSubresources(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/targets/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid target id "+parts[0], http.StatusBadRequest)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := s.store.DeleteTarget(id); err != nil {
			http.Error(w, "delete target: "+err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) == 2 && parts[1] == "scrapes" && r.Method == http.MethodGet {
		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}
		scrapes, err := s.store.ListScrapes(id, limit)
		if err != nil {
			http.Error(w, "list scrapes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, scrapes)
		return
	}
	http.NotFound(w, r)
}

// handleQuery: GET /api/query?metric=name&from=ms&to=ms returns the
// stored points for a metric (defaults: last hour).
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		http.Error(w, "metric parameter is required", http.StatusBadRequest)
		return
	}
	to := time.Now().UTC().UnixMilli()
	from := to - int64(time.Hour/time.Millisecond)
	if v := r.URL.Query().Get("to"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			to = p
		}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = p
		}
	}
	points, err := s.store.QueryRange(metric, from, to)
	if err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metric": metric,
		"points": points,
	})
}

// handleRecentScrapes: GET /api/scrapes returns the most recent scrape
// per target (the UI's "recent scrapes" panel).
func (s *Server) handleRecentScrapes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		http.Error(w, "list targets: "+err.Error(), http.StatusInternalServerError)
		return
	}
	type row struct {
		TargetID int64              `json:"target_id"`
		Scrapes  []store.ScrapeMeta `json:"scrapes"`
	}
	rows := []row{}
	for _, tg := range targets {
		scrapes, err := s.store.ListScrapes(tg.ID, 1)
		if err != nil {
			http.Error(w, "list scrapes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		rows = append(rows, row{TargetID: tg.ID, Scrapes: scrapes})
	}
	writeJSON(w, http.StatusOK, rows)
}

// handleUI serves the embedded single-page UI at /.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ui.Page()))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(v)
}
