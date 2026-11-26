# Installer initial password output

Status: implementation and local validation complete. User requested displaying the initial password from install.

Acceptance: a fresh controller install generates its bootstrap password in the
installer, supplies it to the first service start through a private /run env file,
and prints the email/password directly to the caller only after readiness.
Credentials do not enter service logs or persistent installed configuration.
The transient file is removed on success and failure. Existing databases and
passwords are preserved on reinstall; agent installation is unchanged. Rebuild
the Linux bundle, verify checksums, and update first-login documentation.


Evidence: acceptance tests first failed on the missing credential-output contract.
Tests now cover fresh generation, one-time terminal display, configured credentials,
no persistent-config/command-argument leakage, private runtime permissions,
existing-database preservation and cleanup after service failure. Full Go tests,
build and vet pass. Hugo builds 61 pages. Linux controller/agent were rebuilt,
archive contents/checksums verified from extraction, and binary vulnerability
scans report no findings. No fresh VM installation is claimed for this change.
