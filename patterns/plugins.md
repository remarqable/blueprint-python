# Plugins

> **Layer doc.** Requires [tenancy.md](tenancy.md). If `tenancy: personal`, read
> [Single-Tenant Plugins](#single-tenant-plugins) at the bottom and skip the rest.

Tenants install and uninstall plugins that add routes, models, navigation, and
templates. From the tenant's point of view this happens at runtime, with no
restart. Under the hood it does not — and understanding why is the whole design.

---

## Table of Contents

- [The Constraint](#the-constraint)
- [Anatomy of a Plugin](#anatomy-of-a-plugin)
- [The Registry](#the-registry)
- [The Tenant Gate](#the-tenant-gate)
- [Install and Uninstall](#install-and-uninstall)
- [Navigation](#navigation)
- [Why Not pluggy](#why-not-pluggy)
- [Single-Tenant Plugins](#single-tenant-plugins)
- [Traps](#traps)

---

## The Constraint

**Flask cannot register routes after it has served a request.** This is enforced,
not incidental:

```
AssertionError: The setup method 'register_blueprint' can no longer be called on
the application. It has already handled its first request, any changes will not
be applied consistently.
```

`add_url_rule` raises the same. And even with the guard patched out you would
lose: with four Gunicorn workers, a tenant installing a plugin mutates *one*
worker's `url_map`. The other three return 404. You would ship a route that
works a quarter of the time.

So invert it:

> **Plugin code loads at boot, identically in every worker. "Installed for this
> tenant" is a row in a table, checked per request.**

The tenant-visible behavior is exactly what you want — install, and the routes
and nav appear; uninstall, and they are gone. Only the mechanism differs. Odoo,
Discourse, and WordPress multisite all work this way, for this reason.

---

## Anatomy of a Plugin

```
plugins/invoicing/
├── __manifest__.py              # metadata — no imports, no side effects
├── __init__.py                  # exports `plugin`
├── plugin.py                    # Plugin subclass + lifecycle hooks
├── models/invoice.py            # tables prefixed invoicing_
├── controllers/routes.py        # Blueprint('invoicing', ...)
├── views/templates/invoicing/   # namespaced to avoid collisions
└── lang/en.json                 # keys prefixed invoicing.
```

```python
# plugins/invoicing/__manifest__.py
manifest = {
    'slug': 'invoicing',              # unique; used for tables, routes, i18n, templates
    'name': 'Invoicing',
    'version': '1.0.0',
    'url_prefix': '/invoicing',       # namespaced: cannot collide with core routes
    'requires': [],                   # other plugin slugs, resolved at boot
    'nav': [
        {'label': 'invoicing.nav.invoices',
         'endpoint': 'invoicing.index',
         'permission': 'read'},
    ],
}
```

Keep the manifest importable without side effects — the loader reads it to build
the catalogue and resolve dependencies before importing any plugin code.

```python
# plugins/invoicing/models/invoice.py
from app.models.base import BaseModel, OrgScoped
from app.models.types import Money


class Invoice(OrgScoped, BaseModel):
    __tablename__ = 'invoicing_invoice'      # ALWAYS prefix with the slug

    number = db.Column(db.String(50), nullable=False)
    total = db.Column(Money, nullable=False, default=0)

    __table_args__ = (
        db.UniqueConstraint('org_id', 'number', name='uq_invoicing_invoice_org_number'),
    )
```

Plugin models inherit `OrgScoped`, so they get automatic tenant filtering exactly
like core models. The plugin author cannot forget it.

```python
# plugins/invoicing/controllers/routes.py
from flask import Blueprint
from flask_login import login_required
from app.platform.theming import render

bp = Blueprint('invoicing', __name__, template_folder='../views/templates')


@bp.route('/')
@login_required
def index():
    invoices = Invoice.query.order_by(Invoice.created_at.desc()).all()
    return render('invoicing/index.html', invoices=invoices)
```

No `org_id` filter in that query — the session-level filter from
[tenancy.md](tenancy.md#safe-by-default-scoping) applies it. Use `render()` from
[theming.md](theming.md) rather than `render_template` so tenants can restyle
plugin pages.

```python
# plugins/invoicing/plugin.py
from app.platform.plugins import Plugin
from .__manifest__ import manifest


class InvoicingPlugin(Plugin):
    manifest = manifest

    def blueprints(self):
        from .controllers.routes import bp
        return [(bp, manifest['url_prefix'])]

    def on_install(self, org_id: int) -> None:
        """Seed this tenant's data. Idempotent — may be retried."""
        if not InvoiceSequence.query.filter_by(org_id=org_id).first():
            InvoiceSequence(org_id=org_id, prefix='INV', next_number=1).save()

    def on_uninstall(self, org_id: int) -> None:
        """Disable. Do NOT delete tenant data here — see Traps."""
```

```python
# plugins/invoicing/__init__.py
from .plugin import InvoicingPlugin
plugin = InvoicingPlugin()
```

---

## The Registry

The entire loader is about a hundred lines. It runs once, inside `create_app`,
before the first request:

```python
# app/platform/plugins/registry.py
import importlib, pkgutil
from flask import g, abort

REGISTRY: dict[str, 'Plugin'] = {}


def discover() -> list[str]:
    """Slugs of every plugin directory, honouring __DISABLED__ marker files."""
    import plugins
    return sorted(
        m.name for m in pkgutil.iter_modules(plugins.__path__)
        if not m.name.startswith('_')
        and not (Path(plugins.__path__[0]) / m.name / '__DISABLED__').exists()
    )


def load_plugins(app):
    """Import every plugin and register its blueprints. Boot-time only."""
    for slug in _in_dependency_order(discover()):
        module = importlib.import_module(f'plugins.{slug}')
        plugin = module.plugin

        # Import models even if no tenant has installed the plugin: Alembic must
        # see every table, because migrations are global. See Traps.
        importlib.import_module(f'plugins.{slug}.models')

        for bp, prefix in plugin.blueprints():
            if bp.name != slug:
                raise RuntimeError(f'Blueprint name must equal slug: {bp.name} != {slug}')
            bp.before_request(_require_installed(slug))
            app.register_blueprint(bp, url_prefix=prefix)

        REGISTRY[slug] = plugin
        app.logger.info('plugin_loaded', extra={'slug': slug,
                                                'version': plugin.manifest['version']})
```

Requiring `bp.name == slug` gives you endpoint-collision detection for free:
Flask refuses to register two blueprints with the same name, so a duplicate slug
fails loudly at boot instead of silently shadowing routes.

`_in_dependency_order` topologically sorts by `requires` and raises on cycles or
missing dependencies — again at boot, where a crash is a good outcome.

---

## The Tenant Gate

```python
def _require_installed(slug: str):
    """before_request guard bound to one plugin's blueprint."""
    def guard():
        if slug not in installed_slugs():
            abort(404)
    return guard


def installed_slugs() -> set[str]:
    """Slugs installed for the current tenant, memoised per request."""
    if 'installed_plugins' not in g:
        org = getattr(g, 'org', None)
        g.installed_plugins = set() if org is None else {
            row.plugin_slug for row in
            OrgPlugin.query.filter_by(org_id=org.id, is_enabled=True)
        }
    return g.installed_plugins
```

`bp.before_request` fires only for requests routed into that blueprint, so the
gate costs one indexed lookup on plugin routes and nothing at all on core routes.
It is registered during `create_app`, so it is legal boot-time setup.

**Return 404, not 403.** A 403 tells an attacker which plugins exist and which
tenants have them. As far as an uninstalled tenant is concerned, the route does
not exist.

```python
# app/models/org_plugin.py
class OrgPlugin(BaseModel):
    """Which plugins a tenant has installed. NOT OrgScoped — see below."""
    __tablename__ = 'org_plugin'

    org_id = db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                       nullable=False, index=True)
    plugin_slug = db.Column(db.String(50), nullable=False)
    is_enabled = db.Column(db.Boolean, nullable=False, default=True)
    installed_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)

    __table_args__ = (
        db.UniqueConstraint('org_id', 'plugin_slug', name='uq_org_plugin'),
    )
```

`OrgPlugin` deliberately does not inherit `OrgScoped`: the automatic filter reads
`g.org`, and this table is consulted while establishing what the tenant may
access. Keep the filter and the thing that gates the filter separate.

---

## Install and Uninstall

```python
# app/platform/plugins/lifecycle.py
def install(org_id: int, slug: str) -> None:
    plugin = REGISTRY.get(slug)
    if plugin is None:
        raise NotFoundError(f'Unknown plugin: {slug}')

    for dep in plugin.manifest.get('requires', []):
        install(org_id, dep)                       # idempotent, so recursion is safe

    with transaction():
        existing = OrgPlugin.query.filter_by(org_id=org_id, plugin_slug=slug).first()
        if existing:
            existing.is_enabled = True             # re-install: just re-enable
        else:
            db.session.add(OrgPlugin(org_id=org_id, plugin_slug=slug))
            plugin.on_install(org_id)              # seed data in the same transaction

    g.pop('installed_plugins', None)               # invalidate the request cache


def uninstall(org_id: int, slug: str) -> None:
    dependents = [s for s, p in REGISTRY.items()
                  if slug in p.manifest.get('requires', [])
                  and s in _installed_for(org_id)]
    if dependents:
        raise ValidationError(f'Uninstall {", ".join(dependents)} first')

    with transaction():
        OrgPlugin.query.filter_by(org_id=org_id, plugin_slug=slug) \
                       .update({'is_enabled': False})
        REGISTRY[slug].on_uninstall(org_id)
    g.pop('installed_plugins', None)
```

Uninstall **disables**; it does not delete. Data survives so that reinstalling
restores the tenant's state. Purge on the tenant-deletion schedule from
[tenancy.md § Tenant Lifecycle](tenancy.md#tenant-lifecycle), never on uninstall.

---

## Navigation

The nav menu is derived from installed plugins, which is what makes the UI change
per tenant:

```python
@app.context_processor
def inject_nav():
    items = []
    for slug in installed_slugs():
        for entry in REGISTRY[slug].manifest.get('nav', []):
            if can(entry.get('permission', 'read')):
                items.append(entry)
    return {'plugin_nav': items}
```

```html
{% for item in plugin_nav %}
  <a href="{{ url_for(item.endpoint) }}">{{ _(item.label) }}</a>
{% endfor %}
```

Labels are i18n keys, not strings — see [core/i18n.md](core/i18n.md). Plugins ship
their own `lang/*.json`, merged at boot under their slug prefix.

---

## Why Not pluggy

pluggy solves one problem well: fanning a call out to many implementations with
validated hookspecs, ordering (`tryfirst`/`trylast`), and hook wrappers. That is
real value at pytest's scale. Measured against what this system actually needs:

| Need | pluggy provides |
|------|-----------------|
| Discover plugins on disk | nothing |
| Manifest, versions, dependency ordering | nothing |
| Register blueprints under a prefix | nothing |
| **Per-tenant install state** | nothing — `register`/`unregister` is process-global |
| **Per-request tenant gating** | nothing |
| Theme and template resolution | nothing |
| Fan `on_install` out to plugins | yes — but this is a `for` loop |

It addresses the easy fraction, and its `pm.hook.foo()` indirection is a layer
every contributor and every code-generating agent must learn — against this
blueprint's "no magic, explicit and local" philosophy.

**This is not a one-way door.** Because extension points are optional methods on
a `Plugin` base class, moving to pluggy later changes the registry, not the
plugins. Adopt it when you have genuine third-party authors *and* more than about
five extension points *and* you need ordering or wrapper semantics. Hand-rolling
those correctly is fiddly; hand-rolling a `for` loop is not.

---

## Single-Tenant Plugins

With `tenancy: personal` there is no per-tenant state, so there is no gate. The
whole system collapses to a config list read at boot:

```python
# config/local.env
PLUGINS=invoicing,reporting
```

```python
def load_plugins(app):
    enabled = set(app.config['PLUGINS'].split(','))
    for slug in _in_dependency_order(discover()):
        if slug not in enabled:
            continue
        module = importlib.import_module(f'plugins.{slug}')
        for bp, prefix in module.plugin.blueprints():
            app.register_blueprint(bp, url_prefix=prefix)   # no guard
        REGISTRY[slug] = module.plugin
```

Enabling a plugin requires a restart, which is entirely reasonable for a
single-tenant deployment. Do not build the gate you do not need.

---

## Traps

**Migrations are global; installs are not.** Every plugin's tables exist in the
schema always, whether or not any tenant installed it. `on_install` writes rows,
never DDL. Creating and dropping tables per install is where tenant plugin
systems go to die — it makes migrations non-deterministic and rollback
impossible.

**Plugins are trusted first-party code. This is not a sandbox.** Anyone who can
place a directory in `plugins/` has arbitrary code execution as the app user.
If you ever want *tenants* to upload plugins, that is a fundamentally different
problem requiring process isolation at minimum. Say so in your own docs so no
one assumes otherwise.

**The gate checks installation, not ownership.** `_require_installed` answers
"is this plugin on for your tenant" — it says nothing about whether a row belongs
to you. That is `OrgScoped`'s job, which is why plugin models must inherit it.

**Cache invalidation across workers.** `installed_slugs()` is memoised per
request, so a change on worker 1 is picked up by worker 2 on its next request.
Do not promote this to a process-level cache without a version counter or TTL —
you will get tenants seeing stale menus with no way to force a refresh.

**Namespace everything by slug**: `invoicing_` tables, `invoicing` blueprint,
`invoicing/` templates, `invoicing.` i18n keys, `/invoicing` URL prefix. Enforce
it at boot; a collision found at import time is free, one found in production is
not.

---

**Next:** [Theming](theming.md) | [Tenancy](tenancy.md)
