package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// SampleService is an in-process HTTP server that stands in for the customer's
// deployed workload. `deploy` starts it; health/smoke checks hit it for real.
type SampleService struct {
	cfg      ServiceConfig
	srv      *http.Server
	listener net.Listener
	reqs     int64
	errs     int64
	started  time.Time
}

// NewSampleService binds a listener on the config's port (0 picks a free port).
func NewSampleService(cfg ServiceConfig) (*SampleService, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Customer.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// fall back to an ephemeral port if the requested one is taken
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
	}
	s := &SampleService{cfg: cfg, listener: ln, started: time.Now()}
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Endpoints.Health, s.wrap(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	mux.HandleFunc(cfg.Endpoints.Ready, s.wrap(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ready": true, "replicas": cfg.Customer.Replicas})
	}))
	mux.HandleFunc(cfg.Endpoints.Metrics, s.wrap(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"requests": atomic.LoadInt64(&s.reqs),
			"errors":   atomic.LoadInt64(&s.errs),
			"uptimeMs": time.Since(s.started).Milliseconds(),
		})
	}))
	mux.HandleFunc(cfg.Endpoints.API, s.wrap(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			body = []byte(`{"ping":"pong"}`)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	s.srv = &http.Server{Handler: mux}
	return s, nil
}

func (s *SampleService) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&s.reqs, 1)
		h(w, r)
	}
}

// Addr returns the bound host:port.
func (s *SampleService) Addr() string { return s.listener.Addr().String() }

// BaseURL returns the http base URL for the running service.
func (s *SampleService) BaseURL() string { return "http://" + s.Addr() }

// Start begins serving in a goroutine.
func (s *SampleService) Start() {
	go func() { _ = s.srv.Serve(s.listener) }()
}

// Stop gracefully shuts the server down.
func (s *SampleService) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
