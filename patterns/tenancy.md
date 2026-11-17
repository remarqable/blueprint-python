# Multi-Tenancy

> **Layer doc.** Read this only when `tenancy: shared` in the Project
> Configuration. If `tenancy: personal`, skip this file entirely and use
> [core/database.md § User-Owned Data](core/database.md#personal-user-owned).

Shared database, one row per tenant, `org_id` on every business table. This is
the right default for almost every SaaS: one schema, one migration history, one
connection pool, and a straightforward path to schema-per-tenant later if a
large customer demands isolation.

---

## Table of Contents

- [The Model](#the-model)
- [Tenant Resolution](#tenant-resolution)
- [Safe-by-Default Scoping](#safe-by-default-scoping)
- [Roles and Permissions](#roles-and-permissions)
- [Invitations](#invitations)
- [Tenant Lifecycle](#tenant-lifecycle)
- [Testing Isolation](#testing-isolation)
- [Row-Level Security (opt-in)](#row-level-security-opt-in)

---

## The Model

Three tables carry the tenancy: `organization`, `user`, and `membership`.

Users are **global**, memberships are **scoped**. A user has one login and can
belong to several organizations — this costs nothing up front and is painful to
retrofit, because consultants, agencies, and your own support staff all need it
eventually.

```python
# app/models/organization.py
import sqlalchemy as sa
from app.extensions import db
from app.models.base import BaseModel
from app.models.types import BigIntPK, BigIntFK


class Organization(BaseModel):
    """A tenant."""
    __tablename__ = 'organization'

    name = db.Column(db.String(100), nullable=False)
    slug = db.Column(db.String(63), unique=True, nullable=False, index=True)
    theme = db.Column(db.String(50), nullable=False, default='default')
    is_active = db.Column(db.Boolean, nullable=False, default=True)

    RESERVED_SLUGS = {'www', 'api', 'admin', 'app', 'static', 'mail', 'status'}

    def validate(self):
        self.slug = (self.slug or '').strip().lower()
        if not re.fullmatch(r'[a-z0-9]([a-z0-9-]{1,61}[a-z0-9])?', self.slug):
            raise ValidationError('Slug must be 3-63 chars, a-z 0-9 and hyphens')
        if self.slug in self.RESERVED_SLUGS:
            raise ValidationError('That slug is reserved')
```

```python
# app/models/membership.py
class Membership(BaseModel):
    """Links a user to an organization with a role."""
    __tablename__ = 'membership'

    user_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='CASCADE'),
                        nullable=False, index=True)
    org_id = db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                       nullable=False, index=True)
    role = db.Column(db.String(20), nullable=False, default='member')

    __table_args__ = (
        db.UniqueConstraint('user_id', 'org_id', name='uq_membership_user_org'),
    )
```

> `Membership` is deliberately **not** `OrgScoped`. Scoped models are filtered by
> the current tenant, and you need to read a user's memberships *before* you know
> which tenant they are in. Same for `Organization` and `User`.

### The scoped mixin

Every business table inherits this. It is the single point where tenancy is
attached:

```python
# app/models/base.py
from sqlalchemy.orm import declared_attr


class OrgScoped:
    """Mixin: rows belong to exactly one organization.

    Models using this are AUTOMATICALLY filtered by the current tenant.
    See app/platform/tenant.py.
    """

    @declared_attr
    def org_id(cls):
        return db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                         nullable=False, index=True)


class Project(OrgScoped, BaseModel):
    __tablename__ = 'project'
    name = db.Column(db.String(200), nullable=False)
```

Composite indexes should lead with `org_id`, because every query filters on it:

```python
__table_args__ = (
    db.Index('ix_project_org_created', 'org_id', 'created_at'),
)
```

---

## Tenant Resolution

Resolve once, in `before_request`, and stash on `g`. Subdomain is the default
because it gives tenants a branded URL and makes cookie isolation possible.

```python
# app/platform/tenant.py
from flask import g, request, abort, session
from app.models import Organization, Membership


def init_tenant(app):
    @app.before_request
    def resolve_tenant():
        g.org = None
        g.membership = None

        if request.path.startswith(('/static/', '/health')):
            return

        org = _from_subdomain() or _default_org() or _from_session()
        if org is None:
            return                      # installation pages: login, signup, launcher

        if not org.is_active:
            abort(410, 'This workspace has been deactivated')

        # Set g.org BEFORE any auth/membership decision. The tenant filter
        # keys off g.org; leaving it None for a resolved host would serve
        # anonymous visitors UNSCOPED queries. See § Public Content below.
        g.org = org

        if current_user.is_authenticated:
            g.membership = Membership.query.filter_by(
                user_id=current_user.id, org_id=org.id
            ).first()

        # Private workspaces (the B2B default): the tenant does not exist
        # for outsiders. Public-content apps skip this and gate at the
        # content layer instead.
        if not app.config.get('PUBLIC_TENANTS') and g.membership is None:
            abort(404)                  # not 403: 403 confirms the workspace exists


def _from_subdomain():
    host = request.host.split(':')[0]
    base = current_app.config['BASE_DOMAIN']        # e.g. "example.com"
    if not host.endswith('.' + base):
        return None
    slug = host[: -(len(base) + 1)]
    if slug in Organization.RESERVED_SLUGS:
        return None
    return Organization.query.filter_by(slug=slug).first()


def _default_org():
    """Default-org mode: the bare domain answers for a sole organization.

    A single-org installation (most self-hosted installs) should not force
    its users through a subdomain. When the host IS the base domain and
    exactly one active organization exists, resolve to it. The moment a
    second organization appears, the bare domain reverts to installation
    pages (login, launcher) and subdomains take over.
    """
    host = request.host.split(':')[0]
    if host != current_app.config['BASE_DOMAIN']:
        return None
    orgs = Organization.query.filter_by(is_active=True).limit(2).all()
    return orgs[0] if len(orgs) == 1 else None
```

**Return 404, not 403,** when a user is not a member of a private workspace. A
403 confirms the workspace exists, which leaks your customer list to anyone who
can guess slugs.

### Public content and anonymous visitors

The classic version of this hook set `g.org` only after a successful
membership check — correct for a members-only B2B app, where every real route
sits behind `@login_required` anyway. It is **wrong for any app that serves
public content** (a published site, a blog, a community with public reads):
an anonymous request to `acme.example.com` would run with `g.org = None`,
the loader-criteria filter would decline to apply, and every `OrgScoped`
query on the page would read across **all tenants**.

The rules that keep public content safe:

- `g.org` is set whenever the host resolves to an organization — before and
  independent of authentication. Anonymous reads are scoped reads.
- `PUBLIC_TENANTS` (config) chooses what a resolved host means for outsiders:
  `False` (default) 404s non-members as above; `True` lets the request
  proceed with `g.membership = None`.
- With `PUBLIC_TENANTS = True`, *content* visibility (public vs member-only)
  is a model-layer concern — a `visibility` column checked in queries and
  enforced with `can()` — never an excuse to weaken the tenant filter.
- Writes are unaffected: the `before_flush` stamp still requires a resolved
  `g.org`, and write routes still require authentication and membership.

The isolation test for this is as important as the two at the bottom of this
document: request a public page of tenant A as an anonymous visitor and assert
tenant B's rows are absent.

### Choosing a resolution strategy

| Strategy | URL | Use when |
|----------|-----|----------|
| Subdomain | `acme.example.com/projects` | Default. Branded, cookie-isolatable, cache-friendly. |
| Default org | `example.com/projects` | Bare domain, exactly one org. Composes with subdomains: single-org installs get clean URLs, multi-org installs get the launcher. |
| Path prefix | `example.com/acme/projects` | No wildcard DNS/TLS available. Every `url_for` needs the slug. |
| Session only | `example.com/projects` | Users rarely belong to more than one org. Simplest; breaks multi-tab. |

Subdomains need a wildcard DNS record and a wildcard certificate. Caddy handles
the latter automatically with a DNS challenge — see
[core/deployment.md](core/deployment.md).

> **Cookie scope:** with subdomains, set `SESSION_COOKIE_DOMAIN` to the base
> domain so login persists across tenants — or leave it unset to force a
> separate session per tenant, which is stricter. Pick deliberately; the default
> is not obviously right either way.

---

## Safe-by-Default Scoping

The failure mode of multi-tenancy is one forgotten `WHERE org_id = ?`. Relying
on developers (or an agent) to remember it on every query is not a strategy.

Instead, apply the filter in one place, to every ORM query, for every model that
inherits `OrgScoped`:

```python
# app/platform/tenant.py
from sqlalchemy import event
from sqlalchemy.orm import Session, with_loader_criteria
from flask import g, has_request_context

from app.models.base import OrgScoped


@event.listens_for(Session, 'do_orm_execute')
def _apply_tenant_filter(state):
    if not state.is_select or state.is_column_load or state.is_relationship_load:
        return
    if state.session.info.get('unscoped'):
        return
    if not has_request_context():
        return                      # CLI, migrations, workers: use unscoped()

    org = getattr(g, 'org', None)
    if org is None:
        return

    org_id = org.id
    state.statement = state.statement.options(
        with_loader_criteria(OrgScoped, lambda cls: cls.org_id == org_id,
                             include_aliases=True)
    )
```

Verified behavior:

```
org 1 select                  -> ['acme-a', 'acme-b']
org 2 select                  -> ['globex-secret']
org 1 get(id=3) (globex row)  -> None
explicit unscoped             -> ['acme-a', 'acme-b', 'globex-secret']
```

The third line is the important one. `session.get(Project, 3)` — a primary-key
lookup on another tenant's row, i.e. the classic IDOR from a guessed URL —
returns `None` rather than the record. You do not need a manual ownership check
in every controller, because the row is not reachable.

`include_aliases=True` extends this to joins and eager loads, so
`select(Project).join(Task)` filters both sides.

### Writes still need the org set

The filter covers reads. Inserts must set `org_id`, so do it centrally too:

```python
@event.listens_for(Session, 'before_flush')
def _stamp_org(session, _ctx, _instances):
    if not has_request_context():
        return
    org = getattr(g, 'org', None)
    for obj in session.new:
        if isinstance(obj, OrgScoped):
            if obj.org_id is None:
                if org is None:
                    raise RuntimeError(f'{type(obj).__name__} created without a tenant')
                obj.org_id = org.id
            elif org is not None and obj.org_id != org.id:
                raise RuntimeError('Refusing to write across tenants')
```

### The escape hatch

Background jobs, admin tooling, and billing reconciliation legitimately need to
cross tenants. Make that explicit and greppable:

```python
from contextlib import contextmanager

@contextmanager
def unscoped():
    """Disable tenant filtering. Every use should be justified in review."""
    db.session.info['unscoped'] = True
    try:
        yield
    finally:
        db.session.info.pop('unscoped', None)


# app/jobs/billing.py
with unscoped():
    for org in Organization.query.filter_by(is_active=True):
        generate_invoice(org)
```

Grep for `unscoped()` in code review. It should be rare and always outside a
request context.

---

## Roles and Permissions

Three roles cover the overwhelming majority of B2B apps. Resist adding a
permission matrix until a customer actually asks.

```python
ROLES = {
    'owner':  {'billing', 'delete_org', 'manage_members', 'manage_plugins', 'write', 'read'},
    'admin':  {'manage_members', 'manage_plugins', 'write', 'read'},
    'member': {'write', 'read'},
}


def can(permission: str) -> bool:
    m = getattr(g, 'membership', None)
    return bool(m) and permission in ROLES.get(m.role, set())


def require(permission: str):
    def decorator(f):
        @wraps(f)
        def wrapped(*args, **kwargs):
            if not can(permission):
                abort(403)
            return f(*args, **kwargs)
        return wrapped
    return decorator
```

```python
@bp.route('/members/<int:member_id>/remove', methods=['POST'])
@login_required
@require('manage_members')
def remove_member(member_id): ...
```

Register `can` as a Jinja global so templates hide what the user cannot do —
while the decorator, not the template, does the actual enforcing.

**An organization must always have at least one owner.** Enforce it when
removing or demoting members, or accounts become unadministrable.

---

## Invitations

```python
class Invitation(BaseModel):
    __tablename__ = 'invitation'

    org_id = db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                       nullable=False, index=True)
    email = db.Column(db.String(255), nullable=False)
    role = db.Column(db.String(20), nullable=False, default='member')
    token_hash = db.Column(db.String(64), nullable=False, unique=True, index=True)
    expires_at = db.Column(db.DateTime(timezone=True), nullable=False)
    accepted_at = db.Column(db.DateTime(timezone=True), nullable=True)
```

Rules that matter:

- **Store a hash of the token, never the token.** Your database is a list of
  workspace-entry credentials otherwise. Same reasoning as magic links in
  [core/auth.md](core/auth.md).
- Expire in days, not weeks.
- Accepting creates a `Membership`; it does not mutate the invitation's role.
- Re-inviting an existing member is a no-op, not an error.
- If the invited email has no account, the signup flow must carry the token
  through so acceptance is atomic with account creation.

---

## Tenant Lifecycle

**Provisioning** — one transaction, or you get orphaned organizations with no
owner:

```python
def provision(name: str, slug: str, owner: User) -> Organization:
    with transaction():
        org = Organization(name=name, slug=slug)
        db.session.add(org)
        db.session.flush()                       # need org.id
        db.session.add(Membership(user_id=owner.id, org_id=org.id, role='owner'))
        for slug_, major in DEFAULT_PLUGINS:     # see plugins.md
            install_plugin(org.id, slug_, major)
    return org
```

**Suspension** — `is_active = False` returns 410 for the whole workspace while
keeping data intact. This is what non-payment should do.

**Deletion** — soft-delete, then purge on a schedule:

1. `is_active = False`, record `deletion_requested_at`.
2. Wait out the grace period (30 days is customary).
3. Hard-delete. With `ondelete='CASCADE'` on every `org_id` FK, deleting the
   `Organization` row removes everything — **but only if foreign keys are
   enforced**. On SQLite they are off by default and cascades silently do
   nothing, leaving orphaned tenant data behind after a GDPR deletion request.
   See [core/portability.md § SQLite Connection Pragmas](core/portability.md#sqlite-connection-pragmas).

**Export** — build it early. It is trivial while you have five tables and
miserable at fifty, and enterprise deals ask for it:

```python
def export(org_id: int) -> dict:
    with unscoped():
        return {
            m.__tablename__: [row.to_dict() for row in m.query.filter_by(org_id=org_id)]
            for m in ALL_ORG_SCOPED_MODELS
        }
```

---

## Testing Isolation

One test in the suite matters more than the rest: prove the filter holds.

```python
def test_tenant_cannot_read_other_tenants_rows(client, acme, globex):
    secret = ProjectFactory(org_id=globex.id, name='globex-secret')

    login_as(client, acme.owner)
    assert client.get(f'/projects/{secret.id}').status_code == 404
    assert b'globex-secret' not in client.get('/projects').data


def test_write_across_tenants_is_refused(app, acme, globex):
    login_as(client, acme.owner)
    with pytest.raises(RuntimeError):
        Project(org_id=globex.id, name='smuggled').save()
```

Add a fixture that creates two organizations for every controller test. If a new
endpoint leaks, a two-tenant fixture catches it and a single-tenant one never
will.

---

## Row-Level Security (opt-in)

> Applies only when `tenant_isolation: rls` in the
> [Project Configuration](../CLAUDE.md#-project-configuration), which requires
> `tenancy: shared` and `database: postgres`. With `tenant_isolation: app` (the
> default) **nothing in this section applies**: skip it and build none of it.

Everything above stays exactly as written. `OrgScoped`, the loader-criteria
filter and the `before_flush` stamp are still how queries are written and
scoped. RLS adds a second check underneath them, in PostgreSQL: a policy on
every `OrgScoped` table compares `org_id` with a per-transaction setting,
`app.org_id`. It covers what the application filter cannot see: raw SQL, Core
statements, bulk `update()`/`delete()`, a misplaced `unscoped()`.

It is easy to build RLS that enforces nothing and reports no error. Each part
below closes one way that happens.

### Two roles, two URLs

Superusers and table owners are exempt from RLS. An application that connects
as the role that ran the migrations has policies that are never applied.

```sql
-- once per database, as a superuser
CREATE ROLE app_owner LOGIN PASSWORD '...';               -- owns the tables, runs migrations
CREATE ROLE app_user  LOGIN PASSWORD '...' NOBYPASSRLS;   -- the web app and the worker
CREATE DATABASE app OWNER app_owner;
\c app
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO app_user;
```

On an existing database, the role that owns the tables today is `app_owner`:
skip its `CREATE ROLE` and the `CREATE DATABASE`, and run the rest.

```bash
# .env
DATABASE_URL=postgresql+psycopg://app_user:...@localhost:5432/app          # web and worker
DATABASE_OWNER_URL=postgresql+psycopg://app_owner:...@localhost:5432/app   # migrations only
```

The application never reads `DATABASE_OWNER_URL`. Only the migration command
does, everywhere it runs today (systemd `ExecStartPre`, `scripts/deploy.sh`,
the Docker entrypoint, `make run`; see [core/database.md](core/database.md#migrations)):

```bash
DATABASE_URL="$DATABASE_OWNER_URL" flask db upgrade
```

`RUN_MIGRATIONS_ON_STARTUP` stays `False`: `app_user` cannot alter the schema.

### The policy migration

`flask db migrate` does not generate policies. Write them by hand:

```python
# migrations/versions/xxxx_enable_rls.py
from alembic import op

# Every table whose model inherits OrgScoped, at this revision.
ORG_SCOPED_TABLES = ['project']

ORG_ID = "NULLIF(current_setting('app.org_id', true), '')::BIGINT"


def upgrade():
    if op.get_bind().dialect.name != 'postgresql':
        return
    for table in ORG_SCOPED_TABLES:
        op.execute(f'ALTER TABLE "{table}" ENABLE ROW LEVEL SECURITY')
        op.execute(f'ALTER TABLE "{table}" FORCE ROW LEVEL SECURITY')
        op.execute(f'''
            CREATE POLICY org_isolation ON "{table}"
              USING      (org_id = {ORG_ID})
              WITH CHECK (org_id = {ORG_ID})
        ''')


def downgrade():
    if op.get_bind().dialect.name != 'postgresql':
        return
    for table in ORG_SCOPED_TABLES:
        op.execute(f'DROP POLICY org_isolation ON "{table}"')
        op.execute(f'ALTER TABLE "{table}" NO FORCE ROW LEVEL SECURITY')
        op.execute(f'ALTER TABLE "{table}" DISABLE ROW LEVEL SECURITY')
```

- `USING` decides which rows a statement can see (SELECT, UPDATE, DELETE);
  `WITH CHECK` decides which rows it may write (INSERT, and the new value of an
  UPDATE). State both.
- `current_setting(..., true)` returns NULL when the setting was never set,
  instead of raising. `org_id = NULL` matches nothing.
- `NULLIF(..., '')` is required. Once a pooled connection has had the setting,
  later transactions read it as `''`, not NULL, and `''::BIGINT` raises.
- `FORCE` applies the policy to the owner too. Without it `app_owner` sees
  every row, so a `DATABASE_URL` pointed at the owner by mistake fails open.
- Tables that are not `OrgScoped` get no policy: `organization`, `user`,
  `membership`, `invitation` and `job` are read before the organization is known.
- Every later migration that creates an `OrgScoped` table repeats the three
  statements for it. A test below fails when one is forgotten.

`FORCE` has two costs for the owner. A data migration on an `OrgScoped` table
updates **zero rows** and reports success: put `NO FORCE ROW LEVEL SECURITY`
before it and `FORCE ROW LEVEL SECURITY` after it in the same migration (DDL
is transactional, so the window never commits). And `pg_dump` as `app_owner`
stops with `query would be affected by row-level security policy`: take
backups as a superuser.

### Setting the organization

`set_config('app.org_id', ..., true)` is `SET LOCAL` with a bind parameter. The
value lasts until the transaction ends, so it cannot reach the next request
that draws the same connection from the pool. The cost is that it must be set
again in **every** transaction, and `save()` commits, so one request has
several. The session's `after_begin` event runs at the start of each one:

```python
# app/platform/rls.py
from contextlib import contextmanager

import sqlalchemy as sa
from flask import g, has_request_context
from sqlalchemy import event
from sqlalchemy.orm import Session

from app.extensions import db

_SET_ORG = sa.text("SELECT set_config('app.org_id', :org_id, true)")


def _current_org_id(session) -> int | None:
    if 'org_id' in session.info:                 # org_scope(): jobs, CLI
        return session.info['org_id']
    org = getattr(g, 'org', None) if has_request_context() else None
    if org is None:
        return None
    # Not org.id: after a commit the object is expired, and loading it here
    # would run a query inside after_begin, which SQLAlchemy refuses.
    return sa.inspect(org).identity[0]


def _set_org(connection, org_id: int | None) -> None:
    if connection.dialect.name != 'postgresql':
        return
    connection.execute(_SET_ORG, {'org_id': '' if org_id is None else str(org_id)})


@event.listens_for(Session, 'after_begin')
def _set_org_on_begin(session, _transaction, connection):
    """Every transaction the session opens starts by naming its organization."""
    org_id = _current_org_id(session)
    if org_id is not None:
        _set_org(connection, org_id)


def init_rls(app):
    @app.before_request
    def set_org_on_open_transaction():
        # resolve_tenant() has already queried, so a transaction is open that
        # began before g.org was known. Name the organization on it now, or
        # clear it when this request has none.
        session = db.session()                   # the real Session, not the proxy
        org_id = _current_org_id(session)
        if org_id is not None or session.in_transaction():
            _set_org(session.connection(), org_id)


@contextmanager
def org_scope(org_id: int):
    """Run a block as one organization, outside a request (jobs, CLI, tests)."""
    db.session.info['org_id'] = org_id
    try:
        _set_org(db.session.connection(), org_id)
        yield
        _set_org(db.session.connection(), None)
    except Exception:
        db.session.rollback()
        raise
    finally:
        db.session.info.pop('org_id', None)
```

```python
# app/__init__.py -- tenant_isolation: rls only, directly after init_tenant(app)
from .platform.rls import init_rls
init_rls(app)
```

`init_rls` must come after `init_tenant`: `before_request` hooks run in
registration order, and this one needs `g.org` from
[Tenant Resolution](#tenant-resolution). Three rules:

- **Never `SET app.org_id`, and never `set_config(..., false)`.** A session-level
  value survives the pool's rollback and is handed to the next request.
- **Go through `db.session`.** A connection taken from `db.engine` never passes
  the hook, and on an autocommit connection `SET LOCAL` is a no-op.
- **No organization means no rows.** On installation pages (`g.org` is `None`),
  in `flask shell`, in a CLI command, and in either mistake above, every
  `OrgScoped` query returns zero rows and every insert raises. In `app` mode
  those paths see all rows; here an unexpectedly empty list means a missing
  scope, not an empty table.

A refused INSERT, or an UPDATE that moves a row to another organization, raises
`sqlalchemy.exc.ProgrammingError` (`new row violates row-level security
policy`). A cross-organization UPDATE or DELETE raises nothing: it matches zero
rows.

### Jobs and platform code

[`unscoped()`](#the-escape-hatch) switches off the application filter and
nothing else: under RLS the database still filters, so it no longer widens a
query. The worker connects as `app_user` like the web app, and code outside a
request names its organization with `org_scope`. When `jobs: true`
([jobs.md § Tenancy](jobs.md#tenancy)), wrap the handler call in `_execute`:

```python
# app/controllers/jobs_cli.py -- in _execute(), in place of the bare handler call
if job.org_id is not None:
    with org_scope(job.org_id):                  # a job for one organization
        HANDLERS[job.name](job.payload)
else:
    HANDLERS[job.name](job.payload)              # sees no OrgScoped rows by itself
```

Work that crosses organizations is a loop, each pass in its own scope:

```python
# app/jobs/billing.py
for org in Organization.query.filter_by(is_active=True):
    with org_scope(org.id):
        generate_invoice(org)
```

Deleting an `Organization` still cascades to its rows; foreign-key actions are
not subject to policies. If platform code needs one query across every
organization, give it a separate read-only role with `BYPASSRLS` and its own
URL. Never widen `app_user`, and never use that role in a request.

### Testing RLS

These tests pass on a broken setup in three ways: connected as a superuser or
the owner (the usual CI `postgres` user bypasses RLS), asserting through the
ORM (the application filter passes the test by itself), or run on SQLite. So
they connect as `app_user`, use raw SQL, and run on PostgreSQL even when
development uses SQLite. They need their own database, because the main `app`
fixture runs `drop_all()`; CI creates it and the two roles with the SQL above.

```python
# tests/conftest.py
import flask_migrate
import sqlalchemy as sa

RLS_URL = os.environ.get('TEST_RLS_DATABASE_URL')          # app_user
RLS_OWNER_URL = os.environ.get('TEST_RLS_OWNER_URL')       # app_owner, same database


@pytest.fixture
def rls_app():
    if not (RLS_URL and RLS_OWNER_URL):
        if os.environ.get('CI'):
            pytest.fail('CI must set TEST_RLS_DATABASE_URL and TEST_RLS_OWNER_URL')
        pytest.skip('RLS tests need PostgreSQL and both roles')

    class OwnerConfig(TestConfig):
        SQLALCHEMY_DATABASE_URI = RLS_OWNER_URL
        SQLALCHEMY_ENGINE_OPTIONS = {}

    class RlsConfig(TestConfig):
        SQLALCHEMY_DATABASE_URI = RLS_URL
        SQLALCHEMY_ENGINE_OPTIONS = {'pool_size': 1, 'max_overflow': 0}   # reuse is certain

    with create_app(OwnerConfig).app_context():      # the real migrations, as the owner
        flask_migrate.upgrade()
        tables = ', '.join(f'"{t.name}"' for t in db.metadata.sorted_tables)
        db.session.execute(sa.text(f'TRUNCATE {tables} RESTART IDENTITY CASCADE'))
        db.session.commit()
        db.engine.dispose()

    app = create_app(RlsConfig)
    with app.app_context():
        yield app
        db.session.remove()
        db.engine.dispose()
```

```python
# tests/test_rls.py
import pytest
import sqlalchemy as sa
from flask import jsonify
from sqlalchemy.exc import ProgrammingError

from app.extensions import db
from app.models import Membership, Organization, Project, User
from app.models.base import OrgScoped
from app.platform.rls import org_scope

DENIED = 'row-level security'


def names():
    # Raw SQL on purpose: an ORM query would pass on the application filter alone.
    return db.session.execute(sa.text('SELECT name FROM project ORDER BY id')).scalars().all()


@pytest.fixture
def two_orgs(rls_app):
    ids = []
    for slug, project in (('acme', 'acme-a'), ('globex', 'globex-secret')):
        org = Organization(name=slug, slug=slug).save()
        with org_scope(org.id):
            Project(org_id=org.id, name=project).save()
        ids.append(org.id)
    return ids


def test_runtime_role_cannot_bypass(rls_app):
    assert db.session.execute(sa.text(
        "SELECT rolsuper OR rolbypassrls OR EXISTS (SELECT 1 FROM pg_tables "
        "WHERE schemaname = 'public' AND tableowner = current_user) "
        "FROM pg_roles WHERE rolname = current_user")).scalar() is False


def test_every_org_scoped_table_has_a_forced_policy(rls_app):
    tables = {m.local_table.name for m in db.Model.registry.mappers
              if issubclass(m.class_, OrgScoped)}
    protected = set(db.session.execute(sa.text(
        "SELECT c.relname FROM pg_class c JOIN pg_policies p "
        "  ON p.schemaname = 'public' AND p.tablename = c.relname "
        "WHERE c.relnamespace = 'public'::regnamespace "
        "  AND c.relrowsecurity AND c.relforcerowsecurity "
        "  AND p.qual IS NOT NULL AND p.with_check IS NOT NULL")).scalars())
    assert tables and tables <= protected, tables - protected


def test_isolation_in_every_direction(two_orgs):
    acme, globex = two_orgs
    with org_scope(acme):
        assert names() == ['acme-a']                                        # read
        run = lambda sql: db.session.execute(sa.text(sql), {'g': globex})
        assert run("UPDATE project SET name = 'x' WHERE org_id = :g").rowcount == 0
        assert run('DELETE FROM project WHERE org_id = :g').rowcount == 0
        smuggle = sa.insert(Project.__table__).values(org_id=globex, name='smuggled')
        move = sa.text('UPDATE project SET org_id = :g')                   # own row, other org
        for statement in (smuggle, move):
            with pytest.raises(ProgrammingError, match=DENIED):
                db.session.execute(statement, {'g': globex})
            db.session.rollback()
    with org_scope(globex):
        assert names() == ['globex-secret']                                 # untouched


def test_no_org_means_no_rows_and_no_writes(two_orgs):
    assert names() == []
    with pytest.raises(ProgrammingError, match=DENIED):
        db.session.execute(sa.insert(Project.__table__).values(org_id=two_orgs[0], name='orphan'))


def test_next_request_on_the_same_connection_inherits_nothing(rls_app, two_orgs):
    @rls_app.get('/_rls_probe')
    def probe():
        pid = db.session.execute(sa.text('SELECT pg_backend_pid()')).scalar()
        return jsonify(names=names(), pid=pid)

    owner = User.create('owner@acme.test', 'Owner')
    Membership(user_id=owner.id, org_id=two_orgs[0], role='owner').save()
    client = rls_app.test_client()
    login_as(client, owner)

    first = client.get('/_rls_probe', base_url='http://acme.localhost').json
    second = client.get('/_rls_probe', base_url='http://localhost').json    # two orgs: no default org
    assert first['names'] == ['acme-a']
    assert second['names'] == [] and second['pid'] == first['pid']
```

Policy predicates are planned like any other condition, so they are as fast as
the index behind them: keep `org_id` leading every composite index, and never
put a subquery in a policy. The hook adds one round trip per transaction.

### RLS checklist

- [ ] `tenant_isolation: rls` recorded, with `tenancy: shared` and `database: postgres`
- [ ] `DATABASE_URL` is `app_user`: `NOBYPASSRLS`, not a superuser, owns no table
- [ ] Migrations run as `app_owner` through `DATABASE_OWNER_URL`, nowhere else
- [ ] Every `OrgScoped` table: `ENABLE` and `FORCE ROW LEVEL SECURITY`, one policy
      with `USING` and `WITH CHECK`
- [ ] The policy reads `NULLIF(current_setting('app.org_id', true), '')::BIGINT`
- [ ] Each new `OrgScoped` table gets its policy in the migration that creates it
- [ ] `init_rls(app)` is called directly after `init_tenant(app)`
- [ ] `app.org_id` is set only by `app/platform/rls.py`, always with `true`
- [ ] Jobs and CLI code enter `org_scope`; cross-organization work loops over it
- [ ] Data migrations on `OrgScoped` tables lift and restore `FORCE`; backups
      run as a superuser
- [ ] `tests/test_rls.py` runs in CI on PostgreSQL as `app_user`, and fails
      rather than skips when its URLs are missing

---

**Next:** [Plugins](plugins.md) | [Theming](theming.md) | [Database](core/database.md)
