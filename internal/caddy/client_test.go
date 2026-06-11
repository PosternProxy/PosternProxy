package caddy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Load_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/load" {
			t.Errorf("path = %s, want /load", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("expected non-empty body")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	cfg := &Config{}
	if err := client.Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestClient_Load_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.Load(&Config{})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestClient_GetConfig_Success(t *testing.T) {
	expected := `{"apps":{"http":{}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expected))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if string(result) != expected {
		t.Errorf("result = %s, want %s", string(result), expected)
	}
}

func TestClient_SetPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/config/apps/http/servers/srv0" {
			t.Errorf("path = %s, want /config/apps/http/servers/srv0", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.SetPath("apps/http/servers/srv0", map[string]string{"listen": ":80"}); err != nil {
		t.Fatalf("SetPath: %v", err)
	}
}

func TestClient_DeletePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.DeletePath("apps/http/servers/srv0/routes/0"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}
}

func TestClient_GetUpstreams(t *testing.T) {
	upstreams := []UpstreamStatus{
		{Address: "10.0.0.1:8080", NumRequests: 100, Fails: 2},
		{Address: "10.0.0.2:8080", NumRequests: 95, Fails: 0},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(upstreams)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.GetUpstreams()
	if err != nil {
		t.Fatalf("GetUpstreams: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Address != "10.0.0.1:8080" {
		t.Errorf("address = %s, want 10.0.0.1:8080", result[0].Address)
	}
	if result[1].Fails != 0 {
		t.Errorf("fails = %d, want 0", result[1].Fails)
	}
}

func TestClient_Ping_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestClient_Ping_Unreachable(t *testing.T) {
	client := NewClient("http://localhost:0")
	if err := client.Ping(); err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestClient_PatchPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.PatchPath("apps/http", map[string]bool{"grace_period": true}); err != nil {
		t.Fatalf("PatchPath: %v", err)
	}
}

func TestClient_AppendToArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/config/apps/http/servers/srv0/routes" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if err := client.AppendToArray("apps/http/servers/srv0/routes", map[string]string{"handle": "static"}); err != nil {
		t.Fatalf("AppendToArray: %v", err)
	}
}
