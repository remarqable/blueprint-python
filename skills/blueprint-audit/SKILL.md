---
name: blueprint-audit
description: Audit code against the Flask SaaS blueprint. Launches an independent adversarial reviewer agent over the changed files, then reports violations and a scored conformance table across MVC, database, portability, security, typing, frontend, HTMX, auth, i18n, and the enabled layers. Respects the blueprint's scoped-reading rule. This is the mandatory verification gate — run it after every implementation, before reporting the work complete.
---

# Blueprint Audit

> **Scope: any project built on `remarqable/blueprint-python`.** Not sparQ —
> sparQ vendors a diverged blueprint with a different layout and sparQ-only
> patterns. Use `/sk-sparq-blueprint-audit` there.

Check changed code against the blueprint and report what violates it. The review
is run by a **separate agent** with an adversarial brief: its job is to find
violations, not to confirm the code is fine.

## This is a gate, not an option

The blueprint's agent instructions make this audit mandatory. Any agent that
generated or modified code under a blueprint project runs this skill before it
reports the work finished. A feature is not done when it runs — it is done when
it runs *and* conforms.

Three things follow from that, and they are the whole point:

- **Do not skip it because the change is small.** The violations this catches —
  a missing `org_id`, a `datetime.utcnow()`, an unbuilt stylesheet — are
  overwhelmingly small changes. Size is not evidence of safety.
- **Do not skip it because you are confident.** Confidence is the symptom being
  controlled for. See below.
- **Do not report the audit's result as your own judgment.** Print what the
  reviewer returned.

## Why a separate agent

The agent that wrote the code knows why every shortcut was taken and will
rationalize each one. A reviewer with no memory of the implementation, and no
access to the goal or plan, cannot. That independence is the mechanism — do not
skip it, and do not do the review yourself in the implementing context.

## The scoped-reading rule

This blueprint is **not** a menu of alternatives to blend. It describes one
architecture with optional layers, and reading a layer that does not apply to the
project produces code that does not run. The blueprint states this as its first
rule, and the reviewer must obey it too:

- `patterns/core/` applies to every project. Always in scope.
- A layer doc (`tenancy.md`, `plugins.md`, `theming.md`, `storage.md`,
  `jobs.md`) is in scope **only** if that layer is enabled.
- Two decisions select a section *within* a core doc rather than a layer doc:
  `auth` picks one strategy inside `patterns/core/auth.md`, and `deploy` picks
  one shape inside `patterns/core/deployment.md`. Review against the chosen
  branch only.

A reviewer that reads a disabled layer will report violations for patterns the
project deliberately does not implement. That is worse than no review, because
the findings look authoritative.

## When to Use

- **After every implementation, before reporting the work complete** — mandatory
- Before committing any change to a blueprint project
- To review a contributor's pull request against the blueprint
- Any time you want a second opinion independent of the implementation context

## Workflow

### Step 1: Identify Changes

```bash
# Uncommitted changes (default)
git diff HEAD --name-only

# Nothing uncommitted? Review the last commit
git diff HEAD~1 --name-only

# Reviewing a PR branch
git diff main...HEAD --name-only
```

If nothing has changed, say so and stop.

When this skill runs as the post-implementation gate, the changed set is the work
you just did. Prefer the explicit file list you know you touched over a `git
diff` that may also sweep in unrelated working-tree edits.

| File pattern | Brings in |
|---|---|
| `app/models/*.py` | MVC, database, typing, tenancy |
| `app/controllers/*.py` | MVC, auth, security, typing |
| `app/views/**/*.html` | Frontend, HTMX, i18n, CSRF |
| `app/platform/**` | Whichever layer that subsystem implements |
| `app/static/css/**` | Frontend — and check `app.css` was rebuilt |
| `plugins/**` | Plugins layer |
| `app/views/themes/**` | Theming layer |
| `migrations/**` | Database |
| `tests/**` | Testing |
| `app/lang/*.json` | i18n |

### Step 2: Resolve the Blueprint and Establish the Active Layers

The blueprint is a git submodule. Its path is per-project — find it rather than
assuming:

```bash
git config -f .gitmodules --get-regexp '\.path$'
```

It is `blueprint/` in most projects and `blueprint/python/` where a repo carries
more than one language blueprint. Everything below is relative to that path:

- Master doc: `<blueprint>/CLAUDE.md`
- Always-applicable patterns: `<blueprint>/patterns/core/` — `audit`, `auth`,
  `database`, `deployment`, `frontend`, `htmx`, `i18n`, `mobile-navigation`,
  `mvc`, `portability`, `security`, `testing`, `typing`
