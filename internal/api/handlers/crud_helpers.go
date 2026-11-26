package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// parseID extracts the "id" URL parameter and parses it as int64.
// Used directly by handlers that need the raw error; most should use requireID.
func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// Helpers shared by the host CRUD handlers (proxy_hosts, redirection_hosts,
// dead_hosts, stream_hosts). Each does one of the small bits
// of plumbing those handlers used to inline — parse an ID, decode a JSON
// body, run the standard "list with optional ?server_id= filter" pattern.
//
// All helpers return a bool that says "ok to continue"; when false, a response
// has already been written and the caller should return.

// requireID parses the chi "id" URL parameter as int64. On failure, writes a
// 400 and returns ok=false.
func requireID(w http.ResponseWriter, r *http.Request) (id int64, ok bool) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// decodeOrFail decodes the JSON request body into v. On failure, writes a 400
// and returns ok=false.
func decodeOrFail(w http.ResponseWriter, r *http.Request, v any) (ok bool) {
	if err := decodeJSON(r, v); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// listWithServerFilter implements the standard "list, optionally filtered by
// ?server_id=" pattern that every host handler used to inline. The two
// closures encapsulate the resource-specific service calls and return types.
//
// On error, writes "failed to list <pluralNoun>" with a 500 and returns
// ok=false; callers should return without writing further.
//
// Returns (items, true) on success — callers handle nil-to-empty-slice
// conversion themselves since this helper doesn't know the concrete type.
func listWithServerFilter(
	w http.ResponseWriter, r *http.Request,
	fetchAll func(context.Context) (any, error),
	fetchByServer func(context.Context, int64) (any, error),
	pluralNoun string,
) (items any, ok bool) {
	var err error
	if s := r.URL.Query().Get("server_id"); s != "" {
		if id, e := strconv.ParseInt(s, 10, 64); e == nil && id > 0 {
			items, err = fetchByServer(r.Context(), id)
		} else {
			items, err = fetchAll(r.Context())
		}
	} else {
		items, err = fetchAll(r.Context())
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list "+pluralNoun)
		return nil, false
	}
	return items, true
}
