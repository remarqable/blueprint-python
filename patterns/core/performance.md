# Performance

> A page's query count is a constant. It does not grow with the rows the page
> shows.

In apps like this, most slow pages are not slow databases. They are fast
queries issued many times: one per row, one per card, one per category in a
list of fifty. Each costs
a round trip, and the total grows with the data, so a page that took 30 ms on a
seeded laptop takes three seconds in production. This doc is the rule, how to
catch a break of it, the fixes, and the two cheap wins outside the database
(static file caching and compression).

Measuring comes first and is already built: the admin **Info** box and the
`Server-Timing` header ([deployment.md](deployment.md#request-profiling-system-info)).

## Table of Contents

- [The Rule](#the-rule)
- [Catching It](#catching-it)
- [Fixing It](#fixing-it)
- [Relationship Loading](#relationship-loading)
- [Caching Data](#caching-data)
- [Static Files](#static-files)
- [Compression](#compression)
- [Checklist](#checklist)

---

## The Rule

**An N+1 is a bug, not a tuning opportunity.** Fix it before the page ships,
the same as a missing CSRF token. It applies to every page that lists
anything: public listings, admin tables, API endpoints, sitemaps, and jobs that
loop over rows.

What an N+1 looks like is rarely a query in a visible loop. It hides in:

- a model method called per row (`post.author_name()`, `Setting.get_value(user.id, 'theme')`)
- a relationship touched in a template loop (`{% for p in posts %}{{ p.author.name }}`)
- a helper that "just looks one thing up", called once per card
- loading rows to count or group them in Python

Its cousin is extra work on every query: a relationship whose default is eager
loads on every list that returns the model, whether the page shows it or not
([Relationship Loading](#relationship-loading)).

---

## Catching It

**In the browser:** open the page signed in as an admin and read the Info box.
A list page whose query count is near its row count has an N+1. DB time small
against total time means the cost is in Python instead (see
[deployment.md § Using it](deployment.md#using-it)).

**In a test**, mechanically, so it cannot come back. Count statements around a
request, then grow the data and count again. The two numbers must match.

```python
# tests/assertions.py, beside the other test helpers (testing.md)
from contextlib import contextmanager
from typing import Any, Iterator

from sqlalchemy import event

from app.extensions import db


class QueryCount:
    n = 0


@contextmanager
def count_queries() -> Iterator[QueryCount]:
    """Statements issued inside the block. Needs an app context."""
    count = QueryCount()

    def _count(*args: Any) -> None:
        count.n += 1

    event.listen(db.engine, 'before_cursor_execute', _count)
    try:
        yield count
    finally:
        event.remove(db.engine, 'before_cursor_execute', _count)
```

```python
# tests/test_controllers/test_posts.py
from app.extensions import db
from tests.assertions import count_queries
from tests.factories import create_post


def test_post_list_queries_do_not_grow(app, client):
    def measure() -> int:
        db.session.expunge_all()      # start cold, as a production request does
        with count_queries() as q:
            client.get('/posts')
        return q.n

    for _ in range(3):
        create_post()
    client.get('/posts')              # warm: templates, process caches
    few = measure()
    for _ in range(30):
        create_post()
    assert measure() == few, 'a query per post'
```

`create_post` is a factory like `create_user` ([testing.md](testing.md#factory-functions)).
The `app` fixture holds one app context for the whole test, so the test client's
requests share the test's session: rows the factories just created answer a
`post.author` lookup from memory, and the N+1 never reaches the database.
`expunge_all()` before each measurement makes every request start cold.

Comparing two sizes beats a fixed ceiling: it fails on the first query per row
and never needs retuning when the page gains a legitimate query. A test with
three rows and a ceiling of ten passes whether or not the bug exists.

---

## Fixing It

In order of preference. All of these live in the model
([mvc.md](mvc.md#models-fat-models)); the controller still makes one call.

### Fetch for the whole page, then look up in a dict

The general fix. Collect the ids on the page, fetch once, and return a dict the
caller indexes:

```python
# app/models/user.py
@classmethod
def names_by_id(cls, ids: Iterable[int]) -> dict[int, str]:
    """{user_id: name} for a page of rows, in one query."""
    ids = set(ids)
    if not ids:
        return {}
    rows = db.session.execute(sa.select(cls.id, cls.name).where(cls.id.in_(ids)))
    return {r.id: r.name for r in rows}
```

```python
# app/controllers/posts.py
posts = Post.list_recent(limit=50)
names = User.names_by_id(p.author_id for p in posts)
return render_template('posts/index.html', posts=posts, names=names)
```

Keep the single-row method (`User.get_by_id`) for single-row pages. Add the
batch method beside it; don't make every caller build a list of one.

When a row needs "the first of several" (a logo, a primary photo), fetch them
all for the page ordered the same way the single-row method orders them, and
keep the first per key. If the single-row method has no `ORDER BY`, the batch
one cannot promise the same row: give both the same order before switching.

### Eager-load the relationship, per query

When the template walks a relationship, load it with the list:

```python
from sqlalchemy.orm import joinedload, selectinload

posts = db.session.scalars(
    sa.select(Post)
    .options(joinedload(Post.author),          # many-to-one: one JOIN
             selectinload(Post.tags))          # collection: one extra IN query
    .order_by(Post.created_at.desc()).limit(50)).all()
```

`joinedload` for a many-to-one, `selectinload` for a collection. A `joinedload`
of a collection multiplies rows; `selectinload` adds one query however long the
list.

To find the lazy loads a list still does, make them fail while you work on it:
`.options(raiseload('*'))` raises on any relationship the query did not load.

### Count and group in SQL

Loading rows to count them moves the whole table into Python. Let the database
aggregate:

```python
@classmethod
def counts_by_post(cls, post_ids: Iterable[int]) -> dict[int, int]:
    """{post_id: comments} for a page of posts, in one query."""
    post_ids = set(post_ids)
    if not post_ids:
        return {}
    rows = db.session.execute(
        sa.select(cls.post_id, sa.func.count(cls.id))
        .where(cls.post_id.in_(post_ids)).group_by(cls.post_id))
    return {post_id: n for post_id, n in rows}
```

The same goes for "one query per group": fifty categories each counting their
posts is one query grouped by category, split in Python.

### Select columns, not objects, for lists

A list that shows three fields does not need the model. Selecting columns skips
object construction and, more importantly, skips every eager relationship the
model would bring with it:

```python
rows = db.session.execute(
    sa.select(Post.id, Post.title, Post.slug, Post.created_at)
    .where(Post.status == 'published').order_by(Post.created_at.desc()))
```

Use `load_only` ([database.md](database.md#query-optimization)) when the
template needs model methods but not every column.

### Insert in one statement

Writing a row per item (an import, a log line per result) is an N+1 on the
write side:

```python
db.session.execute(sa.insert(Contact), [
    {'org_id': g.org.id, 'email': r['email'], 'name': r['name']} for r in rows])
db.session.commit()
```

Column defaults (`created_at`) still apply. The ORM's hooks do not: there is no
`validate()` ([database.md](database.md#base-model)), and no `before_flush`, so
an `OrgScoped` model does not get `org_id` stamped
([tenancy.md](../tenancy.md)). Validate the rows and set `org_id` yourself, as
above, or keep `save()` for models whose behaviour depends on those hooks.

### Page in the query

`LIMIT`/`OFFSET` in SQL, never slicing a full result in Python. A filter that
needs counts across the whole result set can count in SQL (above) and fetch only
the page's rows in full.

---

## Relationship Loading

A relationship's `lazy=` is the default for **every** query that returns the
model. `lazy='selectin'` on a collection means listing a thousand rows also
loads a thousand rows' worth of that collection, on pages that never show it.

- **Leave collections lazy** (the default, `lazy='select'`) and eager-load them
  per query with `selectinload` where a page uses them.
- A many-to-one that nearly every view shows (a row's author) can default to
  `lazy='joined'` ([audit.md](audit.md) does this for `created_by`): one JOIN,
  no extra round trip.
- Where an eager default already exists, a list that does not need it turns it
  off for that query: `.options(lazyload(Post.attachments))`. `lazyload` still loads
  on first touch, so nothing breaks if a template does use it; `noload` returns
  an empty list instead, and silently hides a bug.

---

## Caching Data

In order of value:

1. **Do less work.** A removed N+1 beats any cache, and it is never stale.
2. **Before caching, add the right index.** Most slow single queries are a
   missing index, not a missing cache ([database.md](database.md#indexes)).
3. **Once per request**, on `g`, for a lookup several parts of one page repeat
   (the navbar, the page and the footer each reading the user's settings). Only
   on `GET`: a `POST` changes rows as it goes and must read afresh, and a job
   has no request at all.

   ```python
   # app/models/setting.py, beside get_map (mvc.md)
   @classmethod
   def map_for_request(cls, user_id: int) -> dict[str, str]:
       """get_map, read once per page view."""
       if not has_request_context() or request.method != 'GET':
           return cls.get_map(user_id)
       memo: dict[int, dict[str, str]] = g.setdefault('_settings', {})
       if user_id not in memo:
           memo[user_id] = cls.get_map(user_id)
       return memo[user_id]
   ```

4. **Once per process**, for data that changes rarely and can be slightly
   stale: `functools.lru_cache` for data that never changes while the process
   runs (a catalog file), a timestamped module-level value with a TTL for data
   that changes (menu links, settings). Each Gunicorn worker holds its own
   copy, so the TTL bounds how long workers disagree.

Rules for any cache: the app is correct with it empty (it is, after every
restart); with `tenancy: shared`, the key includes the tenant
([tenancy.md](../tenancy.md)), because a cross-tenant cache hit is a data leak;
and prefer expiry to invalidation, which is correct only until the third writer.

**Don't cache HTML pages in a shared cache.** A page carries the session cookie,
a CSRF token and, for admins, the Info box. Fix the queries instead.

---

## Static Files

`url_for('static', ...)` URLs are the same before and after a deploy, so Flask
serves them with `Cache-Control: no-cache`: the browser asks the server about
`app.css`, htmx and Alpine on every page load. Put the deployed version in the
URL instead, and a URL's content never changes, so the browser can keep it for
a year:

```python
# app/platform/assets.py -- register_assets(app), called from create_app
"""Static URLs carry the deployed version, so browsers keep them for a year."""
import os
from pathlib import Path
from typing import Any

from flask import Flask, request
from flask.wrappers import Response

ROOT = Path(__file__).resolve().parents[2]
YEAR = 31536000


def asset_version() -> str:
    """The deployed commit and when deploy.sh wrote it, so redeploying an older
    commit still changes every URL. APP_VERSION in a Docker image, which has no
    .git_sha; 'dev' locally."""
    sha = ROOT / '.git_sha'
    try:
        return f'{sha.read_text().strip()}.{int(sha.stat().st_mtime)}'
    except OSError:
        return os.environ.get('APP_VERSION') or 'dev'


def register_assets(app: Flask) -> None:
    version = asset_version()
    if version == 'dev':
        return                                # local edits show on refresh

    @app.url_defaults
    def _versioned(endpoint: str, values: dict[str, Any]) -> None:
        if endpoint == 'static':
            values.setdefault('v', version)   # /static/css/app.css?v=3f2a1c9.1761234567

    @app.after_request
    def _cache_static(response: Response) -> Response:
        if (request.endpoint == 'static' and request.args.get('v') == version
                and response.status_code in (200, 206, 304)):
            response.cache_control.no_cache = None
            response.cache_control.public = True
            response.cache_control.max_age = YEAR
            response.cache_control.immutable = True
        return response
```

- **Templates don't change.** Every `url_for('static', filename=...)` gets the
  version; a hard-coded `/static/...` path does not, so write static URLs with
  `url_for` ([frontend.md](frontend.md)).
- **A deploy busts it.** deploy.sh rewrites `.git_sha`, and the restart reads
  it, so every URL changes. The time is part of the version because the old
  workers keep serving between `git pull` and the restart: a file they hand out
  in that window is the new one under the old URL, and a later rollback to that
  commit must not reuse it.
- **Docker:** build with `--build-arg APP_VERSION=$(git rev-parse --short HEAD)`
  and `ARG APP_VERSION` / `ENV APP_VERSION=$APP_VERSION` in the Dockerfile.
  Without it the image reports `dev` and static files go uncached.
- **Only this version's URL is cached long.** A page from before the deploy
  asking for `?v=<old>` gets `no-cache`, so it can never pin old content.
- **Uploaded files** are already served with a one-year `max_age`
  ([storage.md](../storage.md)); this covers the files in `app/static/`.

---

## Compression

HTML, CSS and JavaScript compress to a fraction of their size. Caddy does it
with one line in the site block, `encode zstd gzip`
([deployment.md](deployment.md#3-caddy-reverse-proxy)), and skips images and
other formats that are already compressed.

---

## Checklist

Before a page that lists rows is done:

- [ ] Its query count is the same with 3 rows and 30 (a `count_queries` test)
- [ ] No model method or relationship is called per row in the controller or the
      template loop
- [ ] Counts and groups are computed in SQL
- [ ] No collection relationship defaults to eager; eager loads are per query
- [ ] Lists page in SQL
- [ ] The admin Info box shows it under a second in production

Once per project:

- [ ] `register_assets(app)` in `create_app`; static URLs via `url_for`
- [ ] `encode zstd gzip` in the Caddyfile

---

**Next:** [Database](database.md) | [Deployment](deployment.md) | [Testing](testing.md)