- Layer docs: `<blueprint>/patterns/` — `jobs.md`, `plugins.md`, `storage.md`,
  `theming.md`, `tenancy.md`

If the directory is empty, the submodule is not initialized. Stop and tell the
user to run `git submodule update --init --recursive` — do not review against
patterns you could not read.

**Determining which layers are active.** `<blueprint>/CLAUDE.md` carries a
Project Configuration block, but the blueprint is a shared submodule, so that
block usually holds **upstream defaults, not this project's answers**. Never take
it at face value. Establish the real configuration in this order:

1. If the project records its own answers (a root `CLAUDE.md`, `AI.md`, an ADR
   under `docs/adr/`), that record wins.
2. Otherwise infer from evidence in the repo, and print what you inferred so the
   user can correct it before the review runs.

Evidence to look for:

| Setting | Evidence |
|---|---|
| `tenancy: shared` | An organization or tenant model, membership model, an `OrgScoped` base or `org_id` columns |
| `plugins: true` | A `plugins/` tree, a per-tenant plugin model |
| `theming: true` | A themes directory under views, theme install or activation code |
| `uploads: true` | An upload model, a media or uploads directory under the data dir |
| `jobs: true` | A job model, a worker entrypoint such as `flask jobs run` |
| `audit_logging: true` | An audit log model or table |
| `auth` | `password` if the user model hashes and verifies passwords; `magic_links` if it issues single-use login tokens |
| `database` | `DATABASE_URL` handling in config; note the default |
| `deploy` | `docker` if there is a Dockerfile and compose file; `vps` if there is a systemd or Caddy provisioning script; both can be true |

### Step 3: Check for a Linter

If the blueprint ships a mechanical linter, run it first so the reviewer starts
from facts:

```bash
ls <blueprint>/scripts/ 2>/dev/null
```

There is no linter today. Note that in the report and continue with the manual
review.

### Step 4: Launch the Reviewer Agent

Use the Agent tool. Give it the changed files, the resolved blueprint path, and —
critically — the active layer list, so it knows which docs it is forbidden to
open.

```
You are a blueprint compliance reviewer. Your role is ADVERSARIAL — you are
looking for violations, not confirming correctness.

You did not write this code. You did not plan this feature. You have no
investment in it passing review.

## Scope

Review ONLY these changed files: <changed file paths>
Blueprint: <resolved blueprint path> (master doc <blueprint>/CLAUDE.md)

Active project configuration:
<the resolved yaml block from Step 2>

## Reading rules — these are hard constraints

1. Read all of <blueprint>/patterns/core/ that the changed files pull in.
2. Read a layer doc ONLY if its layer is enabled above:
     tenancy.md  -> only if tenancy: shared
     plugins.md  -> only if plugins: true
     theming.md  -> only if theming: true
     storage.md  -> only if uploads: true
     jobs.md     -> only if jobs: true
   If a layer is disabled, do not open its doc at all — not for reference,
   not for context.
3. For auth and deployment, read only the branch the configuration selects
   inside patterns/core/auth.md and patterns/core/deployment.md. Do not
   review against the branch this project did not choose.

Which core docs the changes pull in:
   Models      -> core/mvc.md, core/database.md, core/typing.md
   Controllers -> core/mvc.md, core/auth.md, core/security.md
   Templates   -> core/frontend.md, core/htmx.md, core/i18n.md
   Migrations  -> core/database.md
   Tests       -> core/testing.md
   Anything that touches paths, config, or the data dir -> core/portability.md

## Rules

Do NOT:
- Assume the implementer "probably had a good reason"
- Skip a violation because it is minor
- Praise the code
- Suggest improvements the blueprint does not require
- Read GOAL.md, PLAN.md, or any spec — that context is what produces
  rationalization
- Audit files outside the changed list
- Open a disabled layer's doc

DO:
- Check every changed Python file for MVC violations: business logic in
  controllers, the session touched from the request layer, validation in the
  wrong place
- Check database changes against core/database.md: model registration, table
  and column naming, CRUD as classmethods, Mapped[T] columns, migrations
  present for schema changes
- Check the blueprint's non-negotiables explicitly: BigIntPK on primary keys,
  utcnow() rather than datetime.utcnow(), named constraints, SQLite pragmas,
  no migrations inside create_app
- If tenancy: shared, check that every new business table carries its tenant
  column and that queries are scoped — an unscoped query is a cross-tenant
  data leak, which is Critical, not Medium
- Check every changed template for CSRF tokens on non-GET forms, i18n
  wrapping of user-facing strings, and correct HTMX attributes
- Check that any template change is reflected in a rebuilt app/static/css/app.css
- Check templates use logical properties (ms-/me-/ps-/pe-/text-start), never
  ml-/mr-/text-left, and that interactive components carry their own ARIA
- Check type hints against core/typing.md
- Check auth decorators against core/auth.md
- Check for SQL injection and hardcoded secrets
- Check core/portability.md for hardcoded paths, assumed database engine, or
  anything that breaks a self-hosted installation

## Report Format

Respond with ONLY this:

**Blueprint:** <path> @ <submodule short sha>
**Active layers:** <the layers you were permitted to read>
**Files reviewed:** <count>
**Patterns checked:** <list>
**Result:** FAIL | WARN | PASS

### Violations
| # | Severity | Pattern | File:Line | Description |
|---|----------|---------|-----------|-------------|

If none: "No violations found."

### Conformance
| Pattern | Score | Status | Notes |
|---------|-------|--------|-------|
(one row per pattern category you checked; N/A for categories the
changed files never touched)

**Overall: X.X/10**

### Severity Guide
- Critical: exploitable security issue, cross-tenant data exposure, or
  fundamental architecture violation
- High: significant pattern violation
- Medium: best-practice violation
- Low: minor convention issue
```

