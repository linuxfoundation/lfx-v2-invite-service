// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"net/http"
)

// readinessReporter is implemented by transports that can report whether their
// underlying connection is usable (e.g. the NATS client).
type readinessReporter interface {
	IsReady() error
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
