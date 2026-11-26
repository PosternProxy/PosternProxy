//go:build integration

package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/certstore"
)

func TestLiveCustomTLSAndPortableRecovery(t *testing.T) {
	binary := os.Getenv("POSTERNPROXY_TEST_CADDY")
	if binary == "" {
		t.Fatal("set POSTERNPROXY_TEST_CADDY")
	}
	port := func() string {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := l.Addr().String()
		l.Close()
		return address
	}
	adminAddress, httpsAddress := port(), port()
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap.json")
	boot, _ := json.Marshal(map[string]any{"admin": map[string]string{"listen": adminAddress}, "apps": map[string]any{}})
	if err := os.WriteFile(path, boot, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(dir, "caddy.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "run", "--config", path)
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+dir, "XDG_CONFIG_HOME="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
		output.Close()
		if t.Failed() {
			data, _ := os.ReadFile(output.Name())
			t.Log(string(data))
		}
	})
	client := &http.Client{Timeout: 5 * time.Second}
	ready := false
	for range 100 {
		response, err := client.Get("http://" + adminAddress + "/config/")
		if err == nil {
			response.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Caddy did not start")
	}
	reject := false
	shim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reject {
			response, err := client.Post("http://"+adminAddress+"/load", "application/json", bytes.NewBufferString(`{"apps":{"http":{"servers":{"broken":{"listen":["127.0.0.1:0"],"routes":[{"handle":[{"handler":"missing_test_handler"}]}]}}}}}`))
			if err != nil {
				t.Error(err)
				w.WriteHeader(502)
				return
			}
			defer response.Body.Close()
			w.WriteHeader(response.StatusCode)
			io.Copy(w, response.Body)
			return
		}
		var cfg caddy.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var app caddy.HTTPApp
		if err := json.Unmarshal(cfg.Apps["http"], &app); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		delete(app.Servers, "http")
		app.Servers["https"].Listen = []string{httpsAddress}
		app.Servers["https"].AutomaticHTTPS = &caddy.AutoHTTPS{DisableRedirects: true}
		cfg.Apps["http"], _ = json.Marshal(app)
		cfg.Admin.Listen = adminAddress
		raw, _ := json.Marshal(cfg)
		response, err := client.Post("http://"+adminAddress+"/load", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Error(err)
			w.WriteHeader(502)
			return
		}
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
	}))
	defer shim.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "recovered-tls-upstream") }))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	upstreamPort, _ := strconv.Atoi(u.Port())
	db := testDB(t)
	store := certstore.Store{Root: filepath.Join(dir, "source"), Key: [32]byte{1}}
	certPEM, keyPEM := acceptanceCertificatePair(t, "tls.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	svc := NewCertificateService(db, store.Root, NewAuditService(db), store.Key)
	cert, err := svc.UploadCustom(context.Background(), nil, "TLS", json.RawMessage(`["tls.example.com"]`), bytes.NewReader(certPEM), bytes.NewReader(keyPEM), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO proxy_hosts(domain_names,forward_host,forward_port,certificate_id,caddy_id) VALUES('["tls.example.com"]',?,?,?,'live_tls')`, u.Hostname(), upstreamPort, cert.ID); err != nil {
		t.Fatal(err)
	}
	syncSvc := NewCaddySyncService(db, caddy.NewClient(shim.URL), nil, "test")
	runtimeDir := filepath.Join(dir, "runtime")
	syncSvc.ConfigureCertificates(store, runtimeDir)
	if err := syncSvc.FullSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	certBlock, _ := pem.Decode(certPEM)
	leaf, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	checkTLS := func() {
		t.Helper()
		transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "tls.example.com", MinVersion: tls.VersionTLS12}}
		defer transport.CloseIdleConnections()
		httpsClient := &http.Client{Transport: transport, Timeout: 3 * time.Second}
		request, _ := http.NewRequest("GET", "https://"+httpsAddress, nil)
		request.Host = "tls.example.com"
		response, err := httpsClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if string(body) != "recovered-tls-upstream" || !bytes.Equal(response.TLS.PeerCertificates[0].Raw, leaf.Raw) {
			t.Fatal("wrong TLS certificate or upstream")
		}
	}
	checkTLS()
	backup := NewBackupService(db, nil)
	backup.ConfigureCertificates(store)
	archive, err := backup.ExportPortable(context.Background(), "disposable-recovery-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	destinationDB := testDB(t)
	destinationStore := certstore.Store{Root: filepath.Join(dir, "new-source"), Key: [32]byte{2}}
	destinationSync := NewCaddySyncService(destinationDB, caddy.NewClient(shim.URL), nil, "test")
	destinationSync.ConfigureCertificates(destinationStore, filepath.Join(dir, "new-runtime"))
	recovery := NewBackupService(destinationDB, destinationSync)
	recovery.ConfigureCertificates(destinationStore)
	if _, err := recovery.RestorePortable(context.Background(), archive, PortableRestoreOptions{Passphrase: "disposable-recovery-passphrase"}); err != nil {
		t.Fatal(err)
	}
	checkTLS()
	reject = true
	if err := destinationSync.FullSync(context.Background()); err == nil {
		t.Fatal("rejected apply succeeded")
	}
	checkTLS()
}
