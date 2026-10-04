// Package serve is Oneprobe's embedded HTTP surface: target
// registration and listing, sample queries, recent scrapes, /healthz
// for liveness, /donate for the donation addresses, and a minimal UI.
// The scrape loop runs in-process against the same SQLite file.
package serve

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonathanngiroux-star/oneprobe/internal/alert"
	"github.com/jonathanngiroux-star/oneprobe/internal/migrate/datadog"
	"github.com/jonathanngiroux-star/oneprobe/internal/query"
	"github.com/jonathanngiroux-star/oneprobe/internal/scrape"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
	"github.com/jonathanngiroux-star/oneprobe/internal/ui"
)

// Server owns the HTTP handler, the embedded store, the scrape loop,
// and the alert engine.
type Server struct {
	store    *store.Store
	loop     *scrape.Loop
	stopLoop chan struct{}

	alertEngine *alert.Engine
	stopAlerts  chan struct{}

	tickEvery time.Duration

	mu         sync.Mutex
	alertEvery time.Duration
}

// New opens (creating if needed) the SQLite database at dbPath, starts
// the scrape loop and the alert engine, and returns the server. Close
// stops both and closes the store.
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

		alertEngine: alert.NewEngine(st, log.New(os.Stdout, "", log.LstdFlags), &http.Client{Timeout: 10 * time.Second}),
		stopAlerts:  make(chan struct{}),
		alertEvery:  15 * time.Second,
	}
	go s.loop.Run(s.tickEvery, s.stopLoop)
	go s.runAlerts()
	return s, nil
}

// SetAlertEvalInterval changes how often the alert engine evaluates.
// Takes effect on the next engine tick.
func (s *Server) SetAlertEvalInterval(d time.Duration) {
	s.mu.Lock()
	s.alertEvery = d
	s.mu.Unlock()
}

// SetScrapeTickInterval changes how often the loop wakes to re-check
// target due-ness (NOT the per-target scrape interval).
func (s *Server) SetScrapeTickInterval(d time.Duration) {
	s.mu.Lock()
	s.tickEvery = d
	s.mu.Unlock()
}

// runAlerts ticks the alert engine. The interval is re-read each tick
// so tests can accelerate it.
func (s *Server) runAlerts() {
	for {
		s.mu.Lock()
		every := s.alertEvery
		s.mu.Unlock()
		s.alertEngine.Evaluate(time.Now().UTC())
		timer := time.NewTimer(every)
		select {
		case <-s.stopAlerts:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Close stops the scrape loop and alert engine and closes the store.
func (s *Server) Close() error {
	close(s.stopLoop)
	close(s.stopAlerts)
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
	mux.HandleFunc("/api/alerts", s.handleAlerts)
	mux.HandleFunc("/api/alerts/", s.handleAlertSubresources)
	mux.HandleFunc("/api/migrate/datadog", s.handleMigrateDatadog)
	if ui.BridgeArmed() {
		mux.HandleFunc("/testbridge.js", s.handleTestBridgeJS)
	}
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ui.DonatePage()))
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
		if tg.CreatedNew {
			writeJSON(w, http.StatusCreated, tg)
		} else {
			// Same URL already existed: this was an UPDATE (rename).
			writeJSON(w, http.StatusOK, tg)
		}
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

// handleQuery: GET /api/query?expr=<promql> evaluates a subset
// expression; ?metric=name&from=ms&to=ms returns raw stored points for
// one metric (the UI graph's simple mode). Exactly one of expr/metric
// is required.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	expr := r.URL.Query().Get("expr")
	metric := r.URL.Query().Get("metric")
	if expr != "" && metric != "" {
		http.Error(w, "pass either expr or metric, not both", http.StatusBadRequest)
		return
	}
	if expr != "" {
		s.handleQueryExpr(w, r, expr)
		return
	}
	if metric == "" {
		http.Error(w, "metric parameter is required (or use expr=)", http.StatusBadRequest)
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

// handleQueryExpr evaluates a PromQL-subset expression at now with a
// 5-minute lookback for bare selectors.
func (s *Server) handleQueryExpr(w http.ResponseWriter, r *http.Request, expr string) {
	node, err := query.Parse(expr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	res, err := query.Eval(node, s.store, now, 5*time.Minute)
	if err != nil {
		http.Error(w, "eval: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, res)
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
// handleTestBridgeJS serves the in-app test-bridge interpreter.
// Bridge-only: the route is not even registered in a normal launch.
func (s *Server) handleTestBridgeJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(ui.TestBridgeJS()))
}

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

// alertView is a rule joined with its live evaluation state — the
// shape /api/alerts returns.
type alertView struct {
	store.AlertRule
	State      string  `json:"state"`
	LastValue  float64 `json:"last_value"`
	LastEvalAt string  `json:"last_eval_at,omitempty"`
	ChangedAt  string  `json:"changed_at,omitempty"`
}

// handleAlerts: GET lists rules with live state; POST creates one.
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := s.store.ListAlertRules()
		if err != nil {
			http.Error(w, "list alert rules: "+err.Error(), http.StatusInternalServerError)
			return
		}
		views := make([]alertView, 0, len(rules))
		for _, rule := range rules {
			st, err := s.store.GetAlertState(rule.ID)
			if err != nil {
				http.Error(w, "get alert state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			v := alertView{AlertRule: rule, State: st.State, LastValue: st.LastValue}
			if !st.LastEvalAt.IsZero() {
				v.LastEvalAt = st.LastEvalAt.Format(time.RFC3339)
			}
			if !st.ChangedAt.IsZero() {
				v.ChangedAt = st.ChangedAt.Format(time.RFC3339)
			}
			views = append(views, v)
		}
		writeJSON(w, http.StatusOK, views)
	case http.MethodPost:
		var req struct {
			Name       string  `json:"name"`
			Expr       string  `json:"expr"`
			Op         string  `json:"op"`
			Value      float64 `json:"value"`
			ForSecs    int64   `json:"for_secs"`
			WebhookURL string  `json:"webhook_url"`
			Enabled    *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		rule, err := s.store.AddAlertRule(store.AlertRule{
			Name: req.Name, Expr: req.Expr, Op: req.Op, Value: req.Value,
			ForSecs: req.ForSecs, WebhookURL: req.WebhookURL, Enabled: enabled,
		})
		if err != nil {
			http.Error(w, "add alert rule: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, rule)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMigrateDatadog: POST a Datadog dashboard export JSON body,
// receive the dry-run diff report (mapped/partial/unmapped/risks).
// The GUI's migrate screen calls this; the CLI reads a file instead.
func (s *Server) handleMigrateDatadog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var dash datadog.Dashboard
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<20)).Decode(&dash); err != nil {
		http.Error(w, "not a valid Datadog dashboard export: "+err.Error(), http.StatusBadRequest)
		return
	}
	report := datadog.Diff(dash)
	writeJSON(w, http.StatusOK, report)
}

// handleAlertSubresources: GET /api/alerts/{id}/history and DELETE
// /api/alerts/{id}.
func (s *Server) handleAlertSubresources(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}
	// /api/alerts/{id}/history
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		id, err := strconv.ParseInt(rest[:idx], 10, 64)
		if err != nil || rest[idx+1:] != "history" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}
		events, err := s.store.AlertHistory(id, limit)
		if err != nil {
			http.Error(w, "alert history: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, events)
		return
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		http.Error(w, "invalid alert id "+rest, http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.store.DeleteAlertRule(id); err != nil {
		http.Error(w, "delete alert rule: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(v)
}
