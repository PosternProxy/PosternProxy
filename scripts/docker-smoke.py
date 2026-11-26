#!/usr/bin/env python3
"""Exercise versioned controller/agent images in an isolated disposable network."""

import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import re
import secrets
import socket
import ssl
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def docker(*args):
    result = subprocess.run(["docker", *args], capture_output=True, text=True, timeout=180)
    if result.returncode:
        raise RuntimeError("Docker operation failed: " + args[0])
    return result.stdout.strip()


def wait(check, description):
    for _ in range(120):
        try:
            if check():
                print("PASS " + description, flush=True)
                return
        except (OSError, ValueError, RuntimeError, urllib.error.URLError):
            pass
        time.sleep(0.5)
    raise RuntimeError("timed out: " + description)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--controller-image", required=True)
    parser.add_argument("--agent-image", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    prefix = "posternproxy-smoke-" + secrets.token_hex(6)
    controller, agent = prefix + "-controller", prefix + "-agent"
    volumes = [prefix + "-controller-data", prefix + "-agent-data"]
    containers = [controller, agent]
    network_created = False
    with tempfile.TemporaryDirectory(prefix="posternproxy-docker-smoke-") as temporary:
        work = Path(temporary)
        try:
            for image, binary in [(args.controller_image, "posternproxy"), (args.agent_image, "posternproxy-agent")]:
                identity = json.loads(docker("run", "--rm", "--entrypoint", binary, image, "version"))
                if identity.get("version") != args.version:
                    raise RuntimeError("image application version differs from release")
            docker("network", "create", prefix)
            network_created = True
            for volume in volumes:
                docker("volume", "create", volume)
            controller_env = work / "controller.env"
            password = secrets.token_urlsafe(24)
            controller_env.write_text("\n".join([
                "POSTERNPROXY_JWT_SECRET=" + secrets.token_hex(32),
                "POSTERNPROXY_ADMIN_EMAIL=smoke@example.test",
                "POSTERNPROXY_ADMIN_PASSWORD=" + password,
                "POSTERNPROXY_BEHIND_TLS_PROXY=false",
                "POSTERNPROXY_CONTROLLER_URL=http://" + controller + ":81", "",
            ]))
            controller_env.chmod(0o600)

            def compose_file(template, image, name, volume, environment, ports):
                source = Path(__file__).resolve().parent.parent / "docker" / template
                values = dict(os.environ, **environment)
                resolved = subprocess.run(["docker", "compose", "-f", str(source), "config", "--format", "json"],
                                          capture_output=True, text=True, env=values, timeout=30)
                if resolved.returncode:
                    raise RuntimeError("Compose template validation failed")
                configuration = json.loads(resolved.stdout)
                service_name = next(iter(configuration["services"]))
                service = configuration["services"][service_name]
                service.setdefault("environment", {}).update(environment)
                service.update(image=image, container_name=name, ports=ports, networks={"smoke": {}})
                service["restart"] = "no"
                volume_key = next(iter(configuration["volumes"]))
                configuration["volumes"] = {volume_key: {"external": True, "name": volume}}
                configuration["networks"] = {"smoke": {"external": True, "name": prefix}}
                path = work / (name + ".json")
                path.write_text(json.dumps(configuration))
                path.chmod(0o600)
                return path

            controller_values = dict(line.split("=", 1) for line in controller_env.read_text().splitlines())
            controller_compose = compose_file("docker-compose.yml", args.controller_image, controller,
                                              volumes[0], controller_values, ["127.0.0.1::81"])

            def start_controller():
                docker("compose", "-p", prefix, "-f", str(controller_compose), "up", "-d")

            def port(name, container_port):
                return int(docker("port", name, str(container_port) + "/tcp").rsplit(":", 1)[1])

            start_controller()
            base = "http://127.0.0.1:" + str(port(controller, 81))
            jar = http.cookiejar.CookieJar()
            client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
            token = ""

            def api(path, data=None, method=None, content_type="application/json"):
                headers = {"Content-Type": content_type}
                if token:
                    headers["Authorization"] = "Bearer " + token
                for cookie in jar:
                    if cookie.name == "csrf_token":
                        headers["X-CSRF-Token"] = cookie.value
                if data is not None and not isinstance(data, bytes):
                    data = json.dumps(data).encode()
                request = urllib.request.Request(base + "/api" + path, data=data, headers=headers, method=method)
                try:
                    with client.open(request, timeout=15) as response:
                        return json.load(response)
                except urllib.error.HTTPError as error:
                    raise RuntimeError(f"API {path} failed with HTTP {error.code}") from None

            def login():
                return api("/auth/login", {"email": "smoke@example.test", "password": password})["access_token"]

            wait(lambda: api("/version")["version"] == args.version, "controller readiness and version")
            token = login()
            server = api("/servers/", {"name": "Docker smoke agent", "hostname": agent, "deployment_type": "manual"})
            enrollment = server["enrollment"]
            agent_env = work / "agent.env"
            agent_env.write_text("\n".join([
                "POSTERNPROXY_CONTROLLER_URL=" + enrollment["controller_url"],
                "POSTERNPROXY_AGENT_TOKEN=" + enrollment["agent_token"],
                "POSTERNPROXY_SERVER_ID=" + str(server["id"]), "",
            ]))
            agent_env.chmod(0o600)
            agent_values = dict(line.split("=", 1) for line in agent_env.read_text().splitlines())
            agent_compose = compose_file("docker-compose.agent.yml", args.agent_image, agent,
                                         volumes[1], agent_values, ["127.0.0.1::80", "127.0.0.1::443"])
            docker("compose", "-p", prefix + "-agent", "-f", str(agent_compose), "up", "-d")

            def applied():
                states = api("/configuration")
                state = next((entry for entry in states if entry["server_id"] == server["id"]), {})
                return api(f'/servers/{server["id"]}/status')["online"] and state.get("status") == "applied" and state.get("desired_revision") == state.get("applied_revision")

            wait(applied, "authenticated outbound agent and applied revision")
            api("/dead-hosts/", {"server_id": server["id"], "domain_names": ["docker-smoke.test"],
                                 "http_only": True, "status_code": 200, "custom_body": "docker-smoke"})
            http_port, tls_port = port(agent, 80), port(agent, 443)

            def http_traffic():
                request = urllib.request.Request("http://127.0.0.1:" + str(http_port), headers={"Host": "docker-smoke.test"})
                with urllib.request.urlopen(request, timeout=3) as response:
                    return response.status == 200 and response.read() == b"docker-smoke"

            wait(http_traffic, "remote HTTP traffic")
            fixture = (Path(__file__).resolve().parent.parent / "frontend/e2e/testdata/cert-fixture.ts").read_text()
            cert = re.search(r"export const certPem = `([^`]+)`", fixture)[1]
            key = re.search(r"export const keyPem = `([^`]+)`", fixture)[1]
            boundary = "smoke" + secrets.token_hex(16)
            parts = []
            for name, value, filename in [("name", b"Docker TLS", None),
                                          ("domain_names", b'["cert-upload.e2e.test"]', None),
                                          ("cert_file", cert.encode(), "cert.pem"), ("key_file", key.encode(), "key.pem")]:
                disposition = f'Content-Disposition: form-data; name="{name}"'
                if filename:
                    disposition += f'; filename="{filename}"'
                parts.append(f"--{boundary}\r\n{disposition}\r\n\r\n".encode() + value + b"\r\n")
            certificate = api("/certificates/upload", b"".join(parts) + f"--{boundary}--\r\n".encode(),
                              content_type="multipart/form-data; boundary=" + boundary)
            api("/dead-hosts/", {"server_id": server["id"], "domain_names": ["cert-upload.e2e.test"],
                                 "certificate_id": certificate["id"], "status_code": 200, "custom_body": "docker-tls"})
            tls = ssl.create_default_context(cadata=cert)

            def tls_traffic():
                with tls.wrap_socket(socket.create_connection(("127.0.0.1", tls_port), timeout=3), server_hostname="cert-upload.e2e.test") as connection:
                    connection.sendall(b"GET / HTTP/1.1\r\nHost: cert-upload.e2e.test\r\nConnection: close\r\n\r\n")
                    data = b""
                    while chunk := connection.recv(4096):
                        data += chunk
                    return b"200 OK" in data.split(b"\r\n", 1)[0] and data.endswith(b"docker-tls")

            wait(tls_traffic, "verified custom TLS through agent")
            docker("restart", agent)
            http_port, tls_port = port(agent, 80), port(agent, 443)
            wait(lambda: applied() and http_traffic() and tls_traffic(), "agent restart and traffic recovery")
            docker("exec", agent, "caddy", "stop", "--address", "127.0.0.1:2019")
            wait(lambda: docker("inspect", "-f", "{{.State.Running}}", agent) == "false", "container exits when Caddy exits")
            docker("start", agent)
            http_port, tls_port = port(agent, 80), port(agent, 443)
            wait(lambda: applied() and tls_traffic(), "supervisor restart recovery")
            docker("rm", "-f", controller)
            start_controller()
            base = "http://127.0.0.1:" + str(port(controller, 81))
            wait(lambda: api("/version")["version"] == args.version, "replacement controller readiness")
            token = login()
            wait(lambda: applied() and tls_traffic(), "persistent configuration, secrets and certificates after replacement")
        finally:
            for name in containers:
                subprocess.run(["docker", "rm", "-f", name], capture_output=True)
            for name in volumes:
                subprocess.run(["docker", "volume", "rm", name], capture_output=True)
            if network_created:
                subprocess.run(["docker", "network", "rm", prefix], capture_output=True)


if __name__ == "__main__":
    main()
