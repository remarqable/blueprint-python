# Background Jobs

> **Layer doc.** Read this only when `jobs: true` in the Project
> Configuration.

A database-backed queue and a worker process. No Redis, no Celery, no broker —
the database you already operate is the queue, which matters most when other
people run your application (`deploy: docker`) and every extra service is a
support ticket.

---

## Table of Contents

- [When You Need This](#when-you-need-this)
- [The Job Model](#the-job-model)
- [Enqueueing](#enqueueing)
- [Transactional Enqueue](#transactional-enqueue)
- [The Worker](#the-worker)
- [Claiming Work](#claiming-work)
- [Retries and Failure](#retries-and-failure)
- [Idempotency](#idempotency)
- [Scheduled Jobs](#scheduled-jobs)
- [Queues and Priority](#queues-and-priority)
- [Graceful Shutdown](#graceful-shutdown)
- [Tenancy](#tenancy)
- [Tracing a Job Back to Its Request](#tracing-a-job-back-to-its-request)
- [Testing](#testing)
- [Operating It](#operating-it)
- [When to Graduate](#when-to-graduate)

Anything marked **added in 2.1.0** is optional for a project built on 2.0: add
it with one migration, or leave it out. Every default keeps the 2.0 behaviour.

---

## When You Need This

Anything that should not run inside a request: sending email (one message is
borderline, a newsletter batch is not), image variant generation, tenant purge
after the deletion grace period, webhook delivery, nightly cleanup.

The bar for adopting a broker instead (Celery/Redis) is high: you need it when
job *throughput* is the product. For "send the emails, resize the images" a
polling worker over a table is simpler, transactional with your data, and
survives restarts for free.

---

## The Job Model

```python
# app/models/job.py
class Job(BaseModel):
    __tablename__ = 'job'
    # Deliberately NOT OrgScoped: the worker has no request context and
    # crosses tenants by design. org_id is plain data here.

    name = db.Column(db.String(100), nullable=False, index=True)   # registry key
    payload = db.Column(db.JSON, nullable=False, default=dict)
    org_id = db.Column(BigIntFK, db.ForeignKey('organization.id', ondelete='CASCADE'),
                       nullable=True, index=True)

    status = db.Column(db.String(10), nullable=False, default='pending', index=True)
    # pending -> running -> done | failed (terminal after max attempts)
    run_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    attempts = db.Column(db.Integer, nullable=False, default=0)
    max_attempts = db.Column(db.Integer, nullable=False, default=3)
    last_error = db.Column(db.Text, nullable=True)
    locked_at = db.Column(db.DateTime(timezone=True), nullable=True)
    finished_at = db.Column(db.DateTime(timezone=True), nullable=True)

    __table_args__ = (
        db.Index('ix_job_claim', 'status', 'run_at'),
    )
```

Payloads are JSON of **ids, not objects** — the worker reloads rows fresh.
A payload carrying a serialized model is stale the moment it is written.

### Optional columns (added in 2.1.0)

Existing projects add these with one migration (`batch_op.add_column` with the
same `server_default`s, `batch_op.create_index` with the same `where`
arguments), or leave any out together with the parameter that uses it.

```python
# app/models/job.py — added in 2.1.0
LIVE_KEY = sa.text("unique_key IS NOT NULL AND status IN ('pending', 'running', 'done')")
    # inside class Job
    queue = db.Column(db.String(50), nullable=False, default='default',
                      server_default='default')          # Queues and Priority
    priority = db.Column(db.Integer, nullable=False, default=0,
                         server_default='0')             # higher runs first
    unique_key = db.Column(db.String(200), nullable=True)    # Idempotency
    trace_id = db.Column(db.String(64), nullable=True)       # Tracing
    # added to __table_args__. Partial: a failed job releases its key.
    db.Index('uq_job_unique_key', 'name', 'unique_key', unique=True,
             sqlite_where=LIVE_KEY, postgresql_where=LIVE_KEY),
```

---

## Enqueueing

Handlers are plain functions in a registry; enqueueing writes a row in the
caller's transaction, so a job for an object that failed to commit never runs:

```python
# app/platform/jobs.py
HANDLERS: dict[str, Callable] = {}


def job(name: str):
    def register(fn):
        HANDLERS[name] = fn
        return fn
    return register


def enqueue(name: str, *, org_id=None, run_at=None, **payload) -> Job:
    if name not in HANDLERS:
        raise ValueError(f'Unknown job: {name}')
    return Job(name=name, payload=payload, org_id=org_id,
               run_at=run_at or utcnow()).save()
```

```python
# app/models/newsletter.py
@job('newsletter.send_issue')
def send_issue(payload):
    issue = db.session.get(Issue, payload['issue_id'])
    ...

# controller
enqueue('newsletter.send_issue', org_id=g.org.id, issue_id=issue.id)
```

`enqueue` **commits**. Read the next section before calling it from anything
that writes more than one row.

---

## Transactional Enqueue

**Rule: a job is committed together with the write that caused it, or it is
not enqueued.**

`enqueue` ends in `BaseModel.save()`, which calls `db.session.commit()`. That
commit carries everything pending in the session, so the job and its cause are
atomic only when `enqueue` is the **last** statement of the operation. Measured:

| Caller does | Result |
|-------------|--------|
| `db.session.add(issue)`, then `enqueue(...)` last | One commit: both rows, or neither if the flush fails |
| `enqueue(...)`, then a later step raises, in or out of `transaction()` | **The job and every write before it stay committed** |
| `issue.save()`, then `enqueue(...)` | Two commits. A crash between them leaves an issue with no job |

With the 2.0 signature: add the cause without committing (`db.session.add`,
plus `db.session.flush()` when the payload needs its id) and call `enqueue`
last. It is the commit. Never call it inside `transaction()`.

**Added in 2.1.0:** `commit=False` adds and flushes the job and leaves the
commit to the caller, so a rollback removes the job with everything else and
`enqueue` is safe inside `transaction()`. The other new parameters belong to
[Queues and Priority](#queues-and-priority), [Idempotency](#idempotency) and
[Tracing](#tracing-a-job-back-to-its-request). The four names can no longer be
payload keys: grep existing calls first.

```python
# app/platform/jobs.py — 2.1.0 form; every 2.0 call behaves as before
def enqueue(name: str, *, org_id=None, run_at=None, queue='default', priority=0,
            unique_key=None, commit=True, **payload) -> Job:
    if name not in HANDLERS:
        raise ValueError(f'Unknown job: {name}')
    job = None
    if unique_key is not None:                    # already queued, running or done?
        job = db.session.scalar(sa.select(Job).where(
            Job.name == name, Job.unique_key == unique_key,
            Job.status.in_(('pending', 'running', 'done'))))
    if job is None:
        job = Job(name=name, payload=payload, org_id=org_id, run_at=run_at or utcnow(),
                  queue=queue, priority=priority, unique_key=unique_key,
                  trace_id=current_trace_id())    # see Tracing
        db.session.add(job)
    if commit:
        db.session.commit()
    else:
        db.session.flush()                        # the caller's commit decides
    return job
```

---

## The Worker

A second process running the same codebase — `flask jobs run`. In
`deploy: docker` it is the `worker` service (same image, different command);
on a VPS it is a second systemd unit.

```python
# app/controllers/jobs_cli.py
@bp.cli.command('run')
def run_worker():
    app = current_app._get_current_object()
    log.info('worker_started')
    while True:
        job = _claim_next()
        if job is None:
            time.sleep(app.config.get('JOBS_POLL_INTERVAL', 2))
            continue
        _execute(job)


def _claim_next() -> Job | None:
    """Atomically claim one due job."""
    now = utcnow()
    claimed = db.session.execute(
        sa.update(Job)
        .where(Job.id == sa.select(Job.id)
               .where(Job.status == 'pending', Job.run_at <= now)
               .order_by(Job.run_at)
               .limit(1)
               .scalar_subquery())
        .where(Job.status == 'pending')          # re-check: loses the race safely
        .values(status='running', locked_at=now,
                attempts=Job.attempts + 1)
        .returning(Job.id)
    ).scalar()
    db.session.commit()
    return db.session.get(Job, claimed) if claimed else None
```

Concurrency rules by engine, consistent with
[core/portability.md](core/portability.md):

- **SQLite:** run exactly **one** worker process. WAL lets it coexist with
  the web workers; a second claimer is where the pain starts.
- **PostgreSQL:** multiple workers are fine; add
  `.with_for_update(skip_locked=True)` to the inner select so claimers never
  queue behind each other.

Recover zombies at worker startup: any `running` job whose `locked_at` is
older than a generous timeout was orphaned by a crash — reset it to `pending`.

---

## Claiming Work

`_claim_next` is one `UPDATE` with a re-check of `status`. Tested on both
engines with six worker processes draining 2,000 jobs while another process
wrote continuously: every job ran exactly once.

**SQLite.** The statement is one write transaction and SQLite admits one
writer for the whole database, so two workers cannot claim the same row.
Several worker processes are safe, given the pragmas in
[core/portability.md](core/portability.md#sqlite-connection-pragmas). One is
still the default: handler writes queue behind that same lock, so more workers
only help handlers that mostly read or wait on the network. **A handler must
not hold a write open across slow work.** Its first flush takes the write lock
until the next commit; one that flushed and then slept made a web request's
write fail with `database is locked` after the 5 s `busy_timeout`.

**PostgreSQL.** Without `SKIP LOCKED` the claim is still correct, but a worker
that picked the same row as another waits, fails the re-check and gets `None`
while work is queued, then sleeps a poll interval: one worker ran 937 jobs and
another 51. With it the split was 331 to 335. SQLite compiles the clause to
nothing, so write it unconditionally. The 2.1.0 `_claim_next(queues=())` makes
two more changes to the inner select: `.where(Job.queue.in_(queues))` when
`queues` is not empty, and `.order_by(Job.priority.desc(), Job.run_at)`.

A killed worker leaves its job `running`; `locked_at` is the lease:

```python
# app/controllers/jobs_cli.py — added in 2.1.0
def _recover_zombies() -> int:
    """Return jobs orphaned by a dead worker to the queue."""
    cutoff = utcnow() - timedelta(seconds=current_app.config.get('JOBS_LOCK_TIMEOUT', 3600))
    spent = Job.attempts >= Job.max_attempts
    n = db.session.execute(
        sa.update(Job)
        .where(Job.status == 'running',
               sa.or_(Job.locked_at < cutoff, Job.locked_at.is_(None)))
        .values(status=sa.case((spent, 'failed'), else_='pending'), locked_at=None,
                last_error=sa.case((spent, 'worker died mid-job'), else_=Job.last_error))
        .execution_options(synchronize_session=False)).rowcount
    db.session.commit()
    return n
```

**`JOBS_LOCK_TIMEOUT` must exceed the slowest handler**, or a live job is
handed to a second worker and runs twice at once. The `IS NULL` test is for
the 2.0 loop: Ctrl-C mid-job makes `_execute`'s `finally` clear the lock and
commit while the status is still `running`, a row that a recovery comparing
only `locked_at` to a timeout never finds.

---

## Retries and Failure

```python
def _execute(job: Job) -> None:
    try:
        HANDLERS[job.name](job.payload)
        job.status, job.finished_at = 'done', utcnow()
    except Exception as e:
        db.session.rollback()
        log.error('job_failed', job=job.name, id=job.id, error=str(e))
        job.last_error = f'{type(e).__name__}: {e}'
        if job.attempts >= job.max_attempts:
            job.status = 'failed'                        # terminal — surfaced in admin
        else:
            job.status = 'pending'
            job.run_at = utcnow() + timedelta(seconds=30 * 4 ** (job.attempts - 1))
    finally:
        job.locked_at = None
        job.save()
```

Exponential backoff (30s, 2m, 8m), a bounded attempt count, and a terminal
`failed` state that an admin screen can list and retry. **Handlers must be
idempotent** — a crash after the work but before the status write means the
job runs again. "Send email to each recipient not yet marked sent" is
idempotent; "send email to everyone" is a double-send.

Clean up `done` jobs on a schedule; the table is a queue, not an archive.

**Added in 2.1.0: terminal errors.** A deleted row or a malformed payload
fails the same way every time. Raise `JobFatal(Exception)`, defined in
`app/platform/jobs.py`, and the job goes to `failed` on the first attempt. In
`_execute` the test becomes
`if isinstance(e, JobFatal) or job.attempts >= job.max_attempts:`.

A job whose handler was removed raises `KeyError` and reaches `failed` through
the normal retries. Ship a handler before enqueueing it; drain before deleting.

---

## Idempotency

Delivery is **at least once**. A handler runs again when the worker dies
before the status write, when `JOBS_LOCK_TIMEOUT` expires on a live job, and on
every retry: `_execute`'s rollback only undoes what the handler had not
committed. In order of preference:

1. **Natural.** A unique constraint on what the job produces
   (`delivery(issue_id, recipient_id)`), and skip rows that already exist.
2. **Guard on state.** `UPDATE ... WHERE sent_at IS NULL` and check `rowcount`.
3. **Record with the effect.** A database effect and its record go in one
   commit. An external effect (email, a charge) cannot share a transaction:
   pass the provider an idempotency key derived from the job, and commit
   progress per item so a retry resumes instead of restarting.

**Added in 2.1.0: `unique_key`** stops the same work being *queued* twice.
`enqueue` returns the existing job when one with the same `name` and
`unique_key` is pending, running or done; the partial unique index turns a
race between two callers into an `IntegrityError` for the loser. A `failed`
job releases its key, a `done` job holds it until cleanup deletes the row.

---

## Scheduled Jobs

Recurring work is a job that re-enqueues itself for the next slot at the top
of its handler (re-enqueue *first*, so a failure cannot kill the schedule):

```python
@job('system.daily_cleanup')
def daily_cleanup(payload):
    enqueue('system.daily_cleanup', run_at=utcnow() + timedelta(days=1))
    MagicLink.cleanup_expired()
    purge_deleted_tenants()
```

Seed each recurring job once (idempotently) at worker startup. This replaces
cron for anything that needs the application context; keep OS cron for things
outside it (database file backups).

**Added in 2.1.0: key the successor.** Re-enqueueing first has a cost: every
retry of a failing run enqueues another successor. A `daily_cleanup` that
failed its three attempts left three jobs for tomorrow. Keyed on the slot, the
same run leaves one, and the startup seed becomes idempotent too:

```python
    tomorrow = (utcnow() + timedelta(days=1)).replace(hour=3, minute=0,
                                                      second=0, microsecond=0)
    enqueue('system.daily_cleanup', run_at=tomorrow,
            unique_key=f'daily_cleanup:{tomorrow:%Y-%m-%d}')
```

---

## Queues and Priority

**Added in 2.1.0.** A queue exists so that one slow class of work cannot
starve another: ten thousand queued image jobs must not delay a password-reset
email. Separate by latency expectation, not by feature; `default`, `mail` and
`bulk` are usually enough: `enqueue('auth.send_magic_link', queue='mail', ...)`.

`flask jobs run` works every queue, as in 2.0; `--queue mail` (repeatable)
works only that one. Run one worker process per class that needs its own lane.
`priority` orders jobs within what a worker claims: higher first, then
`run_at`. A steady supply of high-priority jobs starves the rest, which a
separate queue does not.

---

## Graceful Shutdown

Every deploy restarts the worker, and systemd stops a unit with SIGTERM. The
2.0 loop does not handle it. Measured with `flask jobs run` and a 3-second job:

| Signal mid-job | 2.0 loop | 2.1.0 loop |
|----------------|----------|------------|
| SIGTERM | Killed at once; job `running` until recovery | Finishes the job, marks it `done`, exits 0 |
| SIGINT (Ctrl-C) | Job `running` with `locked_at` NULL | The same as SIGTERM |
| SIGKILL | Job `running` until recovery | The same; nothing can catch it |

```python
# app/controllers/jobs_cli.py — 2.1.0 form of `flask jobs run`
_stop = threading.Event()


@bp.cli.command('run')
@click.option('--queue', 'queues', multiple=True, help='Repeatable. Default: all.')
def run_worker(queues):
    app = current_app._get_current_object()
    for sig in (signal.SIGTERM, signal.SIGINT):
        signal.signal(sig, lambda *_: _stop.set())      # only sets a flag
    log.info('worker_started', queues=queues or 'all')
    next_recovery = 0.0
    while not _stop.is_set():                           # finish the job, then exit
        if time.monotonic() >= next_recovery:           # at startup, then each minute
            _recover_zombies()
            next_recovery = time.monotonic() + 60
        job = _claim_next(queues)
        if job is None:
            _stop.wait(app.config.get('JOBS_POLL_INTERVAL', 2))   # wakes on signal
            continue
        _execute(job)
    log.info('worker_stopped')
```

A signal handler that raised instead would unwind through `_execute`'s
`finally` and commit a half-finished job.

Under systemd the worker is a copy of the web unit in
[core/deployment.md](core/deployment.md#2-systemd-service), saved as
`yourapp-worker.service`, with `After=network.target yourapp.service`,
`ExecStart=/opt/yourapp/.venv/bin/flask jobs run`, no `ExecStartPre` (the web
unit owns migrations) and `TimeoutStopSec=120`. **`TimeoutStopSec` must exceed
the slowest handler:** when it runs out systemd sends SIGKILL and the job waits
for `JOBS_LOCK_TIMEOUT`. With `deploy: docker` the same setting is
`stop_grace_period` on the `worker` service (default 10 s).

---

## Tenancy

With `tenancy: shared`, the worker runs outside any request context, so the
automatic tenant filter deliberately does not apply (see
[tenancy.md § Safe-by-Default Scoping](tenancy.md#safe-by-default-scoping)) —
reads cross tenants, which is what a worker needs. The discipline that
replaces the filter:

- A job that acts **for one tenant** carries `org_id` and its handler filters
  by it explicitly, first thing.
- A job that acts **across tenants** (purge, digests) iterates organizations
  explicitly, the same shape as the `unscoped()` billing example in
  tenancy.md.
- Writes still stamp `org_id` explicitly — the `before_flush` guard only
  auto-stamps inside a request.

With `tenant_isolation: rls` the database enforces this as well: a handler that
has not entered the job's organization reads zero rows. See
[tenancy.md § Row-Level Security](tenancy.md#row-level-security-opt-in).

With `audit_logging: true`, a job can credit the user who caused it. See
[core/audit.md § Jobs and System Actors](core/audit.md#jobs-and-system-actors).

---

## Tracing a Job Back to Its Request

**Added in 2.1.0.** "My newsletter never arrived" is one query when the job
row carries the id of the request that enqueued it. `trace_id` is an optional
nullable column, and the code that fills it lives in
[core/observability.md](core/observability.md#correlation): `enqueue` stores
`current_trace_id()` from `app/platform/obs.py`, and `_execute` binds the
job's `trace_id` to the log context first thing, so every line the handler
writes carries it and a job enqueued by a job keeps the original id. A job
enqueued outside a request stores `NULL`.

Find a request's jobs with `Job.query.filter_by(trace_id=request_id)`. A
project that leaves the column out drops the `trace_id=` argument.

---

## Testing

Don't poll in tests — run synchronously:

```python
def run_jobs():
    """Execute all due jobs inline. Test helper."""
    while (job := _claim_next()) is not None:
        _execute(job)


def test_publishing_sends_newsletter(client, org, issue):
    client.post(f'/issues/{issue.id}/publish')
    assert Job.query.filter_by(name='newsletter.send_issue').count() == 1
    run_jobs()
    assert issue.deliveries.count() == issue.audience.count()
```

And one test that proves a failing handler retries with backoff and lands on
`failed` after `max_attempts`.

Queue-level tests worth writing once, not per handler (the last three need the
2.1.0 additions):

- A caller that fails before `enqueue` leaves no job; a handler run twice with
  the same payload produces one effect.
- `enqueue(commit=False)` inside a `transaction()` that raises leaves no job.
- The same `unique_key` enqueues once; a `failed` job releases it.
- `_recover_zombies()` requeues an expired lock and leaves a fresh one alone.

---

## Operating It

**Added in 2.1.0.** Add a `flask jobs status` command that prints the count
and the oldest `run_at` per queue and status (one `GROUP BY` over `job`), and
read it this way:

| Look at | It means |
|---------|----------|
| Oldest `pending` `run_at` far in the past | The worker is down or behind. This is the honest signal; depth alone is not, a burst is normal |
| `pending` count growing for 15 minutes | Work arrives faster than it is done: add a worker or split a queue |
| Any new `failed` row | A bug report. Read `last_error` and fix the cause |
| `running` with an old `locked_at` | A worker died mid-job; recovery requeues it after `JOBS_LOCK_TIMEOUT` |
| The same job recovered repeatedly | The handler kills its worker (memory) or outlives the timeout |

Alert on the first and third rows. To retry a `failed` job, set
`status='pending'`, `attempts=0` and `run_at=utcnow()` on it. Delete `done`
rows older than a week from the daily cleanup job; never delete `failed` rows
automatically.

---

## When to Graduate

Stay on the database queue until one of these is true:

- **SQLite:** web requests see `database is locked` because handlers write so
  often. Move to PostgreSQL first; the queue code does not change.
- **PostgreSQL:** sustained thousands of jobs per second, with the claim
  statement near the top of `pg_stat_statements`.
- Many independent consumers need the same event. That is a log, not a queue.

Growth in row count is not a reason: delete `done` rows. A broker also gives
up [transactional enqueue](#transactional-enqueue) unless you add an outbox.

---

**Next:** [Storage](storage.md) | [Deployment](core/deployment.md) | [Tenancy](tenancy.md)
