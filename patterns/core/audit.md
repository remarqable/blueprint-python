# Audit Trail Pattern

Track who created and last updated records using an `AuditMixin`.

> **Applies when `audit_logging: true`** in the
> [Project Configuration](../../CLAUDE.md#-project-configuration)
> ([question 4](../../CLAUDE.md#the-questions-to-ask)). With `false`, add none
> of it. The flag name is historical: it turns on **record audit** (who created
> a row, who last changed it), which is
> [not an audit log](#this-is-not-an-audit-log).

---

## Table of Contents

- [Overview](#overview)
- [Implementation](#implementation)
- [Key Points](#key-points)
- [The Actor in Context](#the-actor-in-context)
- [Filling the Columns](#filling-the-columns)
- [Jobs and System Actors](#jobs-and-system-actors)
- [Tenancy](#tenancy)
- [Bulk Statements and Raw SQL](#bulk-statements-and-raw-sql)
- [Showing Names](#showing-names)
- [Deleting a User](#deleting-a-user)
- [Testing](#testing)
- [This is not an audit log](#this-is-not-an-audit-log)
- [Checklist](#checklist)

---

## Overview

The `AuditMixin` provides automatic tracking of:
- **created_by** - User who created the record (set once, never changes)
- **updated_by** - User who last modified the record (updated on each save)

These are system-managed, readonly fields from the user's perspective.

---

## Implementation

### 1. Create the Mixin

```python
# system/db/mixins.py
from sqlalchemy.ext.declarative import declared_attr
from system.db.database import db

class AuditMixin:
    """Mixin to track who created and last updated records."""

    created_by_id = db.Column(db.Integer, db.ForeignKey("user.id"), nullable=True)
    updated_by_id = db.Column(db.Integer, db.ForeignKey("user.id"), nullable=True)

    @declared_attr
    def created_by(cls):
        return db.relationship(
            "User",
            foreign_keys=[cls.created_by_id],
            lazy="joined",
        )

    @declared_attr
    def updated_by(cls):
        return db.relationship(
            "User",
            foreign_keys=[cls.updated_by_id],
            lazy="joined",
        )

    @property
    def created_by_name(self) -> str:
        """Return full name of creator."""
        if self.created_by:
            return f"{self.created_by.first_name} {self.created_by.last_name}"
        return ""

    @property
    def updated_by_name(self) -> str:
        """Return full name of last updater."""
        if self.updated_by:
            return f"{self.updated_by.first_name} {self.updated_by.last_name}"
        return ""
```

### 2. Apply to Models

```python
from flask_login import current_user
from system.db.mixins import AuditMixin

@ModelRegistry.register
class Contact(db.Model, AuditMixin):
    __tablename__ = "contact"
    # ... fields ...

    @classmethod
    def create(cls, **kwargs) -> "Contact":
        item = cls(**kwargs)
        if current_user and current_user.is_authenticated:
            item.created_by_id = current_user.id
        item.validate()
        db.session.add(item)
        db.session.commit()
        return item

    def update(self, **kwargs) -> "Contact":
        for key, value in kwargs.items():
            if hasattr(self, key):
                setattr(self, key, value)
        if current_user and current_user.is_authenticated:
            self.updated_by_id = current_user.id
        self.validate()
        db.session.commit()
        return self
```

### 3. Display in Templates

```html
<!-- In detail views -->
<div class="text-muted small">
    <p>Created: {{ item.created_at.strftime('%Y-%m-%d') }}
       {% if item.created_by_name %}by {{ item.created_by_name }}{% endif %}
    </p>
    {% if item.updated_by_name %}
    <p>Last updated by {{ item.updated_by_name }}</p>
    {% endif %}
</div>
```


> The module paths in steps 1 and 2 (`system/db/mixins.py`, `system.db.database`,
> `ModelRegistry`) are as first published, and existing projects import them.
> In the layout of [CLAUDE.md](../../CLAUDE.md#folder-structure), `db` comes
> from `app.extensions` and models subclass `BaseModel`. Whatever the module,
> keep the class, column and relationship names exactly as above. Everything
> after [Key Points](#key-points) is additive: steps 1 to 3 work without it.

---

## Key Points

- Fields are **nullable** to handle existing records and public submissions
- Use `@declared_attr` for relationships in mixins (SQLAlchemy requirement)
- Check `current_user.is_authenticated` before accessing `.id`
- The `*_name` properties provide convenient display formatting
- Use `lazy="joined"` to avoid N+1 queries when displaying lists

---

## The Actor in Context

The **actor** is the user responsible for a write. In a request it is
`current_user`. Outside one (a job, a CLI command) `current_user` is `None`, so
`create()` and `update()` above fill nothing: right for system work, wrong for
work a user asked for. One function answers "who is acting" in both places:

```python
# app/platform/audit.py
from contextlib import contextmanager
from contextvars import ContextVar

import sqlalchemy as sa
from flask import has_request_context
from flask_login import current_user
from sqlalchemy import event
from sqlalchemy.orm import Session

from system.db.mixins import AuditMixin

_override: ContextVar[int | None] = ContextVar('audit_actor', default=None)


def current_actor_id() -> int | None:
    """The user responsible for writes made now. None: system or anonymous."""
    override = _override.get()
    if override is not None:
        return override
    if has_request_context() and current_user.is_authenticated:
        return current_user.id
    return None


@contextmanager
def acting_as(user_id: int | None):
    """Credit writes in the block to user_id. Job worker and tests only."""
    token = _override.set(user_id)
    try:
        yield
    finally:
        _override.reset(token)
```

The actor is written to the database, so only two things set it: Flask-Login,
from the session, and the job worker, from the job row. Platform staff fixing a
tenant's record are credited as themselves, never as the user they are helping.

---

## Filling the Columns

The explicit lines in `create()` and `update()` cover the models that have them,
in a request. A `before_flush` listener covers every other path through the
session: `save()` ([database.md](database.md#base-model)), a bare
`db.session.add()`, a job running under `acting_as`.

```python
# app/platform/audit.py (continued)
@event.listens_for(Session, 'before_flush')
def _fill_audit_columns(session, _ctx, _instances):
    actor = current_actor_id()
    if actor is None:
        return                      # system or anonymous: leave the columns alone
    for obj in session.new:
        if isinstance(obj, AuditMixin):
            obj.created_by_id = actor
    for obj in session.dirty:
        if isinstance(obj, AuditMixin) and session.is_modified(obj):
            if sa.inspect(obj).attrs.created_by_id.history.has_changes():
                raise RuntimeError('created_by is set once and never changed')
            obj.updated_by_id = actor
```

Import the module once in `create_app` (`from .platform import audit`) so the
listener is registered. It sets the same values the explicit lines set, so the
two coexist; new models can rely on the listener and plain `save()`.

| Write | `created_by` | `updated_by` |
|-------|--------------|--------------|
| New row, an actor present | actor (a value the caller set is overwritten) | untouched (NULL) |
| Modified row, an actor present | kept | actor |
| Any write with no actor | the caller's value (an import's original author) or NULL | kept as it was |
| Assigning `created_by_id` on a stored row, an actor present | `RuntimeError`, nothing written | |
| Bulk statements, raw SQL | not filled | not filled |

A write with no actor keeps the last editor, as `update()` always has. When a
system job rewrites a row and the last person should no longer be credited,
that job sets `updated_by_id = None` itself.

### Never from request input

`update(**kwargs)` sets any attribute it is given. Passing it the form
(`contact.update(**request.form)`) with a `created_by_id` field in the body
rewrites the creator; without the listener nothing stops it. So:

- Controllers pass named fields (`update(name=..., email=...)`), never the whole
  form or JSON body ([mvc.md](mvc.md#controllers-thin-controllers)).
- No form field, header, query parameter or API field carries an actor.
- The listener is the backstop: it overwrites a forged creator on insert and
  refuses one on update.

---

## Jobs and System Actors

> Only when `jobs: true`. Otherwise skip this section.

**Work done on behalf of a user** (an import they uploaded, a bulk edit they
started) is credited to them. Record the actor on the job row when enqueueing
and restore it in the worker. Three additions to
[jobs.md](../jobs.md#enqueueing):

```python
# app/models/job.py -- one more column on Job
actor_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='SET NULL'),
                     nullable=True)

# app/platform/jobs.py -- enqueue() records who asked
return Job(name=name, payload=payload, org_id=org_id,
           actor_id=current_actor_id(),
           run_at=run_at or utcnow()).save()

# app/controllers/jobs_cli.py -- _execute() restores them around the handler
with acting_as(job.actor_id):
    HANDLERS[job.name](job.payload)
```

The actor is a column, not a key in `payload`: payloads are built from request
data, and the actor must not be. A job enqueued by another job inherits its
actor, because `enqueue()` runs inside `acting_as`.

**Work nobody asked for** (scheduled cleanup, derived fields, a sync from a
third party). `actor_id` is NULL, `acting_as(None)` changes nothing, and the
columns are left alone. Do not create a "system" row in `user` to point at: it
needs an email, shows up in member lists and pickers, and one day someone signs
in as it. To record *which* subsystem wrote a row, add a `source` column to that
table or write an audit log entry.

With `tenancy: shared` the worker has neither a tenant nor a `current_user`
([jobs.md](../jobs.md#tenancy)): the handler filters by the job's `org_id`, and
the actor comes from the job's `actor_id`.

---

## Tenancy

> Only when `tenancy: shared`. Otherwise skip this section.

- **`User` is global, not `OrgScoped`** ([tenancy.md](../tenancy.md#the-model)),
  so the two columns are ordinary foreign keys and the joined load of
  `created_by` is not touched by the tenant filter. An audited model lists as
  `class Project(OrgScoped, AuditMixin, BaseModel)`; scoping stays automatic.
- **Nothing checks that the actor is a member of the tenant**, and nothing needs
  to: the actor is the signed-in user that tenant resolution already checked,
  and it never comes from input.
- **Former members stay credited.** Someone who left the organization still
  created what they created. Show the name, and only the name: `created_by` is
  a whole `User`, email included.
- A "created by me" list needs an index that leads with the tenant:
  `db.Index('ix_project_org_created_by', 'org_id', 'created_by_id')`.

---

## Bulk Statements and Raw SQL

Statements that never build objects bypass the listener and the model methods
([performance.md](performance.md#insert-in-one-statement)):
`db.session.execute(sa.insert(Model), rows)`, `Model.query.filter(...).update({...})`,
`bulk_insert_mappings`, `bulk_save_objects`, and `sa.text(...)`. A bulk update
still moves `updated_at` (a column default), so the row looks edited by nobody.
Set the columns in the statement:

```python
# app/models/contact.py
actor = current_actor_id()

db.session.execute(sa.insert(Contact), [
    {'name': r['name'], 'created_by_id': actor} for r in rows])

Contact.query.filter(Contact.name == '').update(
    {'name': 'Unknown', 'updated_by_id': actor})

db.session.execute(
    sa.text('UPDATE contact SET name = :name, updated_by_id = :actor WHERE id = :id'),
    {'name': name, 'actor': actor, 'id': contact_id})
db.session.commit()
```

`actor` may be `None` (a script, a system job), which writes NULL. Never put
`created_by_id` in a bulk update.

---

## Showing Names

`lazy="joined"` on both relationships means a list of audited rows loads its
creators and editors in the same statement: one query for 3 rows and for 30
([performance.md](performance.md#relationship-loading)). Three things to know:

- **The `*_name` properties read `first_name` and `last_name`.** The `User` in
  [mvc.md](mvc.md#models-fat-models) has a single `name`, and on such a project
  the properties raise `AttributeError`. There, read the relationship:
  `{% if item.created_by %}{{ item.created_by.name }}{% endif %}`.
- **The join is paid by every query that returns the model**, and it loads whole
  `User` rows. A list that shows no names can drop it for that query with
  `.options(lazyload(Contact.created_by), lazyload(Contact.updated_by))`.
- **A query that selects columns instead of objects has no relationship.** Fetch
  the names for the page once, from the ids on it, with `User.names_by_id`
  ([performance.md](performance.md#fixing-it)): two queries however long the
  list.

A NULL creator renders nothing.

---

## Deleting a User

The two foreign keys have no `ondelete`, so the database refuses to delete a
user who created or edited an audited row (`IntegrityError`, on SQLite too once
the [foreign-key pragma](portability.md#sqlite-connection-pragmas) is on).
Prefer deactivating the account (`is_active = False`): history keeps its names.
When the row must go (an erasure request), clear the references first, in the
same transaction:

```python
# app/models/user.py
def clear_audit_references(self) -> None:
    """Before a hard delete: this user's rows stay, credited to no one."""
    for mapper in db.Model.registry.mappers:
        model = mapper.class_
        if issubclass(model, AuditMixin):
            for col in (model.created_by_id, model.updated_by_id):
                db.session.execute(
                    sa.update(model).where(col == self.id).values({col.key: None}))
```

It is a bulk statement on purpose: it crosses tenants, and it does not credit
the admin doing the deletion as the editor of every row. It does move
`updated_at` where the column has `onupdate`. New foreign keys to `user` that
record an actor (`Job.actor_id` above) use `ondelete='SET NULL'` instead.

---

## Testing

Test with two users ([testing.md](testing.md#fixtures)), at the model level,
where `login_user` inside `test_request_context()` stands in for a signed-in
request:

```python
# tests/test_models/test_audit.py
import pytest
from flask_login import login_user

from app.extensions import db
from app.models import Contact
from app.platform.audit import acting_as


def test_created_and_updated_by(app, users):
    alice, bob = users['alice'], users['bob']
    with app.test_request_context():
        login_user(alice)
        contact = Contact.create(name='Ada')
    assert contact.created_by_id == alice.id
    assert contact.updated_by_id is None             # not edited yet

    with app.test_request_context():
        login_user(bob)
        contact.update(name='Ada Lovelace')
        with pytest.raises(RuntimeError):            # a forged creator is refused
            contact.update(created_by_id=bob.id)
        db.session.rollback()
    assert (contact.created_by_id, contact.updated_by_id) == (alice.id, bob.id)


def test_outside_a_request(app, users):
    contact = Contact.create(name='Imported')         # no actor
    assert contact.created_by_id is None
    with acting_as(users['alice'].id):                # what the job worker does
        contact.update(name='Imported, then fixed')
    assert contact.updated_by_id == users['alice'].id
```

Add, per audited resource:

- **One controller test through the real login** that creates and edits a row
  with `authenticated_client` and asserts both columns. A route reached with
  no signed-in user writes NULL, silently.
- **A query-count test on its list page**
  ([performance.md](performance.md#catching-it)): the count with 3 rows equals
  the count with 30.
- With `jobs: true`, a test that enqueues in a request, calls `run_jobs()`, and
  asserts the rows the handler wrote carry the enqueuing user.

The `app` fixture keeps one app context open for the whole test, and Flask-Login
caches the loaded user on `g`. A second user signed in on the same `client`
is therefore still the first one. Call `g.pop('_login_user', None)` between the
two, or keep two-user cases at the model level as above.

---

## This is not an audit log

The columns answer "who made this row, and who touched it last". They keep no
history: each update overwrites the previous editor, a delete leaves nothing
behind, and they say nothing about *what* changed. That is enough for a
"Created by Alice, last updated by Bob" line and for nothing else, and it is all
`audit_logging: true` asks for.

An **audit log** is a different thing: an append-only table with one row per
significant action, answering "who did what, when, to what, from which value".
Build one when a customer, an auditor or your own support needs it: role
changes, deletions, exports, billing changes, sign-ins. The minimum shape is a
model with `actor_id` (nullable, `ondelete='SET NULL'`), `action` (a stable
name such as `member.role_changed`), `entity`, `entity_id`, `changes`
(a [`JSONColumn`](portability.md#portable-column-types) such as
`{'role': ['member', 'admin']}`), `created_at`, and `org_id` when
`tenancy: shared`. The rules that make it trustworthy:

- **Written in the same transaction as the change** (`db.session.add` both, one
  commit), so the action and its record commit or roll back together.
- **Append-only.** The application inserts and reads rows; it never updates or
  deletes them. `actor_id` comes from `current_actor_id()`, like the columns.
- **Not the application log.** Logs are rotated and readable by all of
  engineering; a trail people rely on lives in the database.

---

## Checklist

- [ ] Audited models inherit `AuditMixin`; column and relationship names unchanged
- [ ] `app/platform/audit.py` is imported in `create_app`
- [ ] No controller passes a whole form or JSON body to `create()` / `update()`;
      no form or API field carries an actor
- [ ] `jobs: true`: `Job.actor_id` set by `enqueue()`, restored with
      `acting_as` in the worker; system jobs run with no actor; no "system" user
- [ ] Bulk and raw writes to audited tables set the columns themselves
- [ ] User deletion deactivates, or clears the references first
- [ ] Tests: create/update, outside a request, forged creator, one controller
      test and one query-count test per resource
- [ ] Anything that must answer "what changed, and when" goes to an audit log,
      not these columns

---

**Next:** [Database](database.md) | [Performance](performance.md) | [Testing](testing.md)
