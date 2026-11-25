package installer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
)

func TestAgentInstallWritesIdentityWithoutControllerService(t *testing.T) {
	i, stage, commands := fixtureInstallation(t)
	i.agentConfig = &agentconfig.Config{ControllerURL: "https://controller.test", Token: "literal$(touch /tmp/nope)'\"", ServerID: 23}
	if err := i.apply(context.Background(), stage); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(i.path("/etc/posternproxy-agent/config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "POSTERNPROXY_SERVER_ID=23") || !strings.Contains(string(config), "literal$(touch /tmp/nope)") {
		t.Fatal("agent identity was lost")
	}
	if _, err := os.Stat(i.path("/etc/systemd/system/posternproxy.service")); !os.IsNotExist(err) {
		t.Fatal("agent installed controller service")
	}
	for _, command := range *commands {
		if strings.HasPrefix(command, "ufw ") {
			t.Fatal("agent installer modified firewall by default")
		}
	}
}
func TestAgentControllerPreflightRejectsRedirectWithoutLeakingToken(t *testing.T) {
	received := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true; w.WriteHeader(204) }))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer disposable" || r.URL.Query().Get("server_id") != "2" {
			t.Error("missing identity")
		}
		http.Redirect(w, r, destination.URL, 302)
	}))
	defer origin.Close()
	err := verifyControllerIdentity(context.Background(), agentconfig.Config{ControllerURL: origin.URL, Token: "disposable", ServerID: 2})
	if err == nil || received || strings.Contains(err.Error(), "disposable") {
		t.Fatal("unsafe controller preflight", err)
	}
}
