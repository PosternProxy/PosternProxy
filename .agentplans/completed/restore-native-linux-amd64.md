# Restore native Linux amd64 release support

Status: complete, 2026-11-25. User clarified the added platforms supplement
existing native Linux amd64 support; they do not replace it. Correct the prior
interpretation without changing existing qualification evidence or licensing.

Release matrix: native Linux amd64 and arm64, Windows amd64 with service
installer, Docker controller/agent images for amd64 and arm64. Use one native
Linux CI matrix to build, qualify and package both architectures and include
both in draft release checksums. Correct install/build guides, README and active
release plans; annotate completed plans where their scope interpretation was
wrong. Validate workflow syntax, documentation and artifact naming. Native amd64
runtime qualification already passed on the supplied Ubuntu VM.

Implemented: one native Linux CI matrix uses Ubuntu 24.04 amd64 and Ubuntu 24.04
arm64 runners. Both run tests/race/vet/vulnerability scanning, build matching
controller/agent/Caddy, exercise native installation and traffic, and emit their
own loose assets and tar archives. Draft publication depends on the full Linux
matrix, Windows and Docker jobs; the combined manifest covers both Linux targets.

Corrected README, native installation/build guides, getting-started index,
Windows provisioning notes, release validation/limitations, changelog and active
scope plans. Completed plans explicitly identify the earlier mistaken omission.
No installer runtime code needed restoration: it continued to support amd64,
and native Ubuntu amd64 installation/provisioning/traffic tests had passed.

Validation: actionlint passes, Hugo builds all 63 pages, and git diff whitespace
checks pass. The corrected tagged workflow has not been run or published during
this correction; no commit/tag/push was made.
