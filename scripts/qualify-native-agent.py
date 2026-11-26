#!/usr/bin/env python3
"""Exercise a native agent on a disposable, backed-up systemd VM.

Requires the controller service stopped and no pre-existing native agent config.
Temporarily replaces the local Caddy configuration. The caller must restart the
original controller afterwards to restore its configuration. Never use production.
"""

import argparse
import grp
import http.cookiejar
import json
import os
from pathlib import Path
import pwd
import secrets
import sqlite3
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--assets-dir", required=True, type=Path)
    parser.add_argument("--provision-via-ssh", action="store_true")
    args = parser.parse_args()
    if os.geteuid() != 0 or Path("/etc/posternproxy-agent/config.env").exists():
        raise RuntimeError("requires root and no existing agent configuration")
    if subprocess.run(["systemctl", "is-active", "--quiet", "posternproxy"]).returncode == 0:
        raise RuntimeError("stop and back up the existing controller first")

    account = pwd.getpwnam("posternproxy")
    cert_gid = grp.getgrnam("posternproxy-certs").gr_gid
    work = Path(tempfile.mkdtemp(prefix="posternproxy-qualification-", dir="/var/lib"))
    os.chown(work, account.pw_uid, cert_gid)
    os.chmod(work, 0o700)
    password = secrets.token_urlsafe(32)
    env = dict(os.environ, POSTERNPROXY_LISTEN="127.0.0.1:18181",
               POSTERNPROXY_DATA_DIR=str(work), POSTERNPROXY_DB_PATH=str(work / "test.db"),
               POSTERNPROXY_CERT_DIR=str(work / "certs"),
               POSTERNPROXY_CERT_RUNTIME_DIR=str(work / "runtime"),
               POSTERNPROXY_CADDY_API="http://127.0.0.1:29999",
               POSTERNPROXY_CONTROLLER_URL="ws://127.0.0.1:18181",
               POSTERNPROXY_ADMIN_EMAIL="qualification@example.test",
               POSTERNPROXY_ADMIN_PASSWORD=password, POSTERNPROXY_BEHIND_TLS_PROXY="false")
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    token = ""

    def api(path, data=None, content_type="application/json"):
        headers = {"Content-Type": content_type}
        if token:
            headers["Authorization"] = "Bearer " + token
        for cookie in jar:
            if cookie.name == "csrf_token":
                headers["X-CSRF-Token"] = cookie.value
        if data is not None and not isinstance(data, bytes):
            data = json.dumps(data).encode()
        request = urllib.request.Request("http://127.0.0.1:18181" + path, data=data, headers=headers)
        try:
            with client.open(request, timeout=15) as response:
                result = json.load(response)
        except urllib.error.HTTPError as error:
            # Error payloads can contain input; keep credentials out of output.
            raise RuntimeError(f"API {path} returned HTTP {error.code}") from None
        return result.get("data", result) if isinstance(result, dict) else result

    def wait_for(check, description):
        for _ in range(720):
            try:
                if check():
                    print("PASS " + description, flush=True)
                    return
            except (OSError, RuntimeError, urllib.error.URLError):
                pass
            time.sleep(0.5)
        raise RuntimeError("timed out: " + description)

    def demote():
        os.setgroups([cert_gid])
        os.setgid(cert_gid)
        os.setuid(account.pw_uid)

    log_path = work / "controller.log"
    log = log_path.open("wb")
    os.chmod(log_path, 0o600)
    controller = subprocess.Popen(["/usr/local/bin/posternproxy"], env=env, cwd=work,
                                  preexec_fn=demote, stdout=log, stderr=log)
    authorized_keys = Path("/root/.ssh/authorized_keys")
    qualification_key = None
    try:
        wait_for(lambda: client.open("http://127.0.0.1:18181/api/version", timeout=2).status == 200,
                 "isolated controller ready")
        login = api("/api/auth/login", {"email": "qualification@example.test", "password": password})
        token = login["access_token"]
        api("/api/servers/")  # Issue the CSRF cookie before mutations.
        registration = {"name": "qualification-agent", "hostname": "127.0.0.1", "deployment_type": "manual"}
        if args.provision_via_ssh:
            private_key = work / "ssh-key"
            run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(private_key))
            qualification_key = b"\n" + private_key.with_suffix(".pub").read_bytes().strip() + b"\n"
            with authorized_keys.open("ab") as keys:
                keys.write(qualification_key)
            registration.update(deployment_type="systemd", ssh_user="root", ssh_port=22,
                                ssh_auth_method="key", ssh_credentials=private_key.read_text())
        server = api("/api/servers/", registration)
        if not args.provision_via_ssh:
            enrollment = work / "agent.json"
            enrollment.write_text(json.dumps(server["enrollment"]))
            os.chmod(enrollment, 0o600)
            with (work / "installer.log").open("wb") as output:
                os.chmod(work / "installer.log", 0o600)
                run(str(args.assets_dir.resolve() / "posternproxy-agent"), "install", "--assets-dir",
                    str(args.assets_dir.resolve()), "--config-file", str(enrollment), "--skip-packages",
                    stdout=output, stderr=output, timeout=180)
            print("PASS native agent installer", flush=True)

        database = sqlite3.connect(work / "test.db")

        def applied():
            state = database.execute("SELECT status, desired_revision, applied_revision FROM server_config_state WHERE server_id=?", (server["id"],)).fetchone()
            return state and state[0] == "applied" and state[1] == state[2]

        wait_for(applied, "authenticated agent applied revision")
        if args.provision_via_ssh:
            wait_for(lambda: api(f'/api/servers/{server["id"]}')["status"] == "online",
                     "actual SSH provisioning reports authenticated/applied readiness")
        key, cert = work / "key.pem", work / "cert.pem"
        run("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
            "-subj", "/CN=agent-qualification.test", "-addext", "subjectAltName=DNS:agent-qualification.test",
            "-keyout", str(key), "-out", str(cert), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        os.chmod(key, 0o600)
        boundary = "postern" + secrets.token_hex(16)
        parts = []
        for name, value, filename in [("name", b"Qualification", None),
                                      ("domain_names", b'["agent-qualification.test"]', None),
                                      ("cert_file", cert.read_bytes(), "cert.pem"),
                                      ("key_file", key.read_bytes(), "key.pem")]:
            disposition = f'Content-Disposition: form-data; name="{name}"'
            if filename:
                disposition += f'; filename="{filename}"'
            parts.append(f"--{boundary}\r\n{disposition}\r\n\r\n".encode() + value + b"\r\n")
        uploaded = api("/api/certificates/upload", b"".join(parts) + f"--{boundary}--\r\n".encode(),
                       "multipart/form-data; boundary=" + boundary)
        api("/api/dead-hosts/", {"server_id": server["id"], "domain_names": ["agent-qualification.test"],
                                "certificate_id": uploaded["id"], "status_code": 200, "custom_body": "qualified-agent"})
        wait_for(applied, "agent custom certificate applied")

        def tls_traffic():
            result = subprocess.run(["curl", "--silent", "--show-error", "--noproxy", "*", "--cacert", str(cert),
                                     "--resolve", "agent-qualification.test:443:127.0.0.1",
                                     "https://agent-qualification.test"], capture_output=True, timeout=5)
            return result.returncode == 0 and result.stdout == b"qualified-agent"

        wait_for(tls_traffic, "verified HTTPS through systemd Caddy")
        runtime = Path("/run/posternproxy-agent-certs")
        materials = list(runtime.rglob("*.pem"))
        if len(materials) != 2:
            raise RuntimeError("expected exactly one runtime certificate pair")
        caddy = pwd.getpwnam("caddy")
        nobody = pwd.getpwnam("nobody")
        for material in materials:
            if material.stat().st_mode & 0o777 != 0o640 or material.stat().st_gid != cert_gid:
                raise RuntimeError("runtime material mode or group is incorrect")
            run("setpriv", f"--reuid={caddy.pw_uid}", f"--regid={caddy.pw_gid}", f"--groups={cert_gid}",
                "test", "-r", str(material))
            denied = subprocess.run(["setpriv", f"--reuid={nobody.pw_uid}", f"--regid={nobody.pw_gid}",
                                     "--clear-groups", "test", "-r", str(material)])
            if denied.returncode == 0:
                raise RuntimeError("unrelated user can read runtime certificate")
        print("PASS Caddy group read and unrelated-user denial", flush=True)
        for service in ["posternproxy-agent", "caddy"]:
            run("systemctl", "restart", service)
            wait_for(tls_traffic, service + " restart restored HTTPS")
        database.close()
    finally:
        subprocess.run(["systemctl", "stop", "posternproxy-agent"], check=False)
        subprocess.run(["systemctl", "disable", "posternproxy-agent"], check=False,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        controller.terminate()
        controller.wait(timeout=15)
        log.close()
        if qualification_key:
            authorized_keys.write_bytes(authorized_keys.read_bytes().replace(qualification_key, b""))
        # Test enrollment is deliberately short-lived and must not survive this run.
        for path in [Path("/etc/posternproxy-agent/config.env"), work / "agent.json", work / "key.pem", work / "ssh-key", work / "ssh-key.pub"]:
            path.unlink(missing_ok=True)
        print("Qualification artifacts retained privately in " + str(work), flush=True)


if __name__ == "__main__":
    main()
