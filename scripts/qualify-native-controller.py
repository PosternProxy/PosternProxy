#!/usr/bin/env python3
"""Qualify fresh controller data and installer reruns on a disposable systemd VM.

Existing configuration/data directories are moved aside while services are
stopped, then restored in finally. This is not a fresh-OS package qualification.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--assets-dir", required=True, type=Path)
    args = parser.parse_args()
    if os.geteuid() != 0:
        raise RuntimeError("requires root on a disposable, backed-up VM")
    work = Path(tempfile.mkdtemp(prefix="posternproxy-controller-qualification-", dir="/var/lib"))
    original = [Path("/etc/posternproxy"), Path("/var/lib/posternproxy")]
    moved = []

    def systemctl(*arguments):
        subprocess.run(["systemctl", *arguments], check=True)

    def install():
        result = subprocess.run([str(args.assets_dir.resolve() / "posternproxy"), "install", "--assets-dir",
                                 str(args.assets_dir.resolve()), "--skip-packages"],
                                capture_output=True, timeout=180)
        if result.returncode:
            raise RuntimeError("native controller installer failed; original data will be restored")
        return result.stdout.decode()

    def login(email, password):
        for attempt in range(120):
            try:
                with urllib.request.urlopen("http://127.0.0.1:81/api/version", timeout=2) as ready:
                    if ready.status == 200:
                        break
            except urllib.error.URLError:
                if attempt == 119:
                    raise RuntimeError("controller did not become ready after restart") from None
            time.sleep(0.25)
        request = urllib.request.Request("http://127.0.0.1:81/api/auth/login",
                                         data=json.dumps({"email": email, "password": password}).encode(),
                                         headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=10) as response:
            if response.status != 201 or not json.load(response).get("access_token"):
                raise RuntimeError("initial credentials cannot authenticate")

    systemctl("stop", "posternproxy")
    try:
        for index, path in enumerate(original):
            backup = work / f"original-{index}"
            path.rename(backup)
            moved.append((path, backup))
        output = install()
        email = re.search(r"^Email: (.+)$", output, re.MULTILINE)
        password = re.search(r"^Password: (.+)$", output, re.MULTILINE)
        if not email or not password:
            raise RuntimeError("fresh installer did not print initial credentials")
        login(email[1], password[1])
        if Path("/run/posternproxy-install/admin.env").exists():
            raise RuntimeError("initial password environment survived readiness")
        if password[1].encode() in Path("/var/log/posternproxy/posternproxy.log").read_bytes():
            raise RuntimeError("initial password was written to service logs")
        config = Path("/etc/posternproxy/config.env")
        before = hashlib.sha256(config.read_bytes()).digest()
        for path in [config, Path("/var/lib/posternproxy/posternproxy.db")]:
            if path.stat().st_mode & 0o777 != 0o600:
                raise RuntimeError("sensitive controller file permissions are incorrect")
        print("PASS fresh controller data, printed login, private files and transient password cleanup", flush=True)
        output = install()
        if "Password:" in output or "Existing admin credentials are unchanged." not in output:
            raise RuntimeError("installer rerun reset or printed existing credentials")
        if hashlib.sha256(config.read_bytes()).digest() != before:
            raise RuntimeError("installer rerun replaced controller configuration")
        systemctl("restart", "posternproxy")
        login(email[1], password[1])
        print("PASS installer rerun and controller restart preserve login and configuration", flush=True)
    finally:
        systemctl("stop", "posternproxy")
        for index, (path, backup) in enumerate(moved):
            if path.exists():
                path.rename(work / f"qualified-{index}")
            backup.rename(path)
        systemctl("restart", "caddy")
        systemctl("start", "posternproxy")
        print("Restored original controller; private qualification data: " + str(work), flush=True)


if __name__ == "__main__":
    main()
