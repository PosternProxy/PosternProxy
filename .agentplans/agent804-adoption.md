# Agent804 adoption for PosternProxy

Date: 2026-11-02. Status: proposed implementation sequence; no phase is complete.

## Scope and authority

The owner confirmed production infrastructure, script access to the management API,
and sequential repository editing. Target foundation 4 and rungs 5-8; Security and
Operate apply. Parallel work does not currently apply. Maintenance, Context and
Routing are reported trials, not additional scored gates.

This plan follows the Agent804 0.4.0 assessment in the current chat. It authorizes
no commits, pushes, deployments, migrations, host-policy changes or release decisions.
Preserve the existing dirty tree, outbound-only agents, embedded frontend, single
controller binary, Caddy data plane and dual-licensing model. Existing A-G plans
continue to own product fixes: [development index](beta-exit-development-index.md).

## 1. Establish a recoverable baseline

- [ ] Inventory tracked and untracked work, ownership and unfinished changes. Preserve
  a reconstructible source snapshot or owner-authorized revision; selected hashes
  alone are not sufficient. Do not bundle unrelated work into an adoption commit.
- [ ] Use a task branch before implementation, preserving the existing work. Keep
  writers sequential. If concurrency is introduced, qualify isolated checkouts,
  databases, ports, storage and check state first.
- [ ] Record the adoption boundary for future criteria/test provenance. Historical
  TDD, human approvals and review cannot be reconstructed from commit order alone.

Exit evidence: exact source identity plus retained patch/untracked contents, known
failures and resource requirements. Estimate: half a day. Rules: GR-3, WK-2, WK-8.

## 2. Reach foundation 2 with one verification entry point

- [ ] Implement one check runner callable on supported development hosts and Linux
  CI. Setup installs tools separately; checking must not silently install or update
  dependencies. Run independent clean CI jobs through that runner's declared targets
  and require all targets; a fast local subset must report its incomplete scope.
- [ ] Include non-mutating Go/frontend formatting and lint, types, tests/builds,
  instruction budgets, complexity/size/duplication checks, secret scanning, security
  static analysis and dependency scans. Pin tools and preserve scanner failures.
- [ ] Include browser, pinned-Caddy/two-agent acceptance, docs and supported-platform
  race checks in the full declared scope. Fail clearly on unavailable prerequisites;
  missing tools, services, feeds or targets must never produce a full-pass result.
- [ ] Print discovered/executed counts, skips, exclusions and target results; fail
  empty expected suites. Use disposable data. Document reserved ports and cleanup.
- [ ] Resolve existing format violations separately from behavior changes. Inventory
  maintainability findings, agree numerical limits and scoped reviewed debt; reject
  new/worsening violations. Security or test failures cannot be baselined away.
- [ ] Put the command first in AGENTS.md, keep shared policy within budget, point
  tool-specific instructions to it, and add fast feedback using the same configuration.

Exit evidence: intended-failure fixtures for controls, unchanged source after checks,
and a complete passing run on a capable host. The assessment's socket-denied Go
failures are environment-limited results, not proof of application defects. Do not
weaken network tests to make that environment green. Estimate: 2-4 days excluding
product/security remediation. Rules: IN-3, CK-1/2/6, ST-1, MT-2, SE-2, TS-10/12.

## 3. Qualify guardrails and independent verification

- [ ] Define person-only consent for sensitive Git, deployment, migration, policy
  and decision actions. Implement supported project controls without changing global
  trust or permissions as a side effect. Prevent agent merges to the protected branch.
- [ ] Configure and exercise a completion gate in each host actually used. A bounded
  failure-reporting escape may report unresolved failures, never claim success.
- [ ] Record client/version, policy sources, overrides/loading limits, effective
  permissions, hook trust/loading and passing/failing/timeout behavior in a host record.
  Distinguish configured, fixture-tested and actual-host evidence.
- [ ] Have the owner configure required CI and human review before merge; retrieve
  actual run/review evidence for the exact reviewed revision. Approval auto-review
  and agent-written records cannot establish human consent.
