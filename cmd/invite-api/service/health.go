// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// ListenHealth binds the health server's TCP port. Binding up front, before
// the slow startup work, surfaces a busy or invalid port as an immediate error
// instead of a late failure after NATS has connected.
func ListenHealth(port string) (net.Listener, error) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("listen on health port %q: %w", port, err)
	}
	return ln, nil
}

// readinessReporter is implemented by transports that can report whether their
// underlying connection is usable (e.g. the NATS client).
type readinessReporter interface {
	IsReady() error
}

// flusher is implemented by transports that can confirm the broker has
// processed everything sent so far (the NATS client).
type flusher interface {
	Flush(timeout time.Duration) error
}

// Flush blocks until the broker has processed all subscriptions registered by
// Start, or returns an error if timeout elapses. Subscribe calls are
// asynchronous, so call this after Start and before reporting ready. A
// subscriber that does not implement Flush (such as a test double) has nothing
// to flush and returns nil.
func (s *Server) Flush(timeout time.Duration) error {
	if f, ok := s.subscriber.(flusher); ok {
		return f.Flush(timeout)
	}
	return nil
}

// Ready reports whether the server can currently serve requests. It returns an
// error when the underlying transport reports it is not usable (for NATS: not
// connected or draining). A subscriber that does not implement IsReady (such as
// a test double) is treated as ready.
func (s *Server) Ready() error {
	if r, ok := s.subscriber.(readinessReporter); ok {
		return r.IsReady()
	}
	return nil
}

// HealthHandler returns an http.Handler serving the Kubernetes probe endpoints:
//
//   - GET /livez: always 200 while the process is serving HTTP. It does not
//     inspect NATS state, so a broker outage never restarts the pod.
//   - GET /readyz: 200 when ready returns nil, otherwise 503.
//
// The invite service is a pure NATS subscriber, so these are the only HTTP
// routes it exposes.
func HealthHandler(ready func() error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if err := ready(); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
