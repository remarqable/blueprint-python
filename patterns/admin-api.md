# Admin API

> **Layer doc.** Read this only when `admin_api: true` in the Project
> Configuration.

A read-only REST API that gives an AI agent (or any script) the app's numbers and
people: signups, sign-ins, active people, activation, retention, invitations,
revenue, product usage. It is authenticated by API keys that platform admins make
in the admin panel, and answers under `/admin/api/v1`.

The contract (URLs, response shapes, the metric catalogue, the privacy rules) is
the remarQable spec, `remarqable/docs/specs/admin-api.md`. Every app that turns
this layer on answers the same URLs the same way, so one agent can watch them
all. This doc is how to build it here; where the two disagree, the spec wins.

---

## Table of Contents

- [When You Need This](#when-you-need-this)
- [What It Adds](#what-it-adds)
- [Platform Admins and Real People](#platform-admins-and-real-people)
- [The ApiKey Model](#the-apikey-model)
- [Recording What Happens](#recording-what-happens)
- [The Metric Registry](#the-metric-registry)
- [Series in SQL](#series-in-sql)
- [The Core Catalogue](#the-core-catalogue)
- [The API Blueprint](#the-api-blueprint)
- [The Keys Page](#the-keys-page)
- [OpenAPI](#openapi)
- [Tenancy](#tenancy)
- [Testing](#testing)

---

## When You Need This

When someone other than a person in the admin panel needs the app's numbers: an
agent that reports weekly on growth and joins the app's figures with web
analytics, Search Console and GitHub (which it reads with their own APIs; the
app never relays them). If the only reader is a person, the admin dashboard is
enough; build its numbers on [the registry](#the-metric-registry) anyway, so
turning this on later is wiring, not rewriting.

## What It Adds

```
app/
├── models/
│   ├── api_key.py         # ApiKey: hashed keys, scopes, expiry, revocation
│   └── activity.py        # ActivityDay and AuthEvent: what the metrics count
├── platform/
│   ├── metrics.py         # the registry: Metric, Funnel, register(), series helpers
│   └── metrics_core.py    # the core catalogue (signups, sign-ins, DAU/WAU/MAU…)
├── controllers/
│   ├── admin_api.py       # /admin/api/v1/…: key check, endpoints, problem details
│   └── admin_keys.py      # /admin/api-keys: make, list, revoke
└── views/admin/api_keys.html
```

One migration adds `api_key`, `activity_day` and `auth_event`.

Config:

```python
class Config:
    # ...
    PLATFORM_ADMIN_EMAILS = {e.strip().lower() for e in
                             os.environ.get('PLATFORM_ADMIN_EMAILS', '').split(',') if e.strip()}
    API_KEY_PREFIX = os.environ.get('API_KEY_PREFIX', 'app')   # 'jd' -> jd_live_…
    API_RATE_LIMIT = int(os.environ.get('API_RATE_LIMIT', '120'))   # per key per minute
    APP_REPOSITORY_URL = os.environ.get('APP_REPOSITORY_URL', '')   # for /status commit links
```

---

## Platform Admins and Real People

Two definitions every metric depends on. Write them once, on `User`:

```python
# app/models/user.py
from flask import current_app

class User(BaseModel):
    # ...
    is_synthetic = db.Column(db.Boolean, nullable=False, default=False,
                             server_default=sa.false())   # demo, test or stand-in accounts

    @property
    def is_platform_admin(self) -> bool:
        """Runs the installation (not a tenant's admin role)."""
        return self.email.lower() in current_app.config['PLATFORM_ADMIN_EMAILS']

    @classmethod
    def real_people(cls) -> Query:
        """Accounts that are people: not synthetic, not platform staff. Every
        number the API gives counts only these."""
        staff = current_app.config['PLATFORM_ADMIN_EMAILS']
        query = cls.query.filter(cls.is_synthetic.is_(False))
        return query.filter(sa.func.lower(cls.email).notin_(staff)) if staff else query
```

An app that already marks its stand-ins another way (an email domain, a role)
puts that rule in `real_people()` instead; nothing else should know it.

## The ApiKey Model

The key itself is never stored: only an HMAC of it, keyed with `SECRET_KEY`, the
same way [magic-link tokens](core/auth.md#token-storage) are. Lookup is an
equality match on the hash, so its timing says nothing about how close a guess was.

```python
# app/models/api_key.py
import hashlib
import hmac
import secrets
import string
from datetime import timedelta
from typing import Optional

import sqlalchemy as sa
from flask import current_app

from app.extensions import db
from app.models.base import BaseModel, utcnow
from app.models.types import BigIntFK, JSONColumn
from app.platform.errors import ValidationError
from app.platform.i18n import t

SCOPES = ('metrics:read', 'people:read')
ALPHABET = string.ascii_letters + string.digits
TOUCH_EVERY = timedelta(minutes=1)


def _digest(raw: str) -> str:
    key = current_app.config['SECRET_KEY'].encode()
    return hmac.new(key, raw.encode(), hashlib.sha256).hexdigest()


class ApiKey(BaseModel):
    __tablename__ = 'api_key'

    name = db.Column(db.String(80), nullable=False)
    prefix = db.Column(db.String(16), nullable=False)          # shown in the panel
    key_hash = db.Column(db.String(64), nullable=False, unique=True)
    scopes = db.Column(JSONColumn, nullable=False, default=list)
    created_by_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='SET NULL'))
    expires_at = db.Column(db.DateTime(timezone=True))
    revoked_at = db.Column(db.DateTime(timezone=True))
    last_used_at = db.Column(db.DateTime(timezone=True))
    last_used_ip = db.Column(db.String(45))

    created_by = db.relationship('User')

    @classmethod
    def issue(cls, admin: 'User', name: str, scopes: list[str],
              days: Optional[int] = 90) -> tuple['ApiKey', str]:
        """A new key. Returns the row and the key itself, which is shown once."""
        name = (name or '').strip()
        if not name or len(name) > 80:
            raise ValidationError(t('api_keys.err_name'))
        if not scopes or any(s not in SCOPES for s in scopes):
            raise ValidationError(t('api_keys.err_scopes'))
        env = 'live' if current_app.config['APP_ENV'] == 'production' else 'test'
        raw = f"{current_app.config['API_KEY_PREFIX']}_{env}_" + \
              ''.join(secrets.choice(ALPHABET) for _ in range(32))
        row = cls(name=name, prefix=raw[:12], key_hash=_digest(raw), scopes=sorted(set(scopes)),
                  created_by_id=admin.id,
                  expires_at=utcnow() + timedelta(days=days) if days else None)
        row.save()
        return row, raw

    @classmethod
    def authenticate(cls, raw: str) -> Optional['ApiKey']:
        """The live key this is, or None (unknown, revoked, expired, or its maker
        is no longer a platform admin). Callers answer every None the same way."""
        if not raw or len(raw) > 100:
            return None
        row = cls.query.filter_by(key_hash=_digest(raw)).first()
        if row is None or row.revoked_at is not None:
            return None
        if row.expires_at is not None and row.expires_at <= utcnow():
            return None
        if row.created_by is None or not row.created_by.is_platform_admin:
            return None
        return row

    def allows(self, scope: str) -> bool:
        return scope in (self.scopes or [])

    def touch(self, ip: str) -> None:
        """Note the use, at most once a minute (a busy agent mustn't write per request)."""
        now = utcnow()
        if self.last_used_at is None or now - self.last_used_at > TOUCH_EVERY:
            self.last_used_at, self.last_used_ip = now, (ip or '')[:45]
            db.session.commit()

    def revoke(self) -> None:
        if self.revoked_at is None:
            self.revoked_at = utcnow()
            db.session.commit()
```

Revoked keys are kept, never deleted: the admin log names them.

## Recording What Happens

Metrics come from timestamps the app already has (`user.created_at`, each
product's `created_at`) plus two small tables for what it doesn't:

```python
# app/models/activity.py
from datetime import date
from typing import Optional

import sqlalchemy as sa
from flask import session

from app.extensions import db
from app.models.base import BaseModel, utcnow
from app.models.types import BigIntFK


class ActivityDay(BaseModel):
    """One row per person per day per area they were active in: the source of
    DAU/WAU/MAU, retention, churn and activity counts. Never a log of requests."""
    __tablename__ = 'activity_day'
    __table_args__ = (db.UniqueConstraint('user_id', 'day', 'area',
                                          name='uq_activity_day_user_id_day_area'),)

    user_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='CASCADE'), nullable=False)
    day = db.Column(db.Date, nullable=False, index=True)
    area = db.Column(db.String(30), nullable=False, default='app')   # a product key, or 'app'

    @classmethod
    def note(cls, user, area: str = 'app') -> None:
        """Called from a before_request for signed-in pages. The session remembers
        today's notes, so the database sees one insert per person per day per area."""
        today = utcnow().date().isoformat()
        seen = session.get('_active', {})
        if seen.get(area) == today:
            return
        cls._insert(user.id, utcnow().date(), area)
        session['_active'] = {**seen, area: today}

    @classmethod
    def _insert(cls, user_id: int, day: date, area: str) -> None:
        exists = cls.query.filter_by(user_id=user_id, day=day, area=area).first()
        if exists is None:
            db.session.add(cls(user_id=user_id, day=day, area=area))
            try:
                db.session.commit()
            except sa.exc.IntegrityError:    # another worker noted it first
                db.session.rollback()


class AuthEvent(BaseModel):
    """Sign-ins and failed attempts, for both auth strategies."""
    __tablename__ = 'auth_event'

    kind = db.Column(db.String(20), nullable=False, index=True)   # 'signin' | 'signin_failed'
    user_id = db.Column(BigIntFK, db.ForeignKey('user.id', ondelete='SET NULL'))

    @classmethod
    def record(cls, kind: str, user: Optional['User'] = None) -> None:
        db.session.add(cls(kind=kind, user_id=user.id if user else None))
        db.session.commit()
```

Wire them where the facts happen: `AuthEvent.record('signin', user)` after a
successful login (magic link or password), `'signin_failed'` on a wrong code or
password; `ActivityDay.note(current_user, '<area>')` in each area's
`before_request` (the same place an app notes "last seen"). Index
`auth_event.created_at` and `user.created_at`: every series filters on them.

## The Metric Registry

The API, the admin dashboard and anything later read numbers from here. Each
part of the app registers its metrics at import; the registry answers.

```python
# app/platform/metrics.py
from dataclasses import dataclass, field
from datetime import date, datetime
from typing import Callable, Optional

INTERVALS = ('day', 'week', 'month')
UNITS = ('count', 'percent', 'ratio', 'cents', 'seconds')
KINDS = ('counter', 'gauge', 'rate')

# (from, to, interval, segment) -> {bucket start: value} or {segment: {bucket: value}}
SeriesFn = Callable[[datetime, datetime, str, Optional[str]], dict]


@dataclass(frozen=True)
class Metric:
    key: str                       # 'core.signups', 'ledger.entries_added'
    title: str
    description: str               # the exact definition: an agent reads it
    unit: str
    kind: str
    series: SeriesFn
    higher_is_better: bool = True
    intervals: tuple[str, ...] = INTERVALS
    segments: tuple[str, ...] = ()
    headline: bool = False
    product: str = 'core'


@dataclass(frozen=True)
class FunnelStep:
    key: str
    title: str
    people: Callable[[datetime, datetime], set[int]]   # cohort window -> user ids at this step


@dataclass(frozen=True)
class Funnel:
    key: str
    title: str
    steps: tuple[FunnelStep, ...]


_metrics: dict[str, Metric] = {}
_funnels: dict[str, Funnel] = {}


def register(metric: Metric) -> None:
    if metric.key in _metrics:
        raise ValueError(f'Metric {metric.key} registered twice')
    if metric.unit not in UNITS or metric.kind not in KINDS:
        raise ValueError(f'Metric {metric.key}: unknown unit or kind')
    _metrics[metric.key] = metric


def register_funnel(funnel: Funnel) -> None:
    _funnels[funnel.key] = funnel


def get(key: str) -> Optional[Metric]:
    return _metrics.get(key)


def all_metrics() -> list[Metric]:
    return sorted(_metrics.values(), key=lambda m: m.key)
```

Registration is a startup error when a key repeats or a unit is unknown: an agent
must never get two meanings for one name.

## Series in SQL

A series is one grouped query, never rows loaded into Python. Bucketing differs
by engine ([portability.md](core/portability.md)), so it lives in one helper:

```python
# app/platform/metrics.py (continued)
import sqlalchemy as sa
from datetime import timedelta

from app.extensions import db


def bucket(column, interval: str):
    """The start of `column`'s day, ISO week (Monday) or month, as a date string."""
    if db.engine.dialect.name == 'postgresql':
        unit = {'day': 'day', 'week': 'week', 'month': 'month'}[interval]
        return sa.func.to_char(sa.func.date_trunc(unit, sa.func.timezone('UTC', column)), 'YYYY-MM-DD')
    fmt = {'day': sa.func.date(column),
           'week': sa.func.date(column, '-6 days', 'weekday 1'),
           'month': sa.func.strftime('%Y-%m-01', column)}
    return fmt[interval]


def starts(start: date, end: date, interval: str) -> list[date]:
    """Every bucket start from the one holding `start` up to `end` (exclusive)."""
    if interval == 'week':
        start -= timedelta(days=start.weekday())
    elif interval == 'month':
        start = start.replace(day=1)
    out, cur = [], start
    while cur < end:
        out.append(cur)
        if interval == 'day':
            cur += timedelta(days=1)
        elif interval == 'week':
            cur += timedelta(weeks=1)
        else:
            cur = (cur.replace(day=28) + timedelta(days=4)).replace(day=1)
    return out


def count_by(query, column, start: datetime, end: datetime, interval: str) -> dict[str, int]:
    """A counter: how many rows of `query` fall in each bucket of `column`."""
    b = bucket(column, interval)
    rows = (query.filter(column >= start, column < end)
            .with_entities(b, sa.func.count()).group_by(b).all())
    return {k: n for k, n in rows}


def distinct_by(query, column, who, start: datetime, end: datetime, interval: str) -> dict[str, int]:
    """Distinct `who` per bucket (people active per day, for DAU)."""
    b = bucket(column, interval)
    rows = (query.filter(column >= start, column < end)
            .with_entities(b, sa.func.count(sa.distinct(who))).group_by(b).all())
    return {k: n for k, n in rows}
```

The endpoint fills empty buckets (`0` for counters, the last value for gauges)
from `starts()`, so a series always has every bucket.

Gauges over a sliding window (WAU, MAU) are one query per bucket end: at most 104
for two years of weeks, each an indexed range count on `activity_day`. Allow
`day` for them only over ranges of 90 days or less.

## The Core Catalogue

Every app with the layer registers these; the spec lists the rest (collaboration,
acquisition, revenue, B2B, quality) to add when the app has the feature.

```python
# app/platform/metrics_core.py
from datetime import timedelta

import sqlalchemy as sa

from app.models.activity import ActivityDay, AuthEvent
from app.models.user import User
from app.platform.metrics import Metric, count_by, distinct_by, register


def _real_ids():
    return User.real_people().with_entities(User.id)


def _signups(start, end, interval, _segment):
    return count_by(User.real_people(), User.created_at, start, end, interval)


def _signins(start, end, interval, _segment):
    q = AuthEvent.query.filter(AuthEvent.kind == 'signin', AuthEvent.user_id.in_(_real_ids()))
    return count_by(q, AuthEvent.created_at, start, end, interval)


def _active_in(days: int):
    def series(start, end, interval, _segment):
        from app.platform.metrics import starts
        out = {}
        for b in starts(start.date(), end.date(), interval):
            window_end = b + {'day': timedelta(days=1), 'week': timedelta(weeks=1)}.get(interval, timedelta(days=31))
            q = (ActivityDay.query.filter(ActivityDay.user_id.in_(_real_ids()),
                                          ActivityDay.day >= window_end - timedelta(days=days),
                                          ActivityDay.day < window_end)
                 .with_entities(sa.func.count(sa.distinct(ActivityDay.user_id))))
            out[b.isoformat()] = q.scalar() or 0
        return out
    return series


register(Metric('core.signups', 'Signups',
                'Accounts created, not counting platform admins or synthetic accounts.',
                'count', 'counter', _signups, headline=True))
register(Metric('core.signins', 'Sign-ins', 'Successful sign-ins by real people.',
                'count', 'counter', _signins, headline=True))
register(Metric('core.dau', 'Daily active people',
                'Distinct real people active (ActivityDay) on the day ending each bucket.',
                'count', 'gauge', _active_in(1)))
register(Metric('core.wau', 'Weekly active people', 'Distinct real people active in the 7 days ending each bucket.',
                'count', 'gauge', _active_in(7), intervals=('week', 'month')))
register(Metric('core.mau', 'Monthly active people', 'Distinct real people active in the 30 days ending each bucket.',
                'count', 'gauge', _active_in(30), intervals=('week', 'month'), headline=True))
```

Also register `core.verified_signups`, `core.signin_failures`, `core.stickiness`,
`core.activation_rate` (the app supplies its activation event as a funnel step),
`core.time_to_activate`, `core.churned`, `core.resurrected` and `core.people`, as
defined in the spec. Each product registers its own (`<product>.<name>`) in its
package, so the core never imports a product.

## The API Blueprint

One blueprint, its own auth. It reads no session and sets no cookie: a browser
session, even an admin's, never authenticates it, and a key opens nothing else.

```python
# app/controllers/admin_api.py
import json
import time
from collections import defaultdict, deque
from datetime import datetime, timedelta, timezone
from typing import Any, Optional

from flask import Blueprint, Response, current_app, g, request

from app.middleware.csrf import csrf_exempt_blueprint
from app.models.api_key import ApiKey
from app.models.base import utcnow
from app.platform import metrics
from app.platform.logger import get_logger

bp = Blueprint('admin_api', __name__, url_prefix='/admin/api/v1')
csrf_exempt_blueprint(bp)        # bearer keys, not cookies: there is nothing to forge
log = get_logger()

PROBLEMS = 'https://remarqable.io/problems/'
_hits: dict[int, deque] = defaultdict(deque)     # per-process; see core/security.md for Redis


def problem(status: int, kind: str, title: str, detail: str = '', **extra: Any) -> Response:
    body = {'type': PROBLEMS + kind, 'title': title, 'status': status, **({'detail': detail} if detail else {}), **extra}
    resp = Response(json.dumps(body), status, mimetype='application/problem+json')
    resp.headers['Cache-Control'] = 'no-store'
    return resp


def ok(data: Any, **meta: Any) -> Response:
    body = {'data': data, 'meta': {'app': current_app.config['APP_NAME'],
                                   'generated_at': utcnow().isoformat().replace('+00:00', 'Z'),
                                   'version': current_app.config.get('APP_VERSION', ''),
                                   'commit': current_app.config.get('GIT_SHA', ''), **meta}}
    resp = Response(json.dumps(body, default=str), 200, mimetype='application/json')
    resp.headers['Cache-Control'] = 'no-store'
    return resp


@bp.before_request
def authenticate() -> Optional[Response]:
    header = request.headers.get('Authorization', '')
    raw = header[7:].strip() if header.lower().startswith('bearer ') else ''
    key = ApiKey.authenticate(raw)
    if key is None:                               # one answer for every reason
        resp = problem(401, 'unauthorized', 'Missing or invalid API key')
        resp.headers['WWW-Authenticate'] = 'Bearer'
        return resp
    window, limit = _hits[key.id], current_app.config['API_RATE_LIMIT']
    now = time.monotonic()
    while window and now - window[0] > 60:
        window.popleft()
    if len(window) >= limit:
        resp = problem(429, 'rate-limited', 'Too many requests')
        resp.headers['Retry-After'] = str(int(60 - (now - window[0])) + 1)
        return resp
    window.append(now)
    key.touch(request.remote_addr or '')
    g.api_key = key
    return None


def need(scope: str) -> Optional[Response]:
    if not g.api_key.allows(scope):
        return problem(403, 'forbidden', 'Not allowed', f'This key needs the {scope} scope.')
    return None


@bp.after_request
def log_use(resp: Response) -> Response:
    key = getattr(g, 'api_key', None)
    log.info('admin_api', key_id=key.id if key else None, path=request.path, status=resp.status_code)
    return resp


def _range() -> tuple[datetime, datetime] | Response:
    """from/to (dates or timestamps, UTC): the last 30 days by default, two years at most."""
    try:
        end = _parse(request.args.get('to')) or utcnow()
        start = _parse(request.args.get('from')) or end - timedelta(days=30)
    except ValueError:
        return problem(422, 'invalid-parameter', 'Invalid parameter', 'from and to must be ISO dates', param='from')
    if start >= end or end - start > timedelta(days=731):
        return problem(422, 'invalid-parameter', 'Invalid parameter', 'from must be before to, at most 2 years apart', param='from')
    return start, end


def _parse(value: Optional[str]) -> Optional[datetime]:
    if not value:
        return None
    parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)


@bp.get('/metrics')
def catalogue() -> Response:
    if (denied := need('metrics:read')):
        return denied
    return ok({'metrics': [{'key': m.key, 'title': m.title, 'description': m.description,
                            'product': m.product, 'unit': m.unit, 'kind': m.kind,
                            'higher_is_better': m.higher_is_better, 'intervals': list(m.intervals),
                            'segments': list(m.segments), 'headline': m.headline}
                           for m in metrics.all_metrics()]})


@bp.get('/metrics/<key>')
def series(key: str) -> Response:
    if (denied := need('metrics:read')):
        return denied
    metric = metrics.get(key)
    if metric is None:
        return problem(404, 'not-found', 'Unknown metric', key)
    interval = request.args.get('interval', 'day')
    if interval not in metric.intervals:
        return problem(422, 'invalid-parameter', 'Invalid parameter',
                       f'interval must be one of {", ".join(metric.intervals)}', param='interval')
    window = _range()
    if isinstance(window, Response):
        return window
    start, end = window
    raw = metric.series(start, end, interval, None)
    points, last = [], 0
    for b in metrics.starts(start.date(), end.date(), interval):
        value = raw.get(b.isoformat())
        if value is None:
            value = last if metric.kind == 'gauge' else 0
        points.append({'t': b.isoformat(), 'value': value})
        last = value
    if points:
        points[-1]['partial'] = True if end >= utcnow() - timedelta(minutes=1) else False
    data = {'key': key, 'unit': metric.unit, 'kind': metric.kind, 'interval': interval,
            'from': start.date().isoformat(), 'to': end.date().isoformat(), 'points': points}
    if metric.kind == 'counter':
        data['total'] = sum(p['value'] for p in points)
    return ok(data)
```

`/status`, `/summary`, `/funnels`, `/cohorts/retention`, `/events` and `/people`
follow the same shape: check the scope, validate parameters into a 422, compute
in the models or the registry, wrap in `ok()`. Keep the endpoints thin: what a
person "is" for `/people` (their products, whether they're activated) is a model
method, `User.api_view(detail=False)`, that products extend through hooks.

The CSRF middleware gets a blueprint-level exemption, since its requests carry no
cookie to forge:

```python
# app/middleware/csrf.py (addition)
_exempt_blueprints: set[str] = set()

def csrf_exempt_blueprint(bp) -> None:
    _exempt_blueprints.add(bp.name)

# in check_csrf(): if request.blueprint in _exempt_blueprints: return
```

## The Keys Page

`/admin/api-keys`, platform admins only, in the admin panel's layout.

- A table: name, prefix (`jd_live_4f9K…`), scopes, created (by whom, when),
  expires, last used, and Revoke.
- **New key**: a name, the scopes (`metrics:read` ticked; `people:read` unticked,
  with "Shares people's emails, names and activity"), and an expiry (30, 90, 365
  days, or never; 90 selected).
- After making one, the page shows the whole key **once**, in a read-only field
  with Copy and "Copy it now: it won't be shown again." Hold it in the response,
  never in the session or a flash.
- Revoke asks first (a confirm step, not a JavaScript `confirm()`).
- Making and revoking each write an admin-log entry naming the key's name and
  prefix, never the key.

Every string through `t()`, every form with its CSRF token, and the page with the
rest of the admin panel's protection (`is_platform_admin`, 404 otherwise).

## OpenAPI

`GET /admin/api/v1/openapi.json` is built from the registry and the endpoint
definitions, so it can't drift: each path, its parameters, the envelope, the
problem schema, and `components.schemas.MetricKey` as an enum of every registered
key. An agent that reads it knows everything this app measures.

## Tenancy

With `tenancy: shared`, metrics count across every tenant. The API runs with
tenant scoping off, explicitly: wrap each handler's work in `unscoped()`
([tenancy.md](tenancy.md#the-escape-hatch)), justified once at the blueprint
rather than per query. The B2B catalogue (`b2b.orgs`, `b2b.active_orgs`,
`b2b.seats`) counts organisations and memberships; `activity_day` gains an
`org_id` (nullable) so active organisations come from the same table.

## Testing

```python
# tests/test_admin_api.py
import pytest

from app.models.api_key import ApiKey


@pytest.fixture
def key(app, admin):
    _row, raw = ApiKey.issue(admin, 'test agent', ['metrics:read'])
    return {'Authorization': f'Bearer {raw}'}


@pytest.mark.parametrize('header', [None, 'Bearer ', 'Bearer nonsense', 'Basic abc'])
def test_no_valid_key_is_one_401(client, header):
    resp = client.get('/admin/api/v1/summary', headers={'Authorization': header} if header else {})
    assert resp.status_code == 401 and resp.mimetype == 'application/problem+json'


def test_revoked_and_expired_keys_are_refused_alike(client, admin):
    row, raw = ApiKey.issue(admin, 'x', ['metrics:read'])
    row.revoke()
    assert client.get('/admin/api/v1/summary', headers={'Authorization': f'Bearer {raw}'}).status_code == 401


def test_a_session_never_authenticates_the_api(client, login, admin):
    login(admin)
    assert client.get('/admin/api/v1/summary').status_code == 401


def test_metrics_read_cannot_read_people(client, key):
    assert client.get('/admin/api/v1/people', headers=key).status_code == 403


def test_the_key_is_stored_only_as_its_hash(admin):
    row, raw = ApiKey.issue(admin, 'x', ['metrics:read'])
    assert raw not in (row.key_hash, row.prefix) and row.prefix == raw[:12]


def test_signups_fill_every_bucket_and_leave_out_staff(client, key, make_user, admin):
    make_user('a@example.com'); make_user('b@example.com')
    data = client.get('/admin/api/v1/metrics/core.signups?interval=day', headers=key).json['data']
    assert len(data['points']) == 30 and data['total'] == 2       # the admin isn't counted
```

Plus, from the spec's list: a key whose maker is no longer a platform admin is
refused; `/openapi.json` names every registered metric; the 429 carries
`Retry-After`; making and revoking appear in the admin log; and each product's
metrics match a hand count on a seeded dataset.
