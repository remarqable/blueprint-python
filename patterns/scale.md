# Scale

> **Layer doc.** It has no setting. Read this only when one table will hold
> hundreds of millions of rows, or when `SELECT COUNT(*)` on it has already
> stopped being instant.

The handful of decisions that are cheap now and expensive once the table is
large: shapes that must be right early, because changing them later means a
migration against your biggest table while it is serving traffic.

Cursor pagination, retention, index discipline and counters work on SQLite and
PostgreSQL alike. **Partitioning, connection pooling and read replicas are
PostgreSQL-only.** A project that needs one of them has outgrown SQLite: move
first ([core/portability.md](core/portability.md#migrating-sqlite--postgresql)).

---

## Table of Contents

- [The Dominant Table](#the-dominant-table)
- [Cursor Pagination](#cursor-pagination)
- [Partitioning](#partitioning)
- [Retention](#retention)
- [Connection Pooling](#connection-pooling)
- [N+1](#n1)
- [Index Discipline](#index-discipline)
- [Counters](#counters)
- [Caching](#caching)
- [Read Replicas](#read-replicas)
- [What Not to Do Yet](#what-not-to-do-yet)

---

## The Dominant Table

Most applications have one table that ends up a hundred times larger than every
other: `message` for a chat product, `sample` for a metrics product. Identify it
on day one. It is the only table where these patterns are mandatory. Every
other table keeps the core patterns, `paginate()` included
([core/database.md](core/database.md#pagination)); applying this doc everywhere
is its own premature optimisation.

---

## Cursor Pagination

`OFFSET` reads and discards every row it skips: at offset 100,000 the database
does 100,000 rows of work to return 50. It is wrong as well as slow. Rows
inserted while a user pages shift items between pages, so some are shown twice.

Page by the last key you saw. The key must be **unique and ordered**.
`created_at` alone is not: two rows can share a timestamp, and the boundary row
is then skipped or repeated. Use `id`, or `(created_at, id)` for a list ordered
by time:

```python
# app/models/message.py
class Message(OrgScoped, BaseModel):
    __tablename__ = 'message'
    __table_args__ = (
        db.Index('ix_message_org_channel_created', 'org_id', 'channel_id', 'created_at', 'id'),
    )

    @classmethod
    def page(cls, channel_id: int, cursor: str | None = None,
             limit: int = 50) -> tuple[list['Message'], str | None]:
        """One page, newest first, and the cursor for the next (None at the end)."""
        stmt = sa.select(cls).where(cls.channel_id == channel_id)
        if cursor:
            stmt = stmt.where(
                sa.tuple_(cls.created_at, cls.id) < sa.tuple_(*decode_cursor(cursor)))
        rows = db.session.scalars(
            stmt.order_by(cls.created_at.desc(), cls.id.desc()).limit(limit + 1)).all()
        if len(rows) <= limit:
            return list(rows), None
        last = rows[limit - 1]
        return list(rows[:limit]), encode_cursor(last.created_at, last.id)
```

The row comparison walks the index on both engines, with no sort, at any depth.
The index holds what the query uses, in order: the tenant filter's `org_id`,
the channel, then the sort key. The controller passes `request.args.get('cursor')`.

Encode cursors opaquely, so clients cannot build them and the shape can
change. In `app/platform/cursor.py`, `encode_cursor` is URL-safe base64 of the
JSON `[created_at.isoformat(), id]`, and `decode_cursor` reverses it
(`datetime.fromisoformat`, `int`), raising `ValidationError` on a `ValueError`
or `TypeError`. SQLite hands timestamps back naive: set `tzinfo=timezone.utc`
before encoding, so a cursor has one shape on both engines.

**Do not return a total count with a paginated list.** `COUNT(*)` on a large
table is a scan. `page()` asks for one row more than the page, which is enough
to say "more available"; a number comes from a [counter](#counters), or on
PostgreSQL from the estimate in `pg_class.reltuples`.

---

## Partitioning

**PostgreSQL only.** Partition the dominant table by time once it is large. Two
benefits, one of which is the real one:

- Queries with a time predicate touch fewer partitions.
- **Deleting old data becomes `DROP TABLE` instead of `DELETE`.** A `DELETE` of
  200 million rows leaves 200 million dead tuples, hours of vacuum, table bloat
  and replication lag. Dropping a partition is instant and frees the disk.

```python
# app/models/message.py
class Message(OrgScoped, BaseModel):
    __tablename__ = 'message'
    __table_args__ = (
        db.Index('ix_message_org_channel_created', 'org_id', 'channel_id', 'created_at', 'id'),
        {'postgresql_partition_by': 'RANGE (created_at)'},
    )
    # The partition key must be in the primary key, which becomes (id, created_at)
    created_at = db.Column(db.DateTime(timezone=True), primary_key=True, default=utcnow)

# migrations/versions/..., one per partition
op.execute("""CREATE TABLE message_2026_11 PARTITION OF message
              FOR VALUES FROM ('2026-11-01') TO ('2026-12-01')""")
```

Three consequences to accept up front:

- **The partition key joins every unique constraint.** `PRIMARY KEY (id)` is not
  possible, nor any `UNIQUE` without `created_at`. This is the reason to decide
  early: changing a primary key later is not a small migration.
- **Queries without the partition key scan every partition.** `get_by_id(id)`
  no longer works on this model; fetch with
  `db.session.get(Message, {'id': id, 'created_at': created_at})`, and carry the
  timestamp in the identifier the client holds.
- **Partitions must be created ahead of time.** A row with no matching partition
  is an insert error. Keep three months ahead from a scheduled job, and alert
  if it has not run.

Partition by month for chat-like volume, by day for metrics-like volume. Aim for
partitions in the low tens of gigabytes; too many small ones slows planning.

---

## Retention

Decide what you keep before you are storing it. Retention is a product decision
with a schema consequence, and adding it later makes the first deletion a
migration instead of a routine.

| Data | Typical | Mechanism |
|---|---|---|
| The dominant table | Forever, or per plan | Batched delete; drop partitions once partitioned |
| Derived and summary rows | Rebuildable: keep while useful | Delete freely |
| Job rows (`jobs: true`) | 7–30 days after completion | Batched delete |
| Magic links, invitations | On expiry | The daily cleanup |
| Logs | 14–30 days | The log pipeline, not the database |

Until the table is partitioned, delete in batches, a transaction each. One huge
`DELETE` holds SQLite's single writer for its whole run
([core/portability.md](core/portability.md#the-real-sqlite-limit)), and holds
locks and bloats the table on PostgreSQL.

```python
# app/models/message.py
@classmethod
def purge_older_than(cls, cutoff: datetime, batch: int = 5000) -> int:
    """Delete rows created before cutoff, a batch per transaction."""
    total = 0
    while True:
        ids = (sa.select(cls.id).where(cls.created_at < cutoff)
               .order_by(cls.created_at).limit(batch))
        deleted = db.session.execute(
            sa.delete(cls).where(cls.id.in_(ids)),
            execution_options={'synchronize_session': False}).rowcount
        db.session.commit()
        total += deleted
        if deleted < batch:
            return total
```

It needs an index on `created_at` alone: the composite one leads with `org_id`
and cannot serve a purge across tenants. Run it outside a request (a scheduled
job with `jobs: true`, a CLI command from cron otherwise). On SQLite the freed
pages are reused, but the file shrinks only on `VACUUM`, which rewrites it.

---

## Connection Pooling

**PostgreSQL only.** PostgreSQL allocates several megabytes per connection and
degrades past a few hundred; `max_connections` defaults to 100.

The pool in [Configuration](../CLAUDE.md#configuration) is **per process**:
`pool_size: 5` plus `max_overflow: 10` is up to 15 connections for each Gunicorn
worker and each job worker, so four web workers and a job worker can open 75.
Budget across all processes: `processes × (pool_size + max_overflow)` stays
under `max_connections`, with room for migrations and `psql`.

When the total passes about 200, put PgBouncer in **transaction** mode in front
and point `DATABASE_URL` at it. A connection then goes back to the pool at the
end of every transaction, so nothing session-scoped survives:

- **Tenant scoping is safe by default**: it lives in the ORM
  ([tenancy.md](tenancy.md#safe-by-default-scoping)), not in the connection.
  With `tenant_isolation: rls`, set the tenant inside the transaction that uses
  it, with `set_config(..., true)`. A session-level `SET` stays on the
  connection and applies to the next tenant's request: a data leak.
- **Prepared statements belong to one connection.** psycopg prepares a
  statement on the server after five uses. Unless your PgBouncer supports
  prepared statements, turn that off: add
  `'connect_args': {'prepare_threshold': None}` to `SQLALCHEMY_ENGINE_OPTIONS`.
- **`LISTEN`/`NOTIFY`, `SET` and session advisory locks need a direct
  connection.** Keep two URLs: the pooled one for requests, a direct one for a
  listener and for `flask db upgrade`.

---

## N+1

A query per row is a bug at any size, so the rule, the test that catches it and
the fixes are in [core/performance.md](core/performance.md#the-rule). What
changes at scale is the batch query itself: a page of ids is fine as
`id.in_(ids)`, but ids collected from a large result become thousands of bind
parameters (PostgreSQL refuses more than 65,535). When they come from another
query, pass the query and let the database join:

```python
# app/models/message.py
@classmethod
def authors_since(cls, channel_id: int, since: datetime) -> list[User]:
    recent = sa.select(cls.user_id).where(cls.channel_id == channel_id,
                                          cls.created_at > since)
    return db.session.scalars(sa.select(User).where(User.id.in_(recent))).all()
```

---

## Index Discipline

Declaring an index is in [core/database.md](core/database.md#indexes). On the
dominant table:

- **`org_id` leads every composite index** on an `OrgScoped` table. The tenant
  filter puts it in every query.
- **Index for the query, not the column.** `(org_id, channel_id, created_at, id)`
  serves the paginated read; separate indexes on each column do not.
- **Partial indexes for skewed predicates.** `WHERE status = 'pending'` on a
  table that is 99% done keeps the index small. Pass the same `sa.text(...)` to
  `db.Index` as both `sqlite_where=` and `postgresql_where=`.
- **Covering indexes for hot reads** (PostgreSQL: `postgresql_include=['body']`)
  avoid the heap fetch. They cost write throughput and disk; measure first.
- **Every index costs every write.** Six indexes means six B-tree updates per
  insert.

Check that a query uses the index built for it: `EXPLAIN QUERY PLAN` on SQLite,
`EXPLAIN` on PostgreSQL. PostgreSQL also reports the unused indexes, which are
pure cost, and the expensive queries (with the `pg_stat_statements` extension),
nearly always a different list from the one you would guess:

```sql
SELECT relname, indexrelname, idx_scan, pg_size_pretty(pg_relation_size(indexrelid))
  FROM pg_stat_user_indexes
 WHERE idx_scan < 50
 ORDER BY pg_relation_size(indexrelid) DESC;

SELECT calls, round(mean_exec_time::numeric, 2) AS avg_ms,
       round(total_exec_time::numeric / 1000, 1) AS total_s, query
  FROM pg_stat_statements
 ORDER BY total_exec_time DESC LIMIT 20;
```

Order by **total** time, not mean. A 3 ms query run 40,000 times a minute costs
more than a 900 ms report run hourly.

---

## Counters

Counting a channel's messages is a scan. For a sidebar of 50 channels, grouping
in SQL ([core/performance.md](core/performance.md#count-and-group-in-sql)) makes
it one query, but one that still reads every row it counts.

Keep a counter row, updated in the same transaction as the insert:

```python
# app/models/channel_stat.py
from sqlalchemy.dialects.postgresql import insert as pg_insert
from sqlalchemy.dialects.sqlite import insert as sqlite_insert


class ChannelStat(OrgScoped, BaseModel):
    __tablename__ = 'channel_stat'

    channel_id = db.Column(BigIntFK, db.ForeignKey('channel.id', ondelete='CASCADE'),
                           nullable=False, unique=True)
    message_count = db.Column(db.BigInteger, nullable=False, default=0)

    @classmethod
    def bump(cls, org_id: int, channel_id: int) -> None:
        """Add one, creating the row on first use. The caller commits."""
        insert = pg_insert if db.engine.dialect.name == 'postgresql' else sqlite_insert
        stmt = insert(cls).values(org_id=org_id, channel_id=channel_id, message_count=1)
        db.session.execute(stmt.on_conflict_do_update(
            index_elements=[cls.channel_id],
            set_={'message_count': cls.message_count + 1, 'updated_at': utcnow()}))

# app/models/message.py, in Message.post(channel, user_id, body)
db.session.add(message)
ChannelStat.bump(channel.org_id, channel.id)
db.session.commit()                         # the row and its count, or neither
```

The upsert is a Core statement, so it passes `org_id` itself: the ORM hook that
stamps it does not run. It serialises writes per channel, which is correct for
chat and wrong for a global counter every write touches. There, keep several
rows and sum them, or accept an approximation refreshed on a schedule.

Unread counts: store, per user and channel, the `message_count` at the moment
the user last looked, and derive unread as `message_count - seen_count`. One row
per user per channel, no per-message state. It is exact while the counter only
goes up, so do not decrement it when messages are deleted or purged.

---

## Caching

The order of value, the per-request and per-process patterns, and the rules for
any cache are in [core/performance.md](core/performance.md#caching-data). The
order does not change at scale: remove the N+1, add the index, then cache. On
PostgreSQL, size `shared_buffers` for the working set before adding a cache of
your own. A shared cache (Redis) is for state several processes must agree on,
such as rate limits; its keys include `org_id`.

---

## Read Replicas

**PostgreSQL only**, and later than you think. One instance on modern hardware
handles a great deal, and replication adds a correctness problem: **replica lag
breaks read-your-own-writes.** A user posts a message, the next request reads a
replica 200 ms behind, and the message is not there.

When you do add one, route deliberately: reports, exports, analytics and search
backfills to the replica; anything a user might have just written to the
primary. Never route by "is this a SELECT". The replica is a second bind, named
only by the model methods that are safe on stale data:

```python
# app/config.py
SQLALCHEMY_BINDS = {'replica': os.environ.get('REPLICA_DATABASE_URL', DATABASE_URL)}

# app/models/message.py, in a report method
rows = db.session.execute(stmt, bind_arguments={'bind': db.engines['replica']})
```

Before a replica, exhaust indexes, N+1, caching, and jobs that write summary
tables.

---

## What Not to Do Yet

Each of these solves a problem you probably do not have, and creates several you
definitely will:

- **Sharding.** Partition first, replicate second, shard only when one machine
  cannot hold the data.
- **Microservices.** A web process and a job worker are separate for their
  resource profiles, not as service boundaries. They share a database and a
  codebase on purpose.
- **A separate search cluster.** PostgreSQL full-text search
  ([core/database.md](core/database.md#full-text-search)) covers a great deal.
- **A message broker.** The job table ([jobs.md](jobs.md), with `jobs: true`) is
  enough well past the point where the product's other problems dominate.
- **Multi-region.** Doubles every operational problem and makes consistency a
  product concern.

The signal to act on is a measurement: a page in the admin Info box
([core/deployment.md](core/deployment.md#request-profiling-system-info)), a
query in `pg_stat_statements`, a queue whose oldest job keeps ageing. Not a
projection, and not an architecture diagram.
