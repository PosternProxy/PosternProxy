# Claude.md

**First action for any non-trivial task:** Read `AGENTS.md` completely before proceeding.

## Claude-Specific Guidance
- Follow all rules in `AGENTS.md` (architecture, licensing, security, DRY, style).
- `.agentplans/` holds current execution directives and task breakdowns. Review relevant documents there when the work aligns with active efforts — do not reference or depend on any specific plan filename.
- When creating, updating, or working on plans, always save the resulting documents to `.agentplans/` (as required by AGENTS.md).
- Be conservative on security, auth, agent protocol, and licensing changes.
- Prefer editing existing files and extending current abstractions over introducing new layers.
- When generating code, include or update tests where logic changes.
- For refactors or new features, first outline the minimal diff and confirm it does not weaken the multi-server or commercial model.

## Communication & Output
- Keep responses concise and structured (bullets, short sections).
- After changes, note what was updated and whether `.agentplans/` documents or changelog need attention.
- If a change has commercial or licensing implications, explicitly call it out.

Use the general rules in `AGENTS.md` as the primary source of truth. This file only adds Claude-specific workflow notes on top.