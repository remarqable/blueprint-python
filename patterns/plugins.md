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
- [Version Pinning](#version-pinning)
- [Versions and the Database](#versions-and-the-database)
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
├── __manifest__.py              # catalogue metadata — no imports, no side effects
├── v1/
│   ├── __init__.py              # exports `plugin`
│   ├── plugin.py                # Plugin subclass + lifecycle hooks
│   ├── models/invoice.py        # tables prefixed invoicing_v1_
│   ├── controllers/routes.py    # Blueprint('invoicing_v1', ...)
│   ├── views/templates/invoicing/
│   └── lang/en.json
└── v2/
    └── …                        # same shape, tables prefixed invoicing_v2_
```

**One directory per major version.** Tenants pin to a major; minor releases
(1.0 → 1.1) replace the contents of `v1/` in place and must stay backward
compatible. See [Versions and the Database](#versions-and-the-database) for why
the line falls at major versions.

```python
# plugins/invoicing/__manifest__.py
manifest = {
    'slug': 'invoicing',              # unique; used for tables, routes, i18n, templates
    'name': 'Invoicing',
    'versions': ['1', '2'],           # major versions present on disk
    'default_version': '2',           # what a fresh install pins to
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
# plugins/invoicing/v1/models/invoice.py
from app.models.base import BaseModel, OrgScoped
from app.models.types import Money


class Invoice(OrgScoped, BaseModel):
    __tablename__ = 'invoicing_v1_invoice'   # prefix with slug AND major version

    number = db.Column(db.String(50), nullable=False)
    total = db.Column(Money, nullable=False, default=0)

    __table_args__ = (
        db.UniqueConstraint('org_id', 'number', name='uq_invoicing_v1_invoice_org_number'),
    )
```

Plugin models inherit `OrgScoped`, so they get automatic tenant filtering exactly
like core models. The plugin author cannot forget it.

```python
# plugins/invoicing/v1/controllers/routes.py
from flask import Blueprint
from flask_login import login_required
from app.platform.theming import render

bp = Blueprint('invoicing_v1', __name__, template_folder='../views/templates')


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
# plugins/invoicing/v1/plugin.py
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

    def on_upgrade_from(self, org_id: int, previous_major: str) -> None:
        """Move this tenant's data from the previous major's tables.
        Only defined on the NEWER version. See Versions and the Database."""
```

```python
# plugins/invoicing/v1/__init__.py
from .plugin import InvoicingPlugin
plugin = InvoicingPlugin()
```

---

## Version Pinning

Two tenants can run different major versions of the same plugin on the same
public URLs: tenant A on calculator 1.x and tenant B on calculator 2.x, both
using `/calculator/`.

### Why the obvious approach fails silently

Registering both versions on the same prefix looks like it works:

```python
app.register_blueprint(calculator_v1, url_prefix='/calculator')
app.register_blueprint(calculator_v2, url_prefix='/calculator')
```

Werkzeug accepts it without complaint, and then:

```
rules matching /calculator/:
    /calculator/ -> endpoint: calculator_v1.index
    /calculator/ -> endpoint: calculator_v2.index
GET /calculator/ returns: CALC v1.0
```

Both rules land in the map and the first one registered wins **for every
request**. Tenant B pins to 2.0 and silently gets 1.0 — no exception, no warning,
no log line. Never register two versions on one prefix.

### Private mounts plus a public dispatcher

Mount each version on a private, version-qualified prefix, then expose one public
route per plugin that dispatches to the tenant's pinned version:

```python
# app/platform/plugins/registry.py

# Private: /_v/<slug>/<major>/...  — never linked, never bookmarked
for major in manifest['versions']:
    module = importlib.import_module(f'plugins.{slug}.v{major}')
    app.register_blueprint(module.plugin.blueprint(),
                           url_prefix=f'/_v/{slug}/{major}')

# Public: one route, all methods, everything below the prefix
public = Blueprint(slug, __name__)

@public.route('/', defaults={'rest': ''}, methods=HTTP_METHODS)
@public.route('/<path:rest>', methods=HTTP_METHODS)
def dispatch(rest):
    g.plugin_slug = slug
    g.plugin_version = installed_version(slug)          # from org_plugin
    if g.plugin_version is None:
        abort(404)                                       # not installed for this tenant

    adapter = current_app.url_map.bind(request.host)
    endpoint, args = adapter.match(
        f'/_v/{slug}/{g.plugin_version}/{rest}', method=request.method)

    # A direct view call skips blueprint before_request hooks, so any guard
    # written as bp.before_request would silently not run. Run them here.
    bp_name = endpoint.rsplit('.', 1)[0]
    for fn in current_app.before_request_funcs.get(bp_name, []):
        rv = current_app.ensure_sync(fn)()
        if rv is not None:
            return rv

    return current_app.ensure_sync(current_app.view_functions[endpoint])(**args)

app.register_blueprint(public, url_prefix=manifest['url_prefix'])
```

Verified behavior — same URLs, different code, per tenant:

```
acme    GET /calculator/        -> CALC-1.0  trace=['v1:before_request']
acme    GET /calculator/add/2/3 -> CALC-1.0 add=5
globex  GET /calculator/        -> CALC-2.0  trace=['v2:before_request']
globex  GET /calculator/add/2/3 -> CALC-2.0 add=5
unknown subpath -> 404
wrong method    -> 405
```

Because the inner match runs against the real URL map, everything Flask normally
gives you inside a version still works: converters (`<int:a>`), method matching,
405 on the wrong verb, 404 on an unknown path, and view decorators such as
`@login_required` and `@require('write')`.

### Building URLs

`url_for` inside a version returns the **private** path, which must never reach a
template. Use the helper:

```python
def plugin_url_for(view: str, **values) -> str:
    """Public URL for a view in the current tenant's pinned version."""
    slug, version = g.plugin_slug, g.plugin_version
    internal = url_for(f'{slug}_v{version}.{view}', **values)
    return internal.replace(f'/_v/{slug}/{version}', f'/{slug}', 1)
```

Register it as a Jinja global and use it in every plugin template:

```html
<a href="{{ plugin_url_for('invoice_detail', invoice_id=inv.id) }}">…</a>
```

Add a boot-time assertion in development that no plugin template contains a bare
`url_for(` — a leaked `/_v/…` URL still works, which is exactly why it will
survive review and then break when the tenant upgrades.

### Rules

- Plugin views must not be linked with `url_for` — use `plugin_url_for`.
- `/_v/` is private. Block it at the edge (Caddy) so it is unreachable from
  outside; the dispatcher reaches it internally, not over HTTP.
- One major version active per tenant at a time. Concurrent majors for one tenant
  is not supported and does not need to be.

---

## Versions and the Database

Routing is the easy half. The schema is where per-tenant versioning actually
costs something.

**Each major version owns its own tables.**

```
invoicing_v1_invoice     invoicing_v2_invoice
invoicing_v1_line        invoicing_v2_line
```

All of them exist in the schema at all times, for every tenant, because
migrations are global ([Traps](#traps)). A tenant pinned to v1 reads and writes
only the `_v1_` tables.

This is the only arrangement that actually works. Sharing one table set between
majors means v1 and v2 must agree on every column forever: v2 could add nullable
columns v1 ignores, but a renamed column, a changed type, or a new `NOT NULL`
breaks whichever tenant is on the other version. Separate tables also keep each
major's migration history independent, so v2's migrations cannot disturb a tenant
still on v1.

### Major versus minor

| Change | Version bump | Rule |
|--------|--------------|------|
| Bug fix, new optional field, new page | minor (1.0 → 1.1) | Replaces `v1/` in place. **Every tenant on v1 gets it immediately** — it must be backward compatible. |
| Renamed or dropped column, changed type, new required field, changed behavior | major (1.x → 2.0) | New `v2/` directory, new tables. Tenants upgrade explicitly. |

Minor releases are not pinnable. That is deliberate: pinning every minor gives
you an unbounded number of live code paths and table sets. If a minor cannot be
made backward compatible, it is a major.

### Upgrading a tenant

Upgrading is an explicit action that moves that tenant's data between table sets:

```python
def upgrade(org_id: int, slug: str, to_major: str) -> None:
    row = OrgPlugin.query.filter_by(org_id=org_id, plugin_slug=slug).one()
    from_major = row.version
    if from_major == to_major:
        return

    plugin = REGISTRY[slug][to_major]
    with transaction():
        plugin.on_upgrade_from(org_id, from_major)   # copy + transform this tenant's rows
        row.version = to_major
    g.pop('installed_plugins', None)
```

- Runs in one transaction. A failed migration leaves the tenant on the old
  version with data intact.
- Old tables are **not** cleared. Keeping v1 rows is what makes rollback a
  version-column flip rather than a restore from backup.
- Purge old data on the retention schedule, well after the upgrade is confirmed.

### The cost, plainly

Every supported major means another table set, another code path, and another
data migration to write and test. Keep the number of live majors small — two is
comfortable, three is a lot — and set an explicit end-of-life policy for old
majors, or you will be maintaining calculator 1.0 in five years because one
tenant never clicked upgrade.

---

## The Registry

Runs once inside `create_app`, before the first request. `REGISTRY` is keyed by
slug **and** major version.

```python
# app/platform/plugins/registry.py
import importlib, pkgutil
from pathlib import Path
from flask import Blueprint, g, abort, current_app, request

REGISTRY: dict[str, dict[str, 'Plugin']] = {}     # slug -> major -> Plugin
MANIFESTS: dict[str, dict] = {}

HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']


def discover() -> list[str]:
    """Slugs of every plugin directory, honouring __DISABLED__ marker files."""
    import plugins
    root = Path(plugins.__path__[0])
    return sorted(
        m.name for m in pkgutil.iter_modules(plugins.__path__)
        if not m.name.startswith('_') and not (root / m.name / '__DISABLED__').exists()
    )


def load_plugins(app):
    """Import every version of every plugin and wire up routing. Boot-time only."""
    for slug in _in_dependency_order(discover()):
        manifest = importlib.import_module(f'plugins.{slug}.__manifest__').manifest
        MANIFESTS[slug] = manifest
        REGISTRY[slug] = {}

        for major in manifest['versions']:
            module = importlib.import_module(f'plugins.{slug}.v{major}')

            # Import models for EVERY version, installed or not: migrations are
            # global and Alembic must see every table. See Traps.
            importlib.import_module(f'plugins.{slug}.v{major}.models')

            bp = module.plugin.blueprint()
            expected = f'{slug}_v{major}'
            if bp.name != expected:
                raise RuntimeError(f'Blueprint must be named {expected}, got {bp.name}')

            # Private mount. Never linked; never registered on the public prefix.
            app.register_blueprint(bp, url_prefix=f'/_v/{slug}/{major}')
            REGISTRY[slug][major] = module.plugin

        _register_dispatcher(app, slug, manifest)
        app.logger.info('plugin_loaded',
                        extra={'slug': slug, 'versions': manifest['versions']})
```

Requiring `bp.name == f'{slug}_v{major}'` gives collision detection for free:
Flask refuses two blueprints with the same name, so a duplicated slug or version
fails loudly at boot rather than silently shadowing routes.

`_in_dependency_order` topologically sorts by `requires` and raises on cycles or
missing dependencies — again at boot, where crashing is the good outcome.

---

## The Tenant Gate

There is no `before_request` guard on plugin blueprints any more. The public
dispatcher resolves the tenant's pinned version, and a tenant without a pin gets
a 404 before any version code runs:

```python
def installed_version(slug: str) -> str | None:
    """Major version pinned for the current tenant, or None if not installed."""
    if 'installed_plugins' not in g:
        org = getattr(g, 'org', None)
        g.installed_plugins = {} if org is None else {
            row.plugin_slug: row.version
            for row in OrgPlugin.query.filter_by(org_id=org.id, is_enabled=True)
        }
    return g.installed_plugins.get(slug)
```

One indexed lookup per plugin request, memoised for the rest of the request, and
nothing at all on core routes.

**Return 404, not 403.** A 403 confirms which plugins exist and which tenants
have them. As far as an uninstalled tenant is concerned, the route does not
exist.

A pin naming a version that is no longer on disk must fail loudly, not fall back
to a different one — silently serving v2 to a tenant pinned at v1 is the exact
failure this design exists to prevent:

```python
version = installed_version(slug)
if version is None:
    abort(404)
if version not in REGISTRY[slug]:
    raise RuntimeError(f'org {g.org.id} pinned {slug} v{version}, not on disk')
```

Guard against that at boot too: check every distinct `(plugin_slug, version)` in
`org_plugin` against what loaded, and refuse to start if a tenant is stranded.
That turns "we deleted v1 while a customer was still on it" into a failed deploy
instead of a production outage.

```python
# app/models/org_plugin.py
class OrgPlugin(BaseModel):
    """Which plugin, at which major version, each tenant runs.
    NOT OrgScoped — see below."""
    __tablename__ = 'org_plugin'

    org_id = db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                       nullable=False, index=True)
    plugin_slug = db.Column(db.String(50), nullable=False)
    version = db.Column(db.String(10), nullable=False)      # major only, e.g. '1'
    is_enabled = db.Column(db.Boolean, nullable=False, default=True)
    installed_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    upgraded_at = db.Column(db.DateTime(timezone=True), nullable=True)

    __table_args__ = (
        db.UniqueConstraint('org_id', 'plugin_slug', name='uq_org_plugin'),
    )
```

The unique constraint is on `(org_id, plugin_slug)`, not on the version — one
major per tenant per plugin, enforced by the database.

`OrgPlugin` deliberately does not inherit `OrgScoped`: the automatic filter reads
`g.org`, and this table is consulted while establishing what the tenant may
access. Keep the filter separate from the thing that gates it.

---

## Install and Uninstall

```python
# app/platform/plugins/lifecycle.py
def install(org_id: int, slug: str, version: str | None = None) -> None:
    if slug not in REGISTRY:
        raise NotFoundError(f'Unknown plugin: {slug}')
    version = version or MANIFESTS[slug]['default_version']
    if version not in REGISTRY[slug]:
        raise ValidationError(f'{slug} has no major version {version}')
    plugin = REGISTRY[slug][version]

    for dep in plugin.manifest.get('requires', []):
        install(org_id, dep)                       # idempotent, so recursion is safe

    with transaction():
        existing = OrgPlugin.query.filter_by(org_id=org_id, plugin_slug=slug).first()
        if existing:
            existing.is_enabled = True             # re-install keeps the pinned version
        else:
            db.session.add(OrgPlugin(org_id=org_id, plugin_slug=slug, version=version))
            plugin.on_install(org_id)              # seed data in the same transaction

    g.pop('installed_plugins', None)               # invalidate the request cache


def uninstall(org_id: int, slug: str) -> None:
    dependents = [s for s, m in MANIFESTS.items()
                  if slug in m.get('requires', [])
                  and s in _installed_for(org_id)]
    if dependents:
        raise ValidationError(f'Uninstall {", ".join(dependents)} first')

    with transaction():
        OrgPlugin.query.filter_by(org_id=org_id, plugin_slug=slug) \
                       .update({'is_enabled': False})
        REGISTRY[slug][row.version].on_uninstall(org_id)
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
    for slug, version in (g.get('installed_plugins') or {}).items():
        for entry in REGISTRY[slug][version].manifest.get('nav', []):
            if can(entry.get('permission', 'read')):
                items.append(entry)
    return {'plugin_nav': items}
```

```html
{% for item in plugin_nav %}
  <a href="{{ plugin_url_for(item.view) }}">{{ _(item.label) }}</a>
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

```bash
# config/local.env  --  slug:major, one deployment, one version each
PLUGINS=invoicing:2,reporting:1
```

```python
def load_plugins(app):
    for spec in app.config['PLUGINS'].split(','):
        slug, _, major = spec.partition(':')
        manifest = importlib.import_module(f'plugins.{slug}.__manifest__').manifest
        major = major or manifest['default_version']

        module = importlib.import_module(f'plugins.{slug}.v{major}')
        importlib.import_module(f'plugins.{slug}.v{major}.models')

        # Mounted directly on the public prefix: no dispatcher, no gate, because
        # there is only ever one version live.
        app.register_blueprint(module.plugin.blueprint(),
                               url_prefix=manifest['url_prefix'])
        REGISTRY.setdefault(slug, {})[major] = module.plugin
```

Enabling a plugin or changing its version requires a restart, which is entirely
reasonable for a single-tenant deployment. Skip the dispatcher, the gate, and
`plugin_url_for` — plain `url_for` works, because the version blueprint is
mounted on the public prefix.

The versioned table names (`invoicing_v2_invoice`) are still worth keeping: they
make a future major upgrade a data migration rather than a schema rewrite.

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

**Cache invalidation across workers.** `installed_version()` is memoised per
request, so a change on worker 1 is picked up by worker 2 on its next request.
Do not promote this to a process-level cache without a version counter or TTL —
you will get tenants seeing stale menus with no way to force a refresh.

**Namespace by slug and major**: `invoicing_v1_` tables, `invoicing_v1`
blueprint, `invoicing/` templates, `invoicing.` i18n keys, `/invoicing` public
prefix. Enforce at boot; a collision found at import time is free, one found in
production is not.

**Never register two versions on the same URL prefix.** Werkzeug accepts the
duplicate rules without error and the first-registered wins for every tenant, so
a pinned version is silently ignored. See
[Version Pinning](#why-the-obvious-approach-fails-silently).

**Never delete a major version that a tenant is still pinned to.** Add the
boot-time check described in [The Tenant Gate](#the-tenant-gate) so this fails
the deploy rather than the request.

**Minor releases ship to every tenant on that major immediately.** There is no
pinning below the major, so a minor that is not backward compatible breaks
tenants with no opt-out. If it cannot be made compatible, it is a major.

**Plugin templates must use `plugin_url_for`, never `url_for`.** A leaked
`/_v/<slug>/<major>/…` URL works when written, which is why it passes review, and
then serves the wrong version after the tenant upgrades.

---

**Next:** [Theming](theming.md) | [Tenancy](tenancy.md)
