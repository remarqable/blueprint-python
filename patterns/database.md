# Database Patterns

> SQLite by default (local and production), PostgreSQL-compatible conventions, auto-migrations at runtime

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
DATABASE_URL=postgresql://user:pass@localhost:5432/app

# Install driver
pip install psycopg2-binary
```

---

## Auto-Migrations

**Migrations run automatically at startup.** No manual `flask db upgrade` needed.

### How It Works

```python
# app/__init__.py
from flask import Flask
from flask_migrate import upgrade
from .extensions import db, migrate


def create_app(config_class=Config):
    app = Flask(__name__)
    app.config.from_object(config_class)

    db.init_app(app)
    migrate.init_app(app, db)

    # Auto-run migrations on startup
    with app.app_context():
        upgrade()

    # ... rest of app setup
    return app
```

### Creating New Migrations

When you change models, create a migration:

```bash
# Generate migration from model changes
flask db migrate -m "Add user preferences"

# Review the generated migration in migrations/versions/
# Commit to git
```

The migration will auto-apply next time the app starts (locally or in production).

### Migration Best Practices

1. **Always review generated migrations** before committing
2. **Keep migrations small** - one logical change per migration
3. **Test migrations locally** before deploying
4. **Never edit applied migrations** - create new ones to fix issues

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
from datetime import datetime
from app.extensions import db


class BaseModel(db.Model):
    """Base model with common fields."""
    __abstract__ = True

    id = db.Column(db.BigInteger, primary_key=True, autoincrement=True)
    created_at = db.Column(db.DateTime, nullable=False, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, nullable=False, default=datetime.utcnow,
                           onupdate=datetime.utcnow)

    def save(self):
        """Save instance to database."""
        db.session.add(self)
        db.session.commit()
        return self

    def delete(self):
        """Delete instance from database."""
        db.session.delete(self)
        db.session.commit()
```

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

> **Blueprint Configuration:** This section depends on your app type (B2C or B2B).
> See [Project Configuration](#project-configuration) in CLAUDE.md for your app's settings.

### B2C: User-Owned Data

Each user owns their own data. Simple foreign key approach.

```python
# app/models/setting.py
class Setting(BaseModel):
    """User setting (key-value)."""
    __tablename__ = 'setting'

    user_id = db.Column(db.BigInteger, db.ForeignKey('user.id'), nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text)

    # Ensure unique key per user
    __table_args__ = (db.UniqueConstraint('user_id', 'key'),)

    # Relationship
    user = db.relationship('User', backref='settings')

    @classmethod
    def for_user(cls, user_id: int):
        """Get all settings for a user."""
        return cls.query.filter_by(user_id=user_id).all()

    @classmethod
    def get(cls, user_id: int, key: str, default=None):
        """Get a setting value."""
        setting = cls.query.filter_by(user_id=user_id, key=key).first()
        return setting.value if setting else default

    @classmethod
    def set(cls, user_id: int, key: str, value: str):
        """Set a setting value."""
        setting = cls.query.filter_by(user_id=user_id, key=key).first()
        if setting:
            setting.value = value
        else:
            setting = cls(user_id=user_id, key=key, value=value)
        return setting.save()
```

### B2B: Organization-Based

Users belong to organizations. All data is scoped to an organization.

```python
# app/models/organization.py
class Organization(BaseModel):
    """Organization (tenant)."""
    __tablename__ = 'organization'

    name = db.Column(db.String(100), nullable=False)
    slug = db.Column(db.String(100), unique=True, nullable=False, index=True)
    is_active = db.Column(db.Boolean, default=True, nullable=False)


# app/models/user.py
class User(BaseModel):
    """User belonging to an organization."""
    __tablename__ = 'user'

    org_id = db.Column(db.BigInteger, db.ForeignKey('organization.id'), nullable=False, index=True)
    email = db.Column(db.String(255), nullable=False, index=True)
    name = db.Column(db.String(100), nullable=False)
    role = db.Column(db.String(20), default='member', nullable=False)  # owner, admin, member

    # Unique email per organization
    __table_args__ = (db.UniqueConstraint('org_id', 'email'),)

    # Relationship
    organization = db.relationship('Organization', backref='users')


# app/models/base.py - for org-scoped models
class OrgScopedModel(BaseModel):
    """Base for organization-scoped models."""
    __abstract__ = True

    org_id = db.Column(db.BigInteger, db.ForeignKey('organization.id'), nullable=False, index=True)
```

### Query Helper (B2B)

```python
# app/platform/tenant.py
from flask import g
from functools import wraps


def get_current_org_id() -> int:
    """Get current organization ID from request context."""
    return g.get('org_id')


def org_scope(f):
    """Decorator to ensure org_id is set in queries."""
    @wraps(f)
    def decorated(*args, **kwargs):
        if not get_current_org_id():
            raise ValueError("No organization context set")
        return f(*args, **kwargs)
    return decorated


# Usage in model
class Project(OrgScopedModel):
    __tablename__ = 'project'

    name = db.Column(db.String(200), nullable=False)

    @classmethod
    def for_org(cls):
        """Get all projects for current org."""
        return cls.query.filter_by(org_id=get_current_org_id()).all()
```

---

## Query Patterns

### Basic Queries

```python
# Get by ID
user = User.query.get(1)
user = User.query.get_or_404(1)

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
    metadata = db.Column(JSONB, default=dict)  # Flexible JSON storage
```

```python
# Querying JSONB
users = User.query.filter(User.metadata['role'].astext == 'admin').all()
users = User.query.filter(User.metadata.has_key('verified')).all()
users = User.query.filter(User.metadata.contains({'active': True})).all()
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
