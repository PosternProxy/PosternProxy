package provisioner

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func installDockerAgent(input ProvisionInput, c *sshClient, log func(string)) error {
	var uid bytes.Buffer
	if err := c.run("id -u", &uid); err != nil {
		return err
	}
	prefix := ""
	if strings.TrimSpace(uid.String()) != "0" {
		uid.Reset()
		if err := c.run("sudo -n id -u", &uid); err != nil || strings.TrimSpace(uid.String()) != "0" {
			return fmt.Errorf("docker provisioning requires root or passwordless sudo")
		}
		prefix = "sudo -n "
	}
	// Docker installation is an operator prerequisite; never execute a downloaded shell script.
	if err := c.run(prefix+"docker info >/dev/null", io.Discard); err != nil {
		return fmt.Errorf("install and start Docker before provisioning")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	dir := "/tmp/posternproxy-provision-" + hex.EncodeToString(nonce[:])
	if err := c.run("mkdir -m 0700 -- "+shellQuote(dir), io.Discard); err != nil {
		return err
	}
	defer func() {
		if err := c.run("rm -rf -- "+shellQuote(dir), io.Discard); err != nil {
			log("Private provisioning staging cleanup failed.")
		}
	}()
	env := fmt.Sprintf("POSTERNPROXY_CONTROLLER_URL=%s\nPOSTERNPROXY_AGENT_TOKEN=%s\nPOSTERNPROXY_SERVER_ID=%d\n", input.ControllerURL, input.AgentToken, input.ServerID)
	if err := c.upload(dir+"/agent.env", strings.NewReader(env)); err != nil {
		return err
	}
	image := "ghcr.io/posternproxy/posternproxy-agent:" + input.Version
	script := `set -eu
 docker pull "$1"
 if docker container inspect posternproxy-agent-previous >/dev/null 2>&1; then
   echo 'Remove or rename posternproxy-agent-previous before retrying provisioning' >&2; exit 1
 fi
 data_source=posternproxy-agent-data
 had_old=0
 if docker container inspect posternproxy-agent >/dev/null 2>&1; then
   previous_data="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' posternproxy-agent)"
   if [ -n "$previous_data" ]; then data_source="$previous_data"; fi
   docker stop posternproxy-agent >/dev/null
   docker rename posternproxy-agent posternproxy-agent-previous
   had_old=1
 fi
 if ! docker run -d --name posternproxy-agent --restart unless-stopped --env-file "$2" -v "$data_source:/data" -p 80:80 -p 443:443 -p 443:443/udp --cap-drop ALL --cap-add NET_BIND_SERVICE "$1"; then
   docker rm -f posternproxy-agent >/dev/null 2>&1 || true
   if [ "$had_old" = 1 ]; then docker rename posternproxy-agent-previous posternproxy-agent; docker start posternproxy-agent >/dev/null; fi
   exit 1
 fi
 echo 'Container started. Previous container, if any, is retained for operator rollback.'
`
	if err := c.upload(dir+"/install.sh", strings.NewReader(script)); err != nil {
		return err
	}
	writer := &lineWriter{onLine: log}
	err := c.run(prefix+"bash "+shellQuote(dir+"/install.sh")+" "+shellQuote(image)+" "+shellQuote(dir+"/agent.env"), writer)
	writer.flush()
	if err != nil {
		return fmt.Errorf("docker installation failed: %w", err)
	}
	return nil
}
