package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentCaddyOperationsCancelOnStop(t *testing.T) {
	for _, operation := range []string{"load", "version", "routes"} {
		t.Run(operation, func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a := agentClient{ctx: ctx, caddyAPI: server.URL}
			done := make(chan struct{})
			go func() {
				defer close(done)
				switch operation {
				case "load":
					_ = a.caddyLoad(nil)
				case "version":
					a.queryCaddyVersion()
				case "routes":
					a.queryCaddyRouteCount()
				}
			}()
			<-started
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("service stop did not interrupt Caddy operation")
			}
		})
	}
}
