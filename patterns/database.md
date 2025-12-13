# Database Patterns

> SQLAlchemy, Alembic migrations, PostgreSQL/SQLite patterns

---

## Table of Contents

- [Database Choice](#database-choice)
- [SQLAlchemy Setup](#sqlalchemy-setup)
- [Migrations with Alembic](#migrations-with-alembic)
- [Model Conventions](#model-conventions)
- [Query Patterns](#query-patterns)
- [Transactions](#transactions)
- [JSONB Columns](#jsonb-columns-postgresql)
- [Full-Text Search](#full-text-search-postgresql)
- [Multi-Tenancy](#multi-tenancy)
- [Performance](#performance)

---

## Database Choice

### SQLite (Default)

SQLite is the default - no setup required. The database file is created automatically.

```bash
# DATABASE_URL format
sqlite:///app.db

# In-memory (for testing)
sqlite:///:memory:
```

**SQLite works great for:**
- Development and prototyping
- Small to medium production apps
- Single-server deployments
- Apps with <100 concurrent users

### PostgreSQL (Optional - When You Need It)

Add PostgreSQL when you need:
- JSONB columns for flexible data
- Full-text search
- Row-level security (multi-tenancy)
- High concurrency (100+ users)
- Horizontal scaling

```bash
# Install driver
pip install psycopg2-binary

# Start PostgreSQL
docker run --name app-db \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD=app \
  -e POSTGRES_DB=app \
  -p 5432:5432 -d postgres:15-alpine

# Update DATABASE_URL
export DATABASE_URL="postgresql://app:app@localhost:5432/app"
```

**Migration path:** Start with SQLite, switch to PostgreSQL when needed. SQLAlchemy makes this seamless - just change `DATABASE_URL`.

---

## SQLAlchemy Setup

### Extensions Configuration

```python
# app/extensions.py
from flask_sqlalchemy import SQLAlchemy
from flask_migrate import Migrate

db = SQLAlchemy()
migrate = Migrate()
```

### Config with Engine Options

```python
# app/config.py
import os

class Config:
    DATABASE_URL = os.environ.get('DATABASE_URL', 'sqlite:///app.db')
    SQLALCHEMY_DATABASE_URI = DATABASE_URL
    SQLALCHEMY_TRACK_MODIFICATIONS = False

    # PostgreSQL-specific options
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

## Migrations with Alembic

### Initialize Migrations

```bash
# First time setup
flask db init

# Create migration
flask db migrate -m "Create user table"

# Apply migration
flask db upgrade

# Rollback
flask db downgrade
```

### Migration Example

```python
# migrations/versions/001_create_user_table.py
"""Create user table

Revision ID: 001
"""
from alembic import op
import sqlalchemy as sa

revision = '001'
down_revision = None


def upgrade():
    op.create_table(
        'user',
        sa.Column('id', sa.BigInteger(), primary_key=True),
        sa.Column('email', sa.String(255), unique=True, nullable=False),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('avatar_url', sa.String(500), nullable=True),
        sa.Column('is_active', sa.Boolean(), default=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
    )
    op.create_index('ix_user_email', 'user', ['email'])


def downgrade():
    op.drop_table('user')
```

### Demo Data

```python
# migrations/versions/999_demo_data.py
"""Load demo data for development/testing"""

from alembic import op
from datetime import datetime

revision = '999'
down_revision = '001'


def upgrade():
    # Insert demo users
    op.execute("""
        INSERT INTO "user" (email, name, is_active, created_at, updated_at)
        VALUES
            ('alice@example.com', 'Alice', true, NOW(), NOW()),
            ('bob@example.com', 'Bob', true, NOW(), NOW())
    """)


def downgrade():
    op.execute("DELETE FROM \"user\" WHERE email IN ('alice@example.com', 'bob@example.com')")
```

---

## Model Conventions

### Naming

- **Tables**: lowercase, singular (`user`, `setting`)
- **Columns**: snake_case (`created_at`, `user_id`)
- **Foreign keys**: `<entity>_id` (`user_id`, `tenant_id`)
- **Indexes**: `ix_<table>_<column>` (`ix_user_email`)

### Base Model

```python
# app/models/base.py
from datetime import datetime
from app.extensions import db


class BaseModel(db.Model):
    __abstract__ = True

    id = db.Column(db.BigInteger, primary_key=True, autoincrement=True)
    created_at = db.Column(db.DateTime, nullable=False, default=datetime.utcnow)
    updated_at = db.Column(db.DateTime, nullable=False, default=datetime.utcnow,
                           onupdate=datetime.utcnow)

    def save(self):
        if hasattr(self, 'validate'):
            self.validate()
        db.session.add(self)
        db.session.commit()
        return self

    def delete(self):
        db.session.delete(self)
        db.session.commit()
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
count = User.query.count()
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

### Joins and Relationships

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

### Context Manager Pattern

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

## JSONB Columns (PostgreSQL)

### Model Definition

```python
from sqlalchemy.dialects.postgresql import JSONB

class User(BaseModel):
    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False)
    name = db.Column(db.String(100), nullable=False)
    metadata = db.Column(JSONB, default=dict)  # Flexible JSON storage
```

### Querying JSONB

```python
# Filter by JSON key
users = User.query.filter(User.metadata['role'].astext == 'admin').all()

# Check if key exists
users = User.query.filter(User.metadata.has_key('verified')).all()

# Contains
users = User.query.filter(User.metadata.contains({'active': True})).all()
```

### JSONB Index (Migration)

```python
def upgrade():
    # GIN index for JSONB queries
    op.execute("""
        CREATE INDEX ix_user_metadata ON "user"
        USING GIN (metadata)
    """)
```

---

## Full-Text Search (PostgreSQL)

### Model with Search Vector

```python
from sqlalchemy.dialects.postgresql import TSVECTOR

class Article(BaseModel):
    __tablename__ = 'article'

    title = db.Column(db.String(200), nullable=False)
    body = db.Column(db.Text, nullable=False)
    search_vector = db.Column(TSVECTOR)  # Generated column
```

### Migration for Full-Text Search

```python
def upgrade():
    op.execute("""
        ALTER TABLE article ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
            setweight(to_tsvector('english', coalesce(body, '')), 'B')
        ) STORED
    """)

    op.execute("""
        CREATE INDEX ix_article_search ON article USING GIN (search_vector)
    """)
```

### Search Query

```python
from sqlalchemy import func

@classmethod
def search(cls, query: str, limit: int = 20):
    """Full-text search on articles."""
    return cls.query.filter(
        cls.search_vector.match(query, postgresql_regconfig='english')
    ).order_by(
        func.ts_rank(cls.search_vector, func.plainto_tsquery('english', query)).desc()
    ).limit(limit).all()
```

---

## Multi-Tenancy

### Option 1: User-Owned Data (B2C)

Simple foreign key approach:

```python
class Setting(BaseModel):
    __tablename__ = 'setting'

    user_id = db.Column(db.BigInteger, db.ForeignKey('user.id'), nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text)

    @classmethod
    def for_user(cls, user_id: int):
        """Get all settings for a user."""
        return cls.query.filter_by(user_id=user_id).all()
```

### Option 2: Tenant-Based (B2B)

Add tenant_id to all tables:

```python
class BaseModel(db.Model):
    __abstract__ = True

    id = db.Column(db.BigInteger, primary_key=True)
    tenant_id = db.Column(db.BigInteger, db.ForeignKey('tenant.id'), nullable=False, index=True)
    # ... timestamps
```

### Row-Level Security (PostgreSQL)

```sql
-- Enable RLS
ALTER TABLE setting ENABLE ROW LEVEL SECURITY;

-- Policy: users can only see their own data
CREATE POLICY user_isolation ON setting
    USING (user_id = current_setting('app.user_id')::bigint);
```

```python
# Set user context before queries
@app.before_request
def set_user_context():
    if current_user.is_authenticated:
        db.session.execute(f"SET LOCAL app.user_id = {current_user.id}")
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
# Use only() to limit columns
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

### Connection Pooling

```python
SQLALCHEMY_ENGINE_OPTIONS = {
    'pool_size': 5,          # Permanent connections
    'max_overflow': 10,      # Extra connections when busy
    'pool_timeout': 30,      # Wait time for connection
    'pool_recycle': 300,     # Recycle connections after 5 min
    'pool_pre_ping': True,   # Check connection health
}
```

---

**Next:** [MVC Pattern](mvc.md) | [Testing](testing.md) | [Deployment](deployment.md)
