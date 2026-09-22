# Skills

Executable procedures that go with the patterns. A pattern doc says what correct
code looks like; a skill is a procedure an agent runs.

| Skill | Purpose | When |
|-------|---------|------|
| [blueprint-audit](blueprint-audit/SKILL.md) | Independent adversarial review of changed code against the blueprint, with a scored conformance table | **Mandatory** after every implementation, before reporting the work complete |

## Using them

Skills work two ways, and the first needs no setup.

**By path (always available).** Every skill is a Markdown file in this
submodule. An agent reads `<blueprint>/skills/<name>/SKILL.md` and follows it.
The blueprint's [AI Agent Instructions](../CLAUDE.md#-ai-agent-instructions)
require exactly this for `blueprint-audit`, so the gate holds in a fresh clone
with nothing installed.

**As a slash command (opt in).** From the project root:

```bash
make skills
```

That symlinks each skill into `.claude/skills/`, making it invocable as
`/blueprint-audit`. Symlinks, not copies — the skill tracks the submodule, so a
`git submodule update` picks up changes with no reinstall. Add `.claude/skills/`
to the project `.gitignore`; the symlinks are local and point at a path that
varies per checkout.

If your project has no `make skills` target yet, copy it from the blueprint's
[Makefile](../Makefile).

## The audit is not optional

The blueprint's whole value is that generated code conforms to it. Prose alone
does not achieve that — an agent reads the patterns, implements, and drifts,
with nothing checking the gap between the two. The audit is that check, and it
is run by a **separate agent** that never saw the plan, because the agent that
wrote the code can justify every shortcut it took.

Skip the audit and the blueprint degrades into style suggestions.

## Adding a skill

One directory per skill, holding a `SKILL.md` with YAML frontmatter:

```markdown
---
name: kebab-case-name
description: One paragraph. Says what it does, what it reports, and when to run it — this is what an agent matches against, so be concrete.
---
```

Keep skills project-agnostic. This submodule is shared across every project on
the blueprint, so resolve the blueprint path rather than hardcoding it, and read
the project's own configuration rather than assuming a layer is enabled.