### Step 5: Score

The reviewer fills the conformance table using this scale, per pattern category:

| Violations in that category | Score |
|---|---|
| none | 10/10 |
| 1 minor | 9/10 |
| 2-3 minor | 8/10 |
| 1 major, or 4+ minor | 7/10 |
| 2 major | 6/10 |
| 3+ major | 5/10 or lower |

**Major** — business logic in a controller, missing CSRF token, SQL injection,
missing auth decorator on a protected route, an unscoped query on a
tenant-scoped table, a schema change with no migration, a violated
non-negotiable.

**Minor** — missing type hints, an unwrapped user-facing string, missing
docstring, naming drift.

Score `N/A` for any category the changed files never touched. Do not average N/A
rows into the overall.

Strictness by category:

- **Strict** — MVC separation, database patterns, tenancy scoping, security
  (CSRF, injection, auth), portability. These are correctness and safety, and on
  a self-hostable product portability is a user-facing promise.
- **Moderate** — typing, HTMX usage. Flag them; do not fail the review over one
  missing annotation.
- **Lenient** — i18n on admin-only text, docstrings on small helpers, test
  coverage where critical paths are already covered.

### Step 6: Present Findings

1. Print the violations table and the conformance table in full.
2. State the result:
   - **FAIL** — "Critical or High violations. Fix before committing."
   - **WARN** — "Minor violations. Worth fixing, safe to proceed."
   - **PASS** — "No violations."
3. For each Critical and High violation, suggest the concrete fix with the
   pattern section it comes from.
4. If the active layer set had to be inferred rather than read from a project
   record, say so, and suggest recording it — in a root `CLAUDE.md` or an ADR —
   so the next review does not have to guess.

### Step 7: Act on the Result

When this ran as the post-implementation gate:

- **FAIL** — fix the Critical and High violations, then re-run the audit on the
  corrected files. Do not report the work complete on a FAIL, and do not commit.
  Fixing in place and re-auditing is the expected loop, not an escalation.
- **WARN** — report the work complete with the violations table included, and say
  plainly which ones you left. Do not silently drop them.
- **PASS** — report the work complete and include the conformance table.

If a violation is one you intend not to fix, say so and give the reason. An
unfixed finding that the user can see is a decision; one you quietly discard is a
regression.

## Rules

- **Never put GOAL.md, PLAN.md, or the spec in the reviewer's context.** This is
  the single most important rule here.
- **Never let the reviewer read a disabled layer.** This is the second.
- This skill reviews and reports. It does not edit code. Fixes happen back in the
  implementing context, after the report.
- Run it standalone, not nested inside another skill's workflow.
- Findings should be specific enough to act on without further explanation — on
  an open-source project the review lands in a PR read by people outside the team.
- If a violation reveals a genuine gap in the blueprint rather than a mistake in
  the code, say so. The fix belongs in `remarqable/blueprint-python`, where every
  project picks it up — never edited in place inside the submodule checkout.
