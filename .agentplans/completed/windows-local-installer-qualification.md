# Windows local installer qualification

Status: complete, 2026-11-25. User explicitly authorized installing and
uninstalling services on this computer. No existing PosternProxy services, data
or managed binaries were found. The current process is not elevated.

Build matching current controller and agent, reuse the verified pinned Windows
Caddy, and generate checksums. Execute the Windows lifecycle harness with an
explicit local-machine mode, management listeners on loopback, and Windows UAC
elevation. Check fresh login, reinstall/secret preservation, service restart,
controller uninstall, native agent enrollment/TLS/restart and agent uninstall.
Record failures, fix and retest within this scope. Preserve test-created data
after uninstall as the intended behavior; do not change firewall rules or touch
unrelated services. Record final service and process state.

Live failures reproduced and corrected:
- SDDL does not accept the `icacls` RX abbreviation. Translate read/execute to
  FRFX; elevated actual-directory ACL tests reproduced the failure and now pass.
- Read active service logs with FileShare.ReadWrite/Delete in the harness.
- The generated Caddy admin listener was localhost while native/container
  bootstrap uses 127.0.0.1. Windows rejected reloads with a bind conflict; use the
  bootstrap address consistently and guard it with a translator regression.

Controller fresh install/login/version, virtual identities, bootstrap cleanup,
stable encryption key/credentials on reinstall, restart and data-preserving
uninstall passed. Agent installation, encrypted enrollment, authenticated outbound
connectivity, acknowledged applied revision, verified custom TLS through real
Caddy, restart and data-preserving removal passed after the admin listener fix.
The harness readiness assertion also needed explicit enumeration of REST arrays.
Failed attempts are archived under ignored `.failed-attempt-*`
directories with their protected ACLs retained. No existing installation existed.

Final check: no posternproxy/posternproxy-agent/posternproxy-caddy services, no
controller/agent/Caddy processes, and no TCP listeners on 80/443/2019/8443/18181/
18182. Program Files/PosternProxy and ProgramData/PosternProxy remain, as intended.
No firewall changes were made. Full Go tests/build/vet pass after the fixes;
elevated fresh-root/existing-tree refusal and FA/RX/TRAVERSE filesystem tests pass.

Artifacts: current source built as v0.0.0-windows-local-test, pinned Caddy 2.11.9
with L4 0.1.1/rate-limit 0.1.0 and Go 1.26.11. Local checksums and non-secret
PASS log/result are under `tmp/windows-local-qualification/`. This is development
bundle evidence, not a candidate CI/public download or full OS support matrix.
