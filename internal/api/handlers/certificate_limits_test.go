package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/auth"
)

func TestCertificateMultipartBodyIsBounded(t *testing.T) {
	tokens, err := auth.GenerateTokenPair("test-secret", 1, "admin@example.test", "admin")
	if err != nil {
		t.Fatal(err)
	}
	body := "--boundary\r\nContent-Disposition: form-data; name=\"unused\"\r\n\r\n" + strings.Repeat("x", 6<<20) + "\r\n--boundary--\r\n"
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/upload", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	w := httptest.NewRecorder()
	handler := auth.Middleware("test-secret", nil)(http.HandlerFunc(NewCertificateHandler(nil).Upload))
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "request body too large") {
		t.Fatalf("oversized multipart request: %d %s", w.Code, w.Body.String())
	}
}
