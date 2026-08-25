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
- [The Worker](#the-worker)
- [Retries and Failure](#retries-and-failure)
- [Scheduled Jobs](#scheduled-jobs)
- [Tenancy](#tenancy)
- [Testing](#testing)

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

---

**Next:** [Storage](storage.md) | [Deployment](core/deployment.md) | [Tenancy](tenancy.md)
