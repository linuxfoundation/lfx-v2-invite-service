// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// flushMock is a subscriber that also supports Flush, like the NATS client.
type flushMock struct {
	mocks.Subscriber
	err     error
	timeout time.Duration
}

func (m *flushMock) Flush(timeout time.Duration) error {
	m.timeout = timeout
	return m.err
}

func TestServer_Flush(t *testing.T) {
	t.Run("nil subscriber has nothing to flush", func(t *testing.T) {
		if err := NewServer(nil, nil, nil, nil, nil).Flush(time.Second); err != nil {
			t.Errorf("Flush() = %v, want nil", err)
		}
	})
	t.Run("subscriber without Flush has nothing to flush", func(t *testing.T) {
		if err := NewServer(&mocks.Subscriber{}, nil, nil, nil, nil).Flush(time.Second); err != nil {
			t.Errorf("Flush() = %v, want nil", err)
		}
	})
	t.Run("delegates to subscriber Flush with the timeout", func(t *testing.T) {
		errSlow := errors.New("flush timeout")
		m := &flushMock{err: errSlow}
		if err := NewServer(m, nil, nil, nil, nil).Flush(3 * time.Second); !errors.Is(err, errSlow) {
			t.Errorf("Flush() = %v, want %v", err, errSlow)
		}
		if m.timeout != 3*time.Second {
			t.Errorf("Flush timeout = %v, want 3s", m.timeout)
		}
		if err := NewServer(&flushMock{}, nil, nil, nil, nil).Flush(time.Second); err != nil {
			t.Errorf("Flush() = %v, want nil", err)
		}
	})
}

func TestListenHealth(t *testing.T) {
	// A free port (0 = OS-assigned) binds.
	ln, err := ListenHealth("0")
	if err != nil {
		t.Fatalf("ListenHealth(0) = %v, want nil", err)
	}
	defer ln.Close() //nolint:errcheck

	// The port is now taken: binding it again must fail with a clear error.
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	if _, err := ListenHealth(port); err == nil {
		t.Errorf("ListenHealth(%s) on a busy port = nil, want error", port)
	}

	// An invalid port fails too.
	if _, err := ListenHealth("not-a-port"); err == nil {
		t.Error("ListenHealth(not-a-port) = nil, want error")
	}
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
