# Observability

> One question to answer: when a user says it is broken, can you find out what
> happened without guessing?

Three signals, in the order you should build them: **structured logs** with a
correlation ID, **metrics** on the few things users feel, **traces** when the
work spans processes and the first two stop being enough.

Correlation, logging and [Never Log](#never-log) apply to every project.
Metrics need a dependency and a scraper: that section is for when you add them.

## Table of Contents

- [Correlation](#correlation)
- [Logging](#logging)
- [Metrics](#metrics)
- [Cardinality](#cardinality)
- [Cost as a Signal](#cost-as-a-signal)
- [Tracing](#tracing)
- [Health Checks](#health-checks)
- [Alerting](#alerting)
- [Never Log](#never-log)
- [Checklist](#checklist)

---

## Correlation

Work that begins as an HTTP request continues in a job, which may call a model,
which writes rows. **One identifier follows all of it.** Without that, a user
complaint is unanswerable: several processes write logs, with no way to line
them up.

[Request profiling](deployment.md#request-profiling-system-info) already makes
the identifier (`g.request_id`, echoed as `X-Request-Id`), but only lines that
pass it by hand carry it. Bind it once per request and every line does:

```python
# app/platform/obs.py -- register_observability(app), called from create_app
# after init_tenant and register_profiling
import uuid
import structlog
from flask import g, request
from flask_login import current_user
from structlog.contextvars import (bind_contextvars, clear_contextvars,
                                   get_contextvars, merge_contextvars)
from app.platform.logger import get_logger

log = get_logger()


def new_trace_id() -> str:
    return str(uuid.uuid4())


def current_trace_id() -> str | None:
    """The trace this code runs under: a request's, or the job's that restored it."""
    return get_contextvars().get('trace_id')


def register_observability(app):
    processors = structlog.get_config()['processors']
    if merge_contextvars not in processors:          # first, so every line carries the IDs
        structlog.configure(processors=[merge_contextvars, *processors])

    @app.before_request
    def _bind():
        clear_contextvars()                          # a worker thread serves many requests
        if not g.get('request_id'):                  # set by request profiling
            g.request_id = request.headers.get('X-Request-Id') or new_trace_id()
        rid = g.request_id[:64]                      # a client can send any length
        bind_contextvars(request_id=rid, trace_id=rid)
        if g.get('org') is not None:                 # tenancy: shared
            bind_contextvars(org_id=g.org.id)
        if request.endpoint != 'static' and current_user.is_authenticated:
            bind_contextvars(user_id=current_user.id)

    @app.after_request
    def _finish(response):
        response.headers.setdefault('X-Request-Id', g.get('request_id', ''))
        if response.status_code >= 500:              # the one line for a failed request
            log.error('request_failed', method=request.method, status=response.status_code,
                      rule=request.url_rule.rule if request.url_rule else None)
        return response
```

`request_id` is one per inbound request; `trace_id` survives across processes
and equals `request_id` where the work began. `init_logger` stays as it is, and
a field passed by hand wins, so the existing `slow_request` line is unchanged.

**When `jobs: true`**, persist the trace on the job row and restore it in the
worker. Three additions to [jobs.md](../jobs.md#the-job-model):

```python
# app/models/job.py -- one nullable column (flask db migrate -m "job trace id")
    trace_id = db.Column(db.String(64), nullable=True)   # the request that enqueued it

# app/platform/jobs.py -- enqueue() stores it
    return Job(name=name, payload=payload, org_id=org_id,
               run_at=run_at or utcnow(), trace_id=current_trace_id()).save()

# app/controllers/jobs_cli.py -- _execute() restores it, first thing
    clear_contextvars()
    bind_contextvars(trace_id=job.trace_id or new_trace_id(), job=job.name,
                     job_id=job.id, org_id=job.org_id)
```

That column is the single highest-value line in this document. It turns "a user
in organization 40 says their newsletter never went out" from an afternoon into
one `grep`. When `ai: true`, [`ai_call`](../ai.md#cost-accounting) carries the
same `trace_id`.

- The slice to 64 matters on PostgreSQL, which rejects a longer value. SQLite
  ignores the length and would store it.
- A job enqueued outside a request stores `NULL` and gets a fresh trace when it
  runs. A [recurring job](../jobs.md#scheduled-jobs) re-enqueues itself and
  would carry one trace forever: wrap that enqueue in
  `with bound_contextvars(trace_id=None):`.

Return the request ID in JSON errors (`'request_id': g.get('request_id')`) and
show it in `errors/500.html` (`{{ g.get('request_id') }}`). Users paste it into
support tickets and you skip the reconstruction entirely.

---

## Logging

Structured, JSON in production, one event per line: `get_logger()` from
[CLAUDE.md § Logging](../../CLAUDE.md#logging).

```python
# app/models/message.py -- request_id, trace_id, org_id, user_id are added by the binding
log.info('message_created', message_id=message.id, channel_id=message.channel_id,
         ms=round((time.monotonic() - started) * 1000))
```

Conventions that make logs queryable, not just voluminous:

- **The event is a stable snake_case name**, not prose: `user_logged_in`,
  `job_failed`, `slow_request`. You will group by it.
- **The fields are the data.** Values go in keyword arguments, never an f-string.
- **One log line per unit of work**, at completion, carrying duration and
  outcome. Not one at the start and one at the end. When `jobs: true`, `_execute`
  logs `job_done` with `ms` on success; a failure already logs `job_failed`.
- **Log the decision, not the narration.** `skipped_reason='too_short'` answers
  a question. "Processing message..." answers none.
- **Errors log once, where they are handled.** An error logged at every level
  of the stack produces five lines for one failure and makes the error rate a
  lie. An unhandled exception gets the one `request_failed` line above, beside
  Flask's traceback.

Levels: `error` is something a human must look at. `warning` is degraded but
handled: a retry, a fallback, a slow request. `info` is the business event
stream. `debug` is off in production.

---

## Metrics

Not installed by default: until something scrapes the app, the `slow_request`,
`request_failed` and `job_failed` lines and the
[access log](deployment.md#server-monitoring) answer the same questions after
the fact. To add metrics, `uv add prometheus-client`, then track the RED signals
(rate, errors, duration) for HTTP and the few domain numbers that matter.

**Gunicorn runs several worker processes, and each has its own counters.** A
plain `/metrics` answers from whichever worker took the scrape: with four
workers and 40 requests it reported 17. In `prometheus_client`'s multiprocess
mode each worker writes to files in one directory and the endpoint adds them up.

```python
# gunicorn.conf.py -- in the project root; Gunicorn loads it from the working directory
import os, shutil

os.environ.setdefault('PROMETHEUS_MULTIPROC_DIR', '/tmp/yourapp-prometheus')


def on_starting(server):        # once, in the master: last run's files would be added in
    path = os.environ['PROMETHEUS_MULTIPROC_DIR']
    shutil.rmtree(path, ignore_errors=True)
    os.makedirs(path)
```

```python
# app/platform/prom.py -- register_metrics(app), called from create_app after register_profiling
import hmac, os, time
from flask import Response, abort, g, request
from prometheus_client import (CONTENT_TYPE_LATEST, REGISTRY, CollectorRegistry, Counter,
                               Histogram, generate_latest, multiprocess)
from prometheus_client.core import GaugeMetricFamily

HTTP_REQUESTS = Counter('http_requests_total', 'Requests served', ['route', 'method', 'status'])
HTTP_DURATION = Histogram('http_request_duration_seconds', 'Time to answer', ['route', 'method'])


class QueueCollector:                                 # jobs: true only
    def collect(self):
        from app.models.job import Job
        pending, oldest, failed = Job.queue_stats()
        yield GaugeMetricFamily('jobs_pending', 'Due jobs waiting', value=pending)
        yield GaugeMetricFamily('jobs_oldest_pending_seconds', 'Age of the oldest due job', value=oldest)
        yield GaugeMetricFamily('jobs_failed', 'Jobs that used every attempt', value=failed)


QUEUE = CollectorRegistry()
QUEUE.register(QueueCollector())


def register_metrics(app):
    @app.after_request
    def _record(response):
        route = request.url_rule.rule if request.url_rule else 'unmatched'   # the pattern, never the path
        HTTP_REQUESTS.labels(route, request.method, response.status_code).inc()
        if g.get('request_start'):
            HTTP_DURATION.labels(route, request.method).observe(time.monotonic() - g.request_start)
        return response

    @app.route('/metrics')
    def metrics():
        token = app.config.get('METRICS_TOKEN') or ''     # Config: os.environ.get('METRICS_TOKEN')
        sent = request.headers.get('Authorization', '')
        if not token or not hmac.compare_digest(sent.encode(), f'Bearer {token}'.encode()):
            abort(404)                                    # no token configured: no endpoint
        if 'PROMETHEUS_MULTIPROC_DIR' in os.environ:      # Gunicorn: add up every worker's files
            registry = CollectorRegistry()
            multiprocess.MultiProcessCollector(registry)
        else:                                             # make run, tests: one process
            registry = REGISTRY
        return Response(generate_latest(registry) + generate_latest(QUEUE),
                        content_type=CONTENT_TYPE_LATEST)
```

- The variable must be set **before `prometheus_client` is imported**: hence
  `gunicorn.conf.py`, not `create_app`.
- A `Gauge` you add needs a `multiprocess_mode` (`livesum` for "how many right
  now") and a `child_exit` hook calling
  `multiprocess.mark_process_dead(worker.pid)`. Process metrics (CPU, memory)
  are not exported in this mode: atop has them.
- `app/platform/metrics.py` and `/admin/api/v1/metrics` are the
  [admin API](../admin-api.md)'s product numbers: another file, another endpoint.

**The jobs worker exports nothing**: it is not a Gunicorn worker and serves no
HTTP. What happens in jobs is read from the table at scrape time, by whichever
web worker answers, and is still right when the worker is dead.

```python
# app/models/job.py -- jobs: true
    @classmethod
    def queue_stats(cls) -> tuple[int, float, int]:
        """(due jobs waiting, age of the oldest in seconds, failed jobs)."""
        now = utcnow()
        pending, oldest = db.session.execute(
            sa.select(sa.func.count(), sa.func.min(cls.run_at))
            .where(cls.status == 'pending', cls.run_at <= now)).one()
        failed = db.session.scalar(
            sa.select(sa.func.count()).select_from(cls).where(cls.status == 'failed'))
        if oldest is not None and oldest.tzinfo is None:    # SQLite returns naive UTC
            oldest = oldest.replace(tzinfo=timezone.utc)
        return pending, (now - oldest).total_seconds() if oldest else 0.0, failed
```

The minimum set, by layer:

| Layer | Metrics |
|---|---|
| HTTP | requests by route/method/status, duration histogram |
| Jobs (`jobs: true`) | pending, **oldest pending age**, failed; duration is the `ms` on `job_done` |
| AI (`ai: true`) | calls, tokens, cost, latency by model/purpose, read from `ai_call` like the queue |
| Database | SQLite: `database is locked` errors, seen as 5xx. PostgreSQL: connections, counted on the server (each worker has its own pool) |

**Oldest pending age beats queue depth** for anything queue-shaped. Depth spikes
harmlessly during bursts; age only rises when you are not keeping up.

---

## Cardinality

**Never put an organization ID, user ID, record ID, or URL path parameter in a
Prometheus label.** Each distinct value creates a time series that lives in
memory forever. An `org_id` label on three metrics across a thousand
organizations is how a monitoring system takes down the thing it monitors.

```python
# Wrong: unbounded series
HTTP_REQUESTS.labels(request.path, request.method, response.status_code).inc()

# Right: the route pattern, '/channels/<int:id>/messages'
HTTP_REQUESTS.labels(request.url_rule.rule, request.method, response.status_code).inc()
```

A request that matches no route is labelled `unmatched`, or every crawler probe
becomes a series. Per-organization numbers belong in the database, where they
are rows and can be aggregated on demand.

Rule of thumb: a label is safe if you can name every value it will ever take.
Log fields have no such limit: `org_id` and `user_id` belong on every line.

---

## Cost as a Signal

> `ai: true` only. Skip this section if the project does not call a model.

For anything model-backed, cost is an operational signal with the same standing
as latency, and it degrades in ways latency does not: silently, and with a
month's delay before the invoice tells you.

Track it two ways, deliberately:

- **Metrics** for aggregate rate and alerting, labelled by `model` and `purpose`
  only.
- **The `ai_call` table** ([ai.md](../ai.md#cost-accounting)) for attribution
  per organization (per user with `tenancy: personal`), which is unbounded and
  therefore not a metric: a `GROUP BY org_id` since a cutoff computed in Python
  with `utcnow()`, because SQL date arithmetic differs between SQLite and
  PostgreSQL.

The number that decides whether the business works is **cost per active user per
month** against price. Put it on a dashboard before launch, not after the first
invoice.

---

## Tracing

Worth adding when a single user action spans processes and you have stopped
being able to say which hop was slow. With one web app and at most one worker,
that threshold arrives late, and for most projects never.

When it does: OpenTelemetry, a span per hop, `trace_id` shared with the logging
context so a trace links to its logs. Sample 1–5% of successful requests, 100%
of errors and of anything past a latency threshold.

Until then, `trace_id` in structured logs answers most of the same questions for
a fraction of the effort. **Do not build tracing before you have the correlation
ID in the logs**; it is the same value at ten times the cost.

---

## Health Checks

Two endpoints, different jobs. Conflating them causes outages. Both already
exist ([deployment.md](deployment.md#health-check)):

| Question | Endpoint | Checks |
|---|---|---|
| Liveness: is this process wedged? | `/health` | nothing |
| Readiness: can it serve? | `/api/v1/health` | the database |

**`/health` must not check the database.** The deploy script and the "be right
back" page poll it, and so does anything that restarts the app. A brief database
blip would fail it everywhere at once, and a recoverable incident becomes a
restart loop. `/api/v1/health` answers 200 either way, so a monitor reads its
`status` field (`healthy` or `degraded`), not the status code.

When `jobs: true`, the worker needs a check too: a process that stopped claiming
jobs is often still happily alive. The signal is `Job.queue_stats()`: jobs
waiting, and the oldest older than a few poll intervals.

---

## Alerting

Alert on **symptoms users experience**, not on causes. Cause-based alerts fire
constantly during normal operation and train people to ignore the channel.

Worth waking someone:

| Alert | Condition |
|---|---|
| Error rate | 5xx above 1% of requests for 5 min |
| Latency | p95 above the route's budget for 10 min |
| Queue falling behind (`jobs: true`) | `jobs_oldest_pending_seconds` past the queue's budget |
| Failed jobs (`jobs: true`) | Any increase in `jobs_failed` |
| Cost (`ai: true`) | Daily spend above 2× the trailing 7-day mean |
| Disk | Above 80% |

Dashboard material, consulted after an alert: memory, connection counts,
individual job failures, a single slow query. The provider's CPU and load email
from [Server Monitoring](deployment.md#server-monitoring) stays: it needs
nothing installed, and it is the alert you have before any of these exist.

Every alert needs a written response. An alert with no action is noise, and it
should be deleted, not tolerated.

---

## Never Log

- Passwords, tokens, session IDs, API keys, including inside request bodies and
  error strings from third parties, which frequently echo the request back.
- `request.path` or a full URL where a route takes a token (magic links,
  confirmations, unsubscribe). Log the route pattern, as
  [`slow_request`](deployment.md#slow-requests-in-the-journal) does.
- Full message bodies or user content. Log IDs and lengths; the content is in
  the database, where access is governed.
- Model prompts and completions containing user content. Log the prompt hash and
  token counts. If you need a sample for debugging, gate it behind an explicit
  flag, sample at a low rate, and expire it.
- Email addresses in metric labels.

A logging pipeline is usually the least access-controlled copy of your data,
retained by default and readable by everyone with a shell on the server. Treat
every line as though it will be read by someone who should not see the
underlying record: in a multi-tenant system, that is the realistic failure.

---

## Checklist

Every project:

- [ ] `register_observability(app)` after `init_tenant` and `register_profiling`
- [ ] Events are snake_case names with fields, one line per unit of work
- [ ] Error pages and JSON errors show the request ID
- [ ] `/health` checks nothing; nothing in [Never Log](#never-log) is logged
- [ ] `jobs: true`: `job.trace_id` stored by `enqueue`, restored in `_execute`

When metrics are added:

- [ ] `gunicorn.conf.py` sets `PROMETHEUS_MULTIPROC_DIR` and empties it on start
- [ ] `/metrics` needs `METRICS_TOKEN` and is 404 without it
- [ ] Labels are route patterns and other closed sets, never an ID or a path
- [ ] With four workers running, two scrapes in a row agree

---

**Next:** [Deployment](deployment.md) | [Performance](performance.md) | [Security](security.md)
