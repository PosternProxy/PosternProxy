package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/services"
)

// CertificateHandler provides HTTP handler methods for the Certificate resource.
type CertificateHandler struct {
	svc *services.CertificateService
}

// NewCertificateHandler creates a CertificateHandler backed by the given service.
func NewCertificateHandler(svc *services.CertificateService) *CertificateHandler {
	return &CertificateHandler{svc: svc}
}

// List writes all Certificate records to the response.
func (h *CertificateHandler) List(w http.ResponseWriter, r *http.Request) {
	certs, err := h.svc.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list certificates")
		return
	}
	if certs == nil {
		respondJSON(w, http.StatusOK, []any{})
		return
	}
	respondJSON(w, http.StatusOK, certs)
}

// Get writes a single Certificate record looked up by URL parameter "id".
func (h *CertificateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cert, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get certificate")
		return
	}
	if cert == nil {
		respondError(w, http.StatusNotFound, "certificate not found")
		return
	}
	respondJSON(w, http.StatusOK, cert)
}

// Upload accepts a multipart form upload (cert_file, key_file) and stores a custom Certificate record.
func (h *CertificateHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil { // #nosec G120 -- body already capped by MaxBytesReader(5 MiB) above
		respondError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	name := r.FormValue("name")
	domainsRaw := r.FormValue("domain_names")
	if domainsRaw == "" {
		domainsRaw = "[]"
	}

	certFile, _, err := r.FormFile("cert_file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "cert_file is required")
		return
	}
	defer certFile.Close()

	keyFile, _, err := r.FormFile("key_file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "key_file is required")
		return
	}
	defer keyFile.Close()

	cert, err := h.svc.UploadCustom(
		r.Context(), &claims.UserID, name, json.RawMessage(domainsRaw),
		certFile, keyFile, r.RemoteAddr,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, cert)
}

// Delete removes a Certificate record by URL parameter "id".
func (h *CertificateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), &claims.UserID, id, r.RemoteAddr); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
