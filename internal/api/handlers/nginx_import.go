package handlers

import (
	"io"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/services"
)

// NginxImportHandler provides HTTP handler methods for importing nginx configuration files.
type NginxImportHandler struct {
	svc *services.NginxImportService
}

// NewNginxImportHandler creates a NginxImportHandler backed by the given service.
func NewNginxImportHandler(svc *services.NginxImportService) *NginxImportHandler {
	return &NginxImportHandler{svc: svc}
}

// Preview parses an uploaded nginx config file and returns a ParseResult without creating any entities.
func (h *NginxImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	content, err := readNginxFile(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.svc.Preview(content)
	if err != nil {
		http.Error(w, "parse failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Confirm re-parses the uploaded nginx config and creates entities from the
// resulting hosts. The client must re-upload the same file; the ParseResult
// is never accepted from the request body.
func (h *NginxImportHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	content, err := readNginxFile(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.svc.Preview(content)
	if err != nil {
		http.Error(w, "parse failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	claims := auth.UserFromContext(r.Context())
	var callerID *int64
	if claims != nil {
		id := claims.UserID
		callerID = &id
	}

	summary, err := h.svc.Confirm(r.Context(), callerID, result, r.RemoteAddr)
	if err != nil {
		http.Error(w, "confirm failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// readNginxFile reads the "file" field from a multipart/form-data upload.
// Falls back to reading the raw body if not multipart.
func readNginxFile(r *http.Request) ([]byte, error) {
	const maxSize = 10 << 20 // 10 MB

	ct := r.Header.Get("Content-Type")
	if len(ct) >= 9 && ct[:9] == "multipart" {
		if err := r.ParseMultipartForm(maxSize); err != nil { // #nosec G120 -- upload size is bounded by maxSize (10 MB)
			return nil, err
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(io.LimitReader(f, maxSize))
	}

	// Plain body (e.g. application/octet-stream or text/plain).
	return io.ReadAll(io.LimitReader(r.Body, maxSize))
}