- [ ] Exercise fresh-clone startup and representative end-to-end behavior from a
  committed source state with synthetic data. Verify production-data separation
  without reading production secrets.

Exit evidence: refused unauthorized actions, exercised completion control, clean CI
and authenticated review/merge protection records. Estimate: 1-2 days plus owner
configuration. Rules: GR-1/2/4/7, SK-1/2, CK-8, RV-1, MT-1.

## 4. Use the framework for each remaining beta-exit change

- [ ] Agree change-sized criteria before implementation. Pass exact criteria and
  contracts to an independent acceptance-authoring context; this may be a sequential
  chat. Preserve intended red execution and exact revision/patch, then green evidence.
- [ ] Retain regression tests, inspect changed assertions and record uncertainty.
  Separate implementation and independent review inputs. No historical TDD claims.
- [ ] Map architecture invariants to focused checks: explicit route access,
  management/agent credential separation, outbound-only connection behavior, shared
  binary logic, permitted dependencies and validation on alternate write paths.
- [ ] Document public contracts and current examples; retire replaced configuration,
  privileges and docs alongside code. Keep compatibility exceptions scoped and owned.
- [ ] Create ADRs only for hard-to-reverse choices, with Proposed/Validated/Accepted/
  Rejected/Superseded states, append-only evidence and frozen accepted decision text.
  The owner accepts decisions; plans do not imply acceptance.

Exit evidence: one complete independently reviewed change with criteria, red/green,
preservation limits and exact source identity, then the same workflow for subsequent
changes. Estimate: about a day for workflow setup; execution follows A-G. Rules:
TS-1/2/3/4/8/9, AR-1/2/3/4/6, MT-1/3, DC-1/2/3, RV-2/3.

## 5. Close production Security and Operate gaps

- [ ] Add an owned threat model linked from instructions: assets, actors, script
  clients, trust boundaries, abuse cases and versioned requirements/exclusions.
- [ ] Continue A-G rather than duplicate their scope. Add dependency identity/build
  review, fresh scanner results, refusal tests, findings owners/deadlines and explicit
  person-approved scoped risk expiry. Verify deployed controls on authorized fixtures.
- [ ] Design a bounded script-facing API contract using shared services and access
  rules. Require explicit scoped write grants; default omitted token scopes to read.
- [ ] Complete mutation audit coverage, including actor, credential/channel identity,
  safe before/after where applicable, and visible audit-write failures.
- [ ] Specify durable machine-write request identity, input binding, concurrent/crash
  replay, expiry and unknown-outcome reconciliation before implementing deduplication.
- [ ] Verify endpoint-issued credentials and resource boundaries; define outgoing
  budgets/retries/caching where applicable. Grade model-specific Operate rules as
  inapplicable until an actual model-facing capability is introduced.

Exit evidence: refusal/retry tests, owned disposition of findings and actual A-G
qualification results. Harness adoption alone does not close beta-exit gates.
Estimate: several focused changes plus VM/container/release qualification. Rules:
SE-1/3/4/5/6, OP-1 through OP-11 where applicable.

## 6. Maintain the loop

- [ ] Keep canonical confirmed-debt records with consequence, owner, human disposition,
  review condition and verified closure; a debt entry never waives a failing gate.
- [ ] Record first-pass checks, corrections, rework, escaped defects/reverts, test
  reliability and maintainability trends. Leave missing data explicit.
- [ ] Establish a weekly reviewed gardening pass for stale instructions/examples,
  obsolete paths, advisories, unsupported dependencies, test discovery and expiring
  exceptions. Do not schedule an automation until the owner requests one.
- [ ] Reassess against fresh execution and review evidence. Move this plan to
  completed only when its applicable exits are verified, retaining open product plans.

Exit evidence: an actual measurement report and gardening dispositions. Estimate:
half a day to establish, followed by recurring review. Rules: MS-1/2/3, MT-4/5, IN-8,
WK-8; optional routing trials remain conditional.

## Current execution state

Only this plan has been added. No check runner, control, product change or score
increase is claimed. The first implementation slice is phases 1-2; phase 5's known
security blockers remain urgent throughout adoption. Estimates are planning ranges,
not a release-date commitment.
