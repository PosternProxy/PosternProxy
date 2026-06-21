package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTLSProbe(t *testing.T) {
	// httptest's built-in TLS cert is valid for example.com and 127.0.0.1.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	svc := NewTLSProbeService(nil, ts.Listener.Addr().String())

	t.Run("matching domain is valid", func(t *testing.T) {
		st := svc.probe("example.com")
		if st.Status != TLSProbeValid {
			t.Errorf("got %q (%s), want %q", st.Status, st.Detail, TLSProbeValid)
		}
		if st.ExpiresAt == nil {
			t.Error("expected an expiry to be reported")
		}
	})

	t.Run("non-matching domain is pending", func(t *testing.T) {
		st := svc.probe("not-in-cert.test")
		if st.Status != TLSProbePending {
			t.Errorf("got %q, want %q", st.Status, TLSProbePending)
		}
	})
}

func TestTLSProbeUnreachable(t *testing.T) {
	svc := NewTLSProbeService(nil, "127.0.0.1:1") // nothing listening
	st := svc.probe("example.com")
	if st.Status != TLSProbeFailed {
		t.Errorf("got %q, want %q", st.Status, TLSProbeFailed)
	}
}
