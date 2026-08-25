# Theming

> **Layer doc.** Pairs with [tenancy.md](tenancy.md). With `tenancy: personal`,
> a theme is one config value and `render()` collapses to `render_template` —
> read [Single-Tenant Theming](#single-tenant-theming) and skip the rest.

Each tenant can change how the app looks: colours and typography at the cheap
end, overridden templates at the expensive end. Unlike routes, this genuinely
works at runtime — Jinja resolves templates per render, not at boot.

---

## Table of Contents

- [Two Levels of Theming](#two-levels-of-theming)
- [Branding Without Templates](#branding-without-templates)
- [Template Overrides](#template-overrides)
- [Theme Assets](#theme-assets)
- [Theme Packaging](#theme-packaging)
- [Security](#security)
- [Single-Tenant Theming](#single-tenant-theming)

---

## Two Levels of Theming

Offer both; most tenants only ever want the first.

| Level | Mechanism | Who | Risk |
|-------|-----------|-----|------|
| **Branding** | Tailwind theme variables + logo, per tenant | any tenant, self-serve | none |
| **Template override** | files on disk shadowing base templates | you, or vetted partners | executes code — see [Security](#security) |

---

## Branding Without Templates

Ninety percent of "can we theme it" requests are a logo and a brand colour. That
needs no template machinery at all — just per-tenant values injected as CSS
custom properties:

```python
class Organization(BaseModel):
    theme = db.Column(db.String(50), nullable=False, default='default')
    brand_primary = db.Column(db.String(7), nullable=True)      # #RRGGBB
    logo_url = db.Column(db.String(500), nullable=True)
```

```html
{# app/views/layouts/base.html #}
{% if g.org and g.org.brand_primary %}
<style>
  :root {
    --color-brand-500: {{ g.org.brand_primary }};
    --color-brand-600: {{ g.org.brand_primary }};
    --color-brand-700: {{ g.org.brand_primary }};
  }
</style>
{% endif %}
```

This works because Tailwind v4 compiles every theme utility to a CSS variable:
`bg-brand-600` is `background-color: var(--color-brand-600)`. Overriding the
variable at `:root` re-skins every button, link, and active state on the page
without recompiling anything — the tenant's colour is data, not a build.

For a full brand scale rather than one flat colour, store the base hue and derive
the ramp in `oklch()`, which keeps perceived lightness consistent across steps:

```html
<style>
  :root {
    --color-brand-500: oklch(0.59 0.20 {{ g.org.brand_hue }});
    --color-brand-600: oklch(0.52 0.20 {{ g.org.brand_hue }});
    --color-brand-700: oklch(0.45 0.18 {{ g.org.brand_hue }});
  }
</style>
```

Validate `brand_hue` as a number in `0..360`, exactly as you validate the hex
form below.

**Validate the colour on write** with a strict `#RRGGBB` pattern. Interpolating
unvalidated tenant input inside a `<style>` block is a CSS injection, and Jinja's
HTML autoescaping does not protect you inside `<style>`:

```python
def validate(self):
    if self.brand_primary and not re.fullmatch(r'#[0-9a-fA-F]{6}', self.brand_primary):
        raise ValidationError('Brand colour must be #RRGGBB')
```

---

## Template Overrides

Themes live on disk and shadow base templates by name:

```
app/views/                        # base templates (the fallback)
├── layouts/base.html
├── users/profile.html
└── themes/
    ├── midnight/
    │   ├── theme.json
    │   ├── layouts/base.html     # overrides the base layout
    │   └── static/theme.css
    └── compact/
        └── users/profile.html    # overrides one page, inherits everything else
```

Resolution is a candidate list — Flask's `render_template` accepts a list and
uses the first template that exists:

```python
# app/platform/theming.py
from flask import g, render_template


def render(template: str, **context) -> str:
    """Render with the current tenant's theme, falling back to base.

    Use this everywhere instead of render_template.
    """
    theme = getattr(g, 'org', None) and g.org.theme or 'default'
    if theme == 'default':
        return render_template(template, **context)
    return render_template([f'themes/{theme}/{template}', template], **context)
```

Verified behavior:

```
tenant=midnight  profile.html   -> MIDNIGHT profile: Ada      (override wins)
tenant=default   profile.html   -> BASE profile: Ada          (no theme)
tenant=midnight  settings.html  -> BASE settings              (partial theme, no crash)
jinja cache keys -> ['themes/midnight/users/profile.html', 'users/profile.html', ...]
```

Two properties fall out of this, both of which matter:

**Themes can be partial.** Override one page and inherit the rest. A theme is not
required to be a complete copy of your template tree, so it does not rot when you
add pages.

**Caching stays correct.** That last line is the reason to use a candidate list
rather than swapping `app.jinja_loader` per request. Jinja caches compiled
templates by name; because `themes/midnight/users/profile.html` and
`users/profile.html` are distinct names, they get distinct cache entries. A
per-request loader swap keeps the *same* name for different content, and under
concurrency serves one tenant's theme to another. That bug is intermittent,
load-dependent, and extremely hard to reproduce — avoid it structurally.

Themes override plugin templates for free, since plugin templates are named
`invoicing/index.html` and go through the same candidate list.

### Theme inheritance

A theme extending the base layout rather than replacing it:

```html
{# app/views/themes/midnight/layouts/base.html #}
{% extends 'layouts/base.html' %}     {# the BASE one — different name, no recursion #}

{% block head %}
  {{ super() }}
  <link rel="stylesheet" href="{{ theme_asset('theme.css') }}">
{% endblock %}
```

This is why theme templates are prefixed rather than shadowing identical names:
`themes/midnight/layouts/base.html` can extend `layouts/base.html` without
infinite recursion.

---

## Theme Assets

Serve theme static files through a single route so unknown themes 404 rather
than traversing your filesystem:

```python
@bp.route('/themes/<theme>/static/<path:filename>')
def theme_static(theme: str, filename: str):
    if theme not in AVAILABLE_THEMES:          # whitelist, never trust the URL
        abort(404)
    return send_from_directory(THEMES_DIR / theme / 'static', filename)


@app.template_global()
def theme_asset(filename: str) -> str:
    theme = getattr(g, 'org', None) and g.org.theme or 'default'
    return url_for('main.theme_static', theme=theme, filename=filename,
                   v=AVAILABLE_THEMES[theme]['version'])   # cache-bust on release
```

`send_from_directory` rejects `..` traversal, but the whitelist check is what
stops an attacker enumerating your theme directory in the first place.

---

## Theme CSS and the Build

A theme's own `theme.css` is a plain stylesheet, not a second Tailwind build. It
should contain variable overrides and at most a handful of rules:

```css
/* app/views/themes/midnight/static/theme.css */
:root {
  --color-brand-500: oklch(0.55 0.19 265);
  --radius-card: 0.25rem;
}
```

If a theme needs utility classes that no base template uses, Tailwind will not
have compiled them — the build scans your templates, and an unused class does not
exist in the output. Two ways out, in order of preference:

1. Keep theme overrides to variables and existing utilities. Covers almost
   everything and needs no build change.
2. Add the theme directory as a source so its classes compile into the shared
   stylesheet: `@source "../../views/themes";` in `input.css`.

Do not give each theme its own Tailwind build. You would ship one stylesheet per
theme, and every tenant would download utilities they already have.

---

## Theme Packaging

```json
{
  "slug": "midnight",
  "name": "Midnight",
  "version": "1.2.0",
  "author": "Acme Design",
  "extends": "default",
  "overrides": ["layouts/base.html", "partials/_navbar.html"]
}
```

Scan `app/views/themes/*/theme.json` at boot into `AVAILABLE_THEMES`. Keep
`overrides` accurate and assert it at boot in development — a theme that claims
an override it does not ship, or ships one it does not declare, is the source of
"why does this tenant look wrong" tickets.

Add a smoke test that renders every page under every installed theme. Themes
break silently: nothing errors until a tenant visits the one page whose override
references a variable you renamed.

---

## Security

**A Jinja template is code.** `{{ config }}`, `{{ self.__init__.__globals__ }}`,
and friends reach the application object, your `SECRET_KEY`, and the database
session. Consequences:

- **Themes are first-party or vetted-partner code, exactly like plugins.**
  Shipping a theme is a deploy, not a tenant self-service action.
- **Never let tenants upload template files.** A theme upload form is a remote
  code execution endpoint with extra steps.

If you do need tenant-authored layout, do not relax this — narrow the surface
instead. Give them a constrained block of content rendered through Jinja's
`SandboxedEnvironment` with an explicit, tiny variable whitelist, and keep it far
away from the loader that serves real templates. Sandbox escapes are a live
research area; treat it as defence in depth, not a guarantee.

For anything less than a full layout, prefer the
[branding](#branding-without-templates) approach. Validated colours and an
uploaded logo cover the real demand with none of the risk.

---

## Single-Tenant Theming

One value in config, resolved at boot:

```python
# config/local.env
THEME=midnight
```

```python
def render(template: str, **context) -> str:
    theme = current_app.config['THEME']
    if theme == 'default':
        return render_template(template, **context)
    return render_template([f'themes/{theme}/{template}', template], **context)
```

Identical call sites, so a single-tenant app that later becomes multi-tenant only
changes this function.

---

**Next:** [Plugins](plugins.md) | [Tenancy](tenancy.md) | [Frontend](core/frontend.md)
