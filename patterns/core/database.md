# Database Patterns

> SQLite by default, PostgreSQL when you outgrow it, switchable via DATABASE_URL.
> Engine differences: [portability.md](portability.md)

---

## Table of Contents

- [Database Choice](#database-choice)
- [Auto-Migrations](#auto-migrations)
- [Model Conventions](#model-conventions)
- [Multi-Tenancy](#multi-tenancy)
- [Query Patterns](#query-patterns)
- [Transactions](#transactions)
- [PostgreSQL Features](#postgresql-features)
- [Performance](#performance)

---

## Database Choice

### SQLite (Default - Local & Production)

SQLite is the default for both development and production. No setup required.

```bash
# DATABASE_URL format
DATABASE_URL=sqlite:///app.db

# In-memory (for testing)
DATABASE_URL=sqlite:///:memory:
```

**SQLite works great for:**
- Development and prototyping
- Small to medium production apps (most SaaS apps)
- Single-server deployments
- Apps with <100 concurrent writers

**Why SQLite in production?**
- Zero configuration, zero maintenance
- No separate database server to manage
- Backup is just copying a file
- Fast for read-heavy workloads
- Good enough for 99% of apps

### PostgreSQL (When You Need It)

Switch to PostgreSQL when you need:
- JSONB columns for flexible data
- Full-text search
- High write concurrency (100+ concurrent writers)
- Horizontal scaling / read replicas

**Migration path:** Just change `DATABASE_URL` in your .env file. The codebase uses PostgreSQL-compatible conventions (BigInteger IDs, proper column types) so migration is seamless.

```bash
# Switch to PostgreSQL
DATABASE_URL=postgresql+psycopg://user:pass@localhost:5432/app

# Install driver
uv add "psycopg[binary]"
```

---

## Migrations

Migrations run **once, before the app starts** -- not inside `create_app`.

### Why not at startup

Running `upgrade()` in the application factory looks convenient and breaks under
the only deployment that matters. With N Gunicorn workers, all N boot at once and
race the same Alembic upgrade against the same database, with no advisory lock on
SQLite. You get partially applied migrations, `database is locked`, or a
corrupted `alembic_version` -- intermittently, under load, on deploy.

It also makes the test suite lie: `create_app()` upgrades whatever database the
config points at *before* a fixture can redirect it.

### Where they belong

One process, before any worker starts. In systemd:

```ini
# /etc/systemd/system/yourapp.service
[Service]
ExecStartPre=/opt/yourapp/.venv/bin/flask db upgrade
ExecStart=/opt/yourapp/.venv/bin/gunicorn -w 4 -b 127.0.0.1:8000 wsgi:app
```

`ExecStartPre` runs once and must exit 0, so a failed migration aborts the deploy
instead of starting a server against a half-migrated schema. `scripts/deploy.sh`
does the same for the non-systemd path.

For local development, `make run` runs `flask db upgrade` before `run.py` -- a
single process, so the race cannot occur.

If you truly want migrations in-process (single-worker containers, say), gate it
explicitly and leave it off by default:

```python
if app.config.get('RUN_MIGRATIONS_ON_STARTUP'):
    with app.app_context():
        upgrade()
```

### Creating migrations

```bash
flask db migrate -m "Add user preferences"    # generate from model changes
flask db upgrade                              # apply locally
```

1. **Always review generated migrations** before committing -- autogenerate
   misses table renames, CHECK constraints, and server defaults.
2. **Keep them small** -- one logical change each.
3. **Never edit an applied migration** -- write a new one.
4. **Use `batch_alter_table`** so migrations work on SQLite:
   [portability.md](portability.md#migrations-that-run-on-both).

---

## Model Conventions

### Naming

- **Tables**: lowercase, singular (`user`, `setting`, `organization`)
- **Columns**: snake_case (`created_at`, `user_id`)
- **Foreign keys**: `<entity>_id` (`user_id`, `org_id`)
- **Indexes**: `ix_<table>_<column>` (`ix_user_email`)

### Base Model

```python
# app/models/base.py
from datetime import datetime, timezone
import sqlalchemy as sa
from app.extensions import db
from app.models.types import BigIntPK


def utcnow() -> datetime:
    """Timezone-aware UTC now. Never use datetime.utcnow() -- it is deprecated
    in Python 3.12+ and returns a naive datetime that claims to be UTC."""
    return datetime.now(timezone.utc)


class BaseModel(db.Model):
    """Base model with common fields."""
    __abstract__ = True

    # BigIntPK is BIGINT on PostgreSQL, INTEGER on SQLite. A plain BigInteger
    # primary key does NOT autoincrement on SQLite and every INSERT fails with
    # "NOT NULL constraint failed". See core/portability.md.
    id = db.Column(BigIntPK, primary_key=True, autoincrement=True)
    created_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    updated_at = db.Column(db.DateTime(timezone=True), nullable=False,
                           default=utcnow, onupdate=utcnow)

    def save(self):
        """Validate and persist. Commits immediately -- for multi-step
        operations use transaction() instead so they stay atomic."""
        if hasattr(self, 'validate'):
            self.validate()
        db.session.add(self)
        db.session.commit()
        return self

    def delete(self):
        db.session.delete(self)
        db.session.commit()

    @classmethod
    def get_by_id(cls, id: int):
        return db.session.get(cls, id)      # Query.get() is legacy in SQLAlchemy 2.0
```

> **Constraint naming is not optional.** Set the naming convention before your
> first migration -- SQLite's table-copy ALTER cannot recreate constraints it
> cannot name. See [portability.md](portability.md#migrations-that-run-on-both).

### User Model

```python
# app/models/user.py
from app.extensions import db
from app.models.base import BaseModel


class User(BaseModel):
    """User model."""
    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False, index=True)
    name = db.Column(db.String(100), nullable=False)
    avatar_url = db.Column(db.String(500), nullable=True)
    is_active = db.Column(db.Boolean, default=True, nullable=False)

    @classmethod
    def get_by_email(cls, email: str):
        """Find user by email."""
        return cls.query.filter_by(email=email.lower()).first()

    @classmethod
    def create(cls, email: str, name: str) -> 'User':
        """Create a new user."""
        user = cls(email=email.lower(), name=name)
        return user.save()
```

---

## Multi-Tenancy

Which ownership column your models carry depends on the `tenancy` setting in
[Project Configuration](../../CLAUDE.md#project-configuration).

### Shared (default): organization-owned

Business tables inherit `OrgScoped` and are filtered by the current tenant
automatically. **Read [tenancy.md](../tenancy.md)** -- the model, tenant
resolution, and the session-level filter that makes cross-tenant leaks
structurally impossible all live there.

```python
class Project(OrgScoped, BaseModel):
    __tablename__ = 'project'
    name = db.Column(db.String(200), nullable=False)
```

### Personal: user-owned

For apps where data is never shared between users. Do not read tenancy.md.

```python
class Setting(BaseModel):
    __tablename__ = 'setting'

    user_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='CASCADE'),
                        nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text, nullable=False, default='')

    __table_args__ = (db.UniqueConstraint('user_id', 'key', name='uq_setting_user_key'),)

    @classmethod
    def get_value(cls, user_id: int, key: str, default: str = '') -> str:
        setting = cls.query.filter_by(user_id=user_id, key=key).first()
        return setting.value if setting else default
```

> Choosing between them is about the future, not your go-to-market label: **will
> data ever be shared between users?** Retrofitting `org_id` means backfilling an
> organization per user and rewriting every query and permission check. Carrying
> it from day one costs one column.

---

## Query Patterns

### Basic Queries

```python
# Get by ID
user = db.session.get(User, 1)          # Query.get() is legacy in 2.0
user = db.get_or_404(User, 1)

# Filter
user = User.query.filter_by(email='alice@example.com').first()
users = User.query.filter(User.is_active == True).all()

# Order and limit
users = User.query.order_by(User.created_at.desc()).limit(10).all()

# Count
count = User.query.filter_by(is_active=True).count()
```

### Pagination

```python
@classmethod
def list_paginated(cls, page: int = 1, per_page: int = 20):
    """List with pagination."""
    return cls.query.order_by(cls.created_at.desc()) \
        .paginate(page=page, per_page=per_page, error_out=False)

# Usage in controller
pagination = User.list_paginated(page=request.args.get('page', 1, type=int))
users = pagination.items
total_pages = pagination.pages
```

### Joins and Eager Loading

```python
# Eager loading (avoid N+1)
users = User.query.options(db.joinedload(User.settings)).all()

# Filter by relationship
users = User.query.join(Setting).filter(Setting.key == 'theme').all()
```

---

## Transactions

### Automatic (Default)

```python
# Each save() commits automatically
user = User(email='test@example.com', name='Test')
user.save()  # Commits here
```

### Manual Transaction

```python
from app.extensions import db

def transfer_credits(from_user_id: int, to_user_id: int, amount: int):
    """Transfer credits between users (atomic operation)."""
    try:
        from_user = User.query.get(from_user_id)
        to_user = User.query.get(to_user_id)

        from_user.credits -= amount
        to_user.credits += amount

        db.session.commit()
    except Exception:
        db.session.rollback()
        raise
```

### Context Manager

```python
from contextlib import contextmanager
from app.extensions import db


@contextmanager
def transaction():
    """Transaction context manager with auto-rollback."""
    try:
        yield db.session
        db.session.commit()
    except Exception:
        db.session.rollback()
        raise


# Usage
with transaction():
    user1.credits -= 100
    user2.credits += 100
```

---

## PostgreSQL Features

> These features only work with PostgreSQL. SQLite will ignore them.

### JSONB Columns

```python
from sqlalchemy.dialects.postgresql import JSONB

class User(BaseModel):
    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False)
    meta = db.Column(JSONColumn, default=dict)   # NOT 'metadata': reserved by Declarative
```

```python
# Querying JSONB
users = User.query.filter(User.meta['role'].astext == 'admin').all()
users = User.query.filter(User.meta.has_key('verified')).all()
users = User.query.filter(User.meta.contains({'active': True})).all()
```

### Full-Text Search

```python
from sqlalchemy.dialects.postgresql import TSVECTOR

class Article(BaseModel):
    __tablename__ = 'article'

    title = db.Column(db.String(200), nullable=False)
    body = db.Column(db.Text, nullable=False)
    search_vector = db.Column(TSVECTOR)  # Generated column
```

Migration for FTS:

```python
def upgrade():
    op.execute("""
        ALTER TABLE article ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
            setweight(to_tsvector('english', coalesce(body, '')), 'B')
        ) STORED
    """)
    op.execute("CREATE INDEX ix_article_search ON article USING GIN (search_vector)")
```

---

## Performance

### Indexes

```python
# Single column index
email = db.Column(db.String(255), index=True)

# Composite index
__table_args__ = (
    db.Index('ix_setting_user_key', 'user_id', 'key'),
)
```

### Query Optimization

```python
# Select specific columns only
users = User.query.options(db.load_only(User.id, User.email)).all()

# Batch inserts
db.session.bulk_insert_mappings(User, [
    {'email': 'user1@example.com', 'name': 'User 1'},
    {'email': 'user2@example.com', 'name': 'User 2'},
])
db.session.commit()

# Batch updates
User.query.filter(User.is_active == False).update({'is_active': True})
db.session.commit()
```

### Connection Pooling (PostgreSQL)

```python
# app/config.py
if DATABASE_URL.startswith('postgresql'):
    SQLALCHEMY_ENGINE_OPTIONS = {
        'pool_size': 5,
        'max_overflow': 10,
        'pool_timeout': 30,
        'pool_recycle': 300,
        'pool_pre_ping': True,
    }
```

---

**Next:** [MVC Pattern](mvc.md) | [Testing](testing.md) | [Deployment](deployment.md)
