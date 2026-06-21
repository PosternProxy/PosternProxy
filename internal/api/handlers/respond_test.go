package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSONEncodesNilSliceAsEmptyArray(t *testing.T) {
	recorder := httptest.NewRecorder()
	var items []string

	respondJSON(recorder, http.StatusOK, items)

	if got := recorder.Body.String(); got != "[]\n" {
		t.Fatalf("response body = %q, want %q", got, "[]\\n")
	}
}

func TestRespondJSONPreservesNilValue(t *testing.T) {
	recorder := httptest.NewRecorder()

	respondJSON(recorder, http.StatusOK, nil)

	if got := recorder.Body.String(); got != "null\n" {
		t.Fatalf("response body = %q, want %q", got, "null\\n")
	}
}
