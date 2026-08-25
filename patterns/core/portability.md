# Database Portability: SQLite ↔ PostgreSQL

> One codebase, two engines. Switch by changing `DATABASE_URL` — nothing else.

SQLite is the default: zero setup, a single file to back up, fine for most SaaS
workloads. PostgreSQL is there when you outgrow it. The rule is that **you never
rewrite application code to move between them** — you change one environment
variable.

That only holds if you follow the conventions below. They are not stylistic;
each one fixes a concrete way the two engines diverge.

---

## Table of Contents

- [Switching Engines](#switching-engines)
- [Portable Column Types](#portable-column-types)
- [SQLite Connection Pragmas](#sqlite-connection-pragmas)
- [Migrations That Run on Both](#migrations-that-run-on-both)
- [Portable Query Idioms](#portable-query-idioms)
- [What Is Not Portable](#what-is-not-portable)
- [Testing Against Both](#testing-against-both)
- [Migrating SQLite → PostgreSQL](#migrating-sqlite--postgresql)

---

## Switching Engines

```bash
# SQLite (default)
DATABASE_URL=sqlite:///app.db

# PostgreSQL
DATABASE_URL=postgresql+psycopg://user:pass@localhost:5432/app
```

```python
# app/config.py
class Config:
    DATABASE_URL = os.environ.get('DATABASE_URL', 'sqlite:///app.db')
    SQLALCHEMY_DATABASE_URI = DATABASE_URL
    SQLALCHEMY_TRACK_MODIFICATIONS = False

    IS_POSTGRES = DATABASE_URL.startswith('postgresql')
    IS_SQLITE = DATABASE_URL.startswith('sqlite')

    if IS_POSTGRES:
        SQLALCHEMY_ENGINE_OPTIONS = {
            'pool_size': 5,
            'max_overflow': 10,
            'pool_timeout': 30,
            'pool_recycle': 300,
            'pool_pre_ping': True,
        }
    else:
        # SQLite: pooling options above are meaningless and some raise.
        SQLALCHEMY_ENGINE_OPTIONS = {
            'connect_args': {'timeout': 30},
        }
```

Install the PostgreSQL driver only when you need it:

```bash
pip install "psycopg[binary]"
```

---

## Portable Column Types

Declare types once in `app/models/types.py` and use them everywhere. Each alias
below compiles to the right thing on both engines — verified output:

| Alias | PostgreSQL | SQLite |
|-------|-----------|--------|
| `BigIntPK` | `BIGINT` | `INTEGER` |
| `JSONColumn` | `JSONB` | `JSON` |
| `sa.Uuid` | `UUID` | `CHAR(32)` |
| `sa.DateTime(timezone=True)` | `TIMESTAMP WITH TIME ZONE` | `DATETIME` |
| `sa.Numeric(12, 2)` | `NUMERIC(12, 2)` | `NUMERIC(12, 2)` |

```python
# app/models/types.py
"""Column types that compile correctly on both SQLite and PostgreSQL."""

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

# BIGINT primary keys do NOT autoincrement on SQLite: only INTEGER PRIMARY KEY
# is a rowid alias. Without this variant every INSERT fails with
# "NOT NULL constraint failed: <table>.id".
BigIntPK = sa.BigInteger().with_variant(sa.Integer, 'sqlite')

# Foreign keys must match the referenced column's type on both engines.
BigIntFK = sa.BigInteger().with_variant(sa.Integer, 'sqlite')

# JSONB on Postgres (indexable, binary), plain JSON on SQLite.
JSONColumn = sa.JSON().with_variant(postgresql.JSONB, 'postgresql')

# Money: never float. NUMERIC is identical on both.
Money = sa.Numeric(12, 2)
```

### The BigInteger trap

This is the single most important line in this document. On SQLite, only
`INTEGER PRIMARY KEY` is an alias for the rowid, so a `BIGINT` primary key never
autoincrements:

```
CREATE TABLE "user" (id BIGINT NOT NULL, email VARCHAR(255) NOT NULL, PRIMARY KEY (id))

>>> session.add(User(email='a@b.c')); session.commit()
sqlalchemy.exc.IntegrityError: (sqlite3.IntegrityError)
NOT NULL constraint failed: user.id
```

`with_variant(sa.Integer, 'sqlite')` fixes it while keeping `BIGINT` on
PostgreSQL. SQLite's `INTEGER` is a variable-width signed 64-bit value, so you
lose no range.

### Timestamps

Always store UTC. SQLite has no timezone type — `DateTime(timezone=True)`
compiles to a bare `DATETIME` and the offset is silently discarded. If you write
aware datetimes on Postgres and naive ones on SQLite, comparisons break when you
switch.

```python
from datetime import datetime, timezone

def utcnow() -> datetime:
    """Timezone-aware UTC now. Use this, never datetime.utcnow()."""
    return datetime.now(timezone.utc)
```

`datetime.utcnow()` is deprecated in Python 3.12+ and returns a *naive* datetime
that claims to be UTC — the worst of both worlds. Do not use it.

---

## SQLite Connection Pragmas

**SQLite disables foreign key enforcement by default.** Every
`ondelete='CASCADE'` in your models silently does nothing, and deleting a parent
leaves orphaned children behind. Demonstrated:

```
ondelete=CASCADE, PRAGMA OFF (SQLite default) -> orphaned rows left: 1
ondelete=CASCADE, PRAGMA ON                   -> orphaned rows left: 0
```

You will not notice this in development and you will notice it in production, on
PostgreSQL, where the constraint suddenly *is* enforced and deletes start
failing. Set the pragmas on every connection:

```python
# app/extensions.py
import sqlalchemy as sa
from sqlalchemy import event
from flask_sqlalchemy import SQLAlchemy

db = SQLAlchemy()


def init_sqlite_pragmas(app):
    """Apply required PRAGMAs to every new SQLite connection.

    No-op on PostgreSQL. Must be registered before the first connection.
    """
    if not app.config['IS_SQLITE']:
        return

    @event.listens_for(db.engine, 'connect')
    def _set_pragmas(dbapi_conn, _record):
        cur = dbapi_conn.cursor()
        cur.execute('PRAGMA foreign_keys=ON')      # enforce FK constraints
        cur.execute('PRAGMA journal_mode=WAL')     # readers don't block writers
        cur.execute('PRAGMA synchronous=NORMAL')   # safe with WAL, much faster
        cur.execute('PRAGMA busy_timeout=5000')    # wait 5s instead of failing
        cur.close()
```

| Pragma | Why |
|--------|-----|
| `foreign_keys=ON` | Off by default. Without it `ondelete` is a no-op. |
| `journal_mode=WAL` | Default rollback journal makes readers block writers. WAL is the single biggest concurrency win. |
| `synchronous=NORMAL` | Safe under WAL, large write speedup. |
| `busy_timeout=5000` | Without it, concurrent writes raise `database is locked` immediately. |

`journal_mode` is persistent (stored in the file); the others are per-connection.
Setting all four on connect is harmless and keeps behavior identical regardless
of how the file was created.

> **Note:** `PRAGMA foreign_keys` is a no-op inside a transaction. Setting it in
> the `connect` event guarantees it runs before any transaction begins.

---

## Migrations That Run on Both

SQLite cannot `ALTER COLUMN`, and older versions cannot `DROP COLUMN`. Alembic
works around this by copying the table — but only if you enable batch mode:

```python
# migrations/env.py
context.configure(
    connection=connection,
    target_metadata=target_metadata,
    render_as_batch=True,          # required for SQLite ALTER support
    compare_type=True,             # detect column type changes
    compare_server_default=True,
)
```

Then write migrations with `batch_alter_table`, which is a no-op wrapper on
PostgreSQL and a table-copy on SQLite:

```python
def upgrade():
    with op.batch_alter_table('user') as batch_op:
        batch_op.add_column(sa.Column('locale', sa.String(10), nullable=True))
        batch_op.alter_column('name', existing_type=sa.String(100), nullable=False)
```

**Always name your constraints.** SQLite's table-copy has to recreate them, and
it cannot recreate what it cannot name:

```python
# app/models/base.py
NAMING_CONVENTION = {
    'ix':  'ix_%(table_name)s_%(column_0_name)s',
    'uq':  'uq_%(table_name)s_%(column_0_name)s',
    'ck':  'ck_%(table_name)s_%(constraint_name)s',
    'fk':  'fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s',
    'pk':  'pk_%(table_name)s',
}

db = SQLAlchemy(metadata=sa.MetaData(naming_convention=NAMING_CONVENTION))
```

Set this **before your first migration**. Adding it later means Alembic wants to
rename every constraint in your schema.

---

## Portable Query Idioms

| Do | Not | Why |
|----|-----|-----|
| `col.ilike('a%')` | `func.lower(col) == 'a'` | `.ilike()` compiles to `ILIKE` on PG and `lower(x) LIKE lower(?)` on SQLite — portable for free. |
| `db.session.get(User, 1)` | `User.query.get(1)` | `Query.get()` is legacy-deprecated in SQLAlchemy 2.0. |
| `db.session.execute(sa.select(User))` | `User.query.filter(...)` | 2.0 style; `Query` is legacy. |
| `sa.text('SELECT 1')` | `'SELECT 1'` | Raw strings raise in 2.0. |
| `sa.func.now()` | `datetime.utcnow` | Server-side, consistent across engines. |
| `Numeric(12, 2)` | `Float` | Float loses money. |

Case-insensitive email is handled in application code — normalize to lowercase
on write — rather than with PostgreSQL `citext` or SQLite `COLLATE NOCASE`.
Neither has a counterpart on the other engine.

```python
# Portable: works identically on both
users = db.session.scalars(
    sa.select(User).where(User.name.ilike(f'%{term}%'))
).all()
```

---

## What Is Not Portable

Do not pretend these work everywhere. Put each behind a method on the model so
there is exactly one place to change.

| Feature | PostgreSQL | SQLite | Strategy |
|---------|-----------|--------|----------|
| Full-text search | `tsvector` + GIN | FTS5 virtual table | Wrap in `Model.search(term)`; branch on dialect |
| JSON *querying* | `->>`, `@>`, GIN index | `json_extract`, no index | Store portably, query only on Postgres |
| Arrays | `ARRAY` | none | Use `JSONColumn` |
| `ON CONFLICT` upsert | `ON CONFLICT DO UPDATE` | supported 3.24+ | Use the dialect-specific `insert()` |
| Partial indexes | yes | yes | Portable, but verify |
| Concurrent writers | thousands | one at a time | See below |
| `LISTEN`/`NOTIFY` | yes | none | Don't build on it if you may stay on SQLite |

```python
class Article(BaseModel):
    @classmethod
    def search(cls, term: str):
        """Full-text search. Falls back to LIKE on SQLite."""
        if current_app.config['IS_POSTGRES']:
            return db.session.scalars(sa.select(cls).where(
                cls.search_vector.op('@@')(sa.func.plainto_tsquery('english', term))
            )).all()
        return db.session.scalars(
            sa.select(cls).where(cls.body.ilike(f'%{term}%'))
        ).all()
```

### The real SQLite limit

SQLite allows **one writer at a time**, database-wide. WAL means readers never
block, so read-heavy apps scale fine. But if two requests write concurrently,
one waits (up to `busy_timeout`) and then fails.

Switch to PostgreSQL when you see `database is locked` under normal load, when
you need more than one application server, or when you want read replicas.
That is a `DATABASE_URL` change, not a rewrite — which is the entire point of
this document.

---

## Testing Against Both

Tests default to SQLite in-memory for speed. CI should run the suite against
PostgreSQL too, because that is where FK enforcement and type strictness differ.

```python
# tests/conftest.py
import os, pytest
from app import create_app
from app.config import TestConfig
from app.extensions import db


@pytest.fixture(scope='session')
def database_url():
    # CI sets TEST_DATABASE_URL to a Postgres DSN for the second matrix leg.
    return os.environ.get('TEST_DATABASE_URL', 'sqlite:///:memory:')


@pytest.fixture
def app(database_url):
    class Cfg(TestConfig):
        SQLALCHEMY_DATABASE_URI = database_url
        IS_SQLITE = database_url.startswith('sqlite')
        IS_POSTGRES = database_url.startswith('postgresql')

    # Config must be passed IN, not set after create_app(): Flask-SQLAlchemy
    # binds its engine during init_app, so a later assignment is ignored and
    # your tests silently run against the development database.
    app = create_app(Cfg)
    with app.app_context():
        db.create_all()
        yield app
        db.session.remove()
        db.drop_all()
```

```yaml
# .github/workflows/test.yml (excerpt)
strategy:
  matrix:
    include:
      - name: sqlite
        test_database_url: ''
      - name: postgres
        test_database_url: postgresql+psycopg://postgres:postgres@localhost:5432/test
```

If a test passes on SQLite and fails on PostgreSQL, the usual causes are: a
missing `ondelete` you relied on, an unnamed constraint, a naive datetime, or a
query that depends on SQLite's loose typing.

---

## Migrating SQLite → PostgreSQL

1. Provision the database and set `DATABASE_URL`.
2. Run migrations against the empty Postgres database: `flask db upgrade`.
   Your migration history is engine-agnostic, so the schema is built correctly.
3. Copy the data. For a small app, a script that reads every model and bulk
   inserts is simpler and safer than `pgloader`:

```python
# scripts/sqlite_to_postgres.py
"""One-shot data copy. Schema must already exist on the target."""
import sqlalchemy as sa
from app.models import ALL_MODELS   # ordered parents-before-children

src = sa.create_engine('sqlite:///app.db')
dst = sa.create_engine(os.environ['DATABASE_URL'])

with sa.orm.Session(src) as s, sa.orm.Session(dst) as d:
    for model in ALL_MODELS:
        rows = [
            {c.name: getattr(o, c.name) for c in model.__table__.columns}
            for o in s.scalars(sa.select(model))
        ]
        if rows:
            d.execute(sa.insert(model.__table__), rows)
        print(f'{model.__tablename__}: {len(rows)}')
    d.commit()
```

4. **Reset the sequences.** Bulk inserts with explicit ids leave PostgreSQL's
   sequences at 1, so the next insert collides. This step is missed constantly:

```python
for model in ALL_MODELS:
    t = model.__tablename__
    d.execute(sa.text(
        f"SELECT setval(pg_get_serial_sequence('\"{t}\"', 'id'), "
        f"COALESCE((SELECT MAX(id) FROM \"{t}\"), 1))"
    ))
d.commit()
```

5. Verify row counts per table, then cut over.

---

**Next:** [Database Patterns](database.md) | [Testing](testing.md) | [Deployment](deployment.md)
