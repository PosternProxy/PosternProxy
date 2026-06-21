package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	"github.com/posternproxy/posternproxy/internal/auth"
)

type errorResponse struct {
	Error string `json:"error"`
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	v = normalizeJSONValue(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func normalizeJSONValue(v any) any {
	if v == nil {
		return v
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}

// RespondJSON is the exported form of respondJSON for use outside the handlers package.
func RespondJSON(w http.ResponseWriter, status int, v any) {
	respondJSON(w, status, v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, errorResponse{Error: msg})
}

// maxRequestBody is the maximum size of a JSON request body (1 MB).
const maxRequestBody = 1 << 20

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxRequestBody)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// requireClaims returns the JWT claims from the request context, or writes a
// 401 and returns nil if they are missing. Callers should return immediately
// when nil is returned.
func requireClaims(w http.ResponseWriter, r *http.Request) *auth.Claims {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}
	return claims
}
