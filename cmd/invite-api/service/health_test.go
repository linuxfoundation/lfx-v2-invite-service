// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linuxfoundation/lfx-v2-invite-service/internal/domain/port/mocks"
)

// readyMock is a subscriber that also reports readiness, like the NATS client.
type readyMock struct {
	mocks.Subscriber
	err error
}

func (m *readyMock) IsReady() error { return m.err }

func TestHealthHandler(t *testing.T) {
	errDown := errors.New("down")

	tests := []struct {
		name   string
		method string
		path   string
		ready  func() error
		want   int
	}{
		{"livez ok", http.MethodGet, "/livez", func() error { return nil }, http.StatusOK},
		{"livez ignores readiness", http.MethodGet, "/livez", func() error { return errDown }, http.StatusOK},
		{"readyz ready", http.MethodGet, "/readyz", func() error { return nil }, http.StatusOK},
		{"readyz not ready", http.MethodGet, "/readyz", func() error { return errDown }, http.StatusServiceUnavailable},
		{"unknown path", http.MethodGet, "/other", func() error { return nil }, http.StatusNotFound},
		{"livez rejects POST", http.MethodPost, "/livez", func() error { return nil }, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			HealthHandler(tt.ready).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.want {
				t.Errorf("%s %s: got status %d, want %d", tt.method, tt.path, rec.Code, tt.want)
			}
		})
	}
}

func TestServer_Ready(t *testing.T) {
	t.Run("nil subscriber is ready", func(t *testing.T) {
		if err := NewServer(nil, nil, nil, nil, nil).Ready(); err != nil {
			t.Errorf("Ready() = %v, want nil", err)
		}
	})
	t.Run("subscriber without IsReady is ready", func(t *testing.T) {
		if err := NewServer(&mocks.Subscriber{}, nil, nil, nil, nil).Ready(); err != nil {
			t.Errorf("Ready() = %v, want nil", err)
		}
	})
	t.Run("delegates to subscriber IsReady", func(t *testing.T) {
		errDown := errors.New("nats down")
		srv := NewServer(&readyMock{err: errDown}, nil, nil, nil, nil)
		if err := srv.Ready(); !errors.Is(err, errDown) {
			t.Errorf("Ready() = %v, want %v", err, errDown)
		}
		srv = NewServer(&readyMock{}, nil, nil, nil, nil)
		if err := srv.Ready(); err != nil {
			t.Errorf("Ready() = %v, want nil", err)
		}
	})
}

func TestAppConfigFromEnv_Port(t *testing.T) {
	t.Setenv("PORT", "")
	if got := AppConfigFromEnv().Port; got != "8080" {
		t.Errorf("default Port = %q, want 8080", got)
	}
	t.Setenv("PORT", "9090")
	if got := AppConfigFromEnv().Port; got != "9090" {
		t.Errorf("Port = %q, want 9090", got)
	}
}
