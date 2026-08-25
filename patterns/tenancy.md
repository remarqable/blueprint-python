# Multi-Tenancy

> **Layer doc.** Read this only when `tenancy: shared` in the Project
> Configuration. If `tenancy: personal`, skip this file entirely and use
> [core/database.md § User-Owned Data](core/database.md#user-owned-data).

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

        org = _from_subdomain() or _from_session()
        if org is None:
            return                      # public pages: login, marketing, signup

        if not org.is_active:
            abort(410, 'This workspace has been deactivated')

        if not current_user.is_authenticated:
            return

        membership = Membership.query.filter_by(
            user_id=current_user.id, org_id=org.id
        ).first()
        if membership is None:
            abort(404)                  # not a member: the tenant does not exist for you

        g.org = org
        g.membership = membership


def _from_subdomain():
    host = request.host.split(':')[0]
    base = current_app.config['BASE_DOMAIN']        # e.g. "example.com"
    if not host.endswith('.' + base):
        return None
    slug = host[: -(len(base) + 1)]
    if slug in Organization.RESERVED_SLUGS:
        return None
    return Organization.query.filter_by(slug=slug).first()
```

**Return 404, not 403,** when a user is not a member. A 403 confirms the
workspace exists, which leaks your customer list to anyone who can guess slugs.

### Choosing a resolution strategy

| Strategy | URL | Use when |
|----------|-----|----------|
| Subdomain | `acme.example.com/projects` | Default. Branded, cookie-isolatable, cache-friendly. |
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

**Next:** [Plugins](plugins.md) | [Theming](theming.md) | [Database](core/database.md)
