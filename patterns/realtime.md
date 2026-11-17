# Realtime

> **Layer doc.** Read this only when `realtime: true` in the
> [Project Configuration](../CLAUDE.md#-project-configuration).

Long-lived connections, server-side fanout, and reconnection that does not lose
messages: chat, live counters, collaborative state. It is a separate concern
from [core/htmx.md](core/htmx.md), which covers request-driven interactivity,
and it runs in a separate **process**.

---

## Table of Contents

- [The Central Rule](#the-central-rule)
- [Process Shape](#process-shape)
- [Transport Choice](#transport-choice)
- [Connection Lifecycle](#connection-lifecycle)
- [Ordering](#ordering)
- [Fanout Across Processes](#fanout-across-processes)
- [Reconnection and Resume](#reconnection-and-resume)
- [Backpressure](#backpressure)
- [Presence](#presence)
- [Authorization](#authorization)
- [Testing](#testing)
- [Operating It](#operating-it)
- [Checklist](#checklist)

---

## The Central Rule

**The socket is a hint. The database is the truth.**

A push tells a client that something changed. It does not deliver the change,
and it is never the only way a client can learn something. Every client can
rebuild complete, correct state from an HTTP read with a cursor, and does so on
connect, on reconnect, and whenever it sees a gap.

Accept this and the hard problems go away: a dropped frame costs latency, not
data; nothing needs exactly-once delivery; a client offline for twenty minutes
recovers by asking, not because you buffered twenty minutes of frames for it.

---

## Process Shape

The gateway is its own process with its own entry point, `wsgi_gateway.py`,
beside `wsgi.py`. It is never the main Gunicorn app: those workers are
synchronous, so one open stream holds one worker until the client leaves, and
four open tabs would stop a `-w 4` app from serving anything else.

`create_gateway_app()` in `app/gateway.py` repeats `create_app` up to and
including `init_tenant` (config, logger, `db`, `login_manager`, SQLite
pragmas), so it shares the models and the session cookie. Then it calls
`init_realtime(app)`, registers the `realtime` blueprint and no other, and
answers its own `/health` with
`{'status': 'healthy', 'connections': len(hub.conns())}`. `create_app` calls
`init_realtime(app)` too, because writers publish.

```bash
gunicorn wsgi_gateway:app -k gthread -w 1 --threads 500 --graceful-timeout 3 -b 127.0.0.1:8010
```

**The ceiling is one thread per open stream.** Measured on one worker: 400 idle
streams cost about 70 KB each, and one event reached all of them in under half a
second. A stream beyond `--threads` waits unanswered, and so does `/health`.
That is hundreds of connections per process, not tens of thousands. Past that
the gateway moves to an async server; clients and the main app do not change,
because they only know the protocol below.

**The gateway never writes domain data.** Clients send writes to the main app
over HTTP; a write path here would repeat authorization, validation and CSRF in
a second place. A client tells the gateway one thing, in the URL: which
channels it is looking at.

---

## Transport Choice

| Transport | Use when |
|---|---|
| **SSE** | Server to client only. Plain HTTP, the browser reconnects by itself, the HTMX SSE extension consumes it. |
| **WebSocket** | You need client to server frames too frequent to be HTTP requests: typing indicators, cursor positions. |

**Start with SSE.** It covers chat delivery completely. A WebSocket is the later
step and changes only the transport: `Conn.frames()` yields text and knows
nothing about HTTP, and the hub, the broker and the resume protocol stay as
they are. Open one stream per page, carrying every channel the page shows.

---

## Connection Lifecycle

```python
# app/platform/realtime.py
class Conn:
    def __init__(self, user_id, org_id, channel_ids, cursor, cfg):
        self.user_id, self.org_id = user_id, org_id
        self.channels = frozenset(channel_ids)
        self.cursor = {c: cursor.get(c, 0) for c in self.channels}   # last seq sent
        self.pending: dict[int, int] = {}      # the buffer -- see Backpressure
        self._lock, self._wake = threading.Lock(), threading.Event()
        self.closed = False
        self.ping_every = cfg['REALTIME_PING_EVERY']
        # Jittered, so streams opened together do not all expire together.
        self.expires_at = time.monotonic() + cfg['REALTIME_MAX_AGE'] * random.uniform(0.8, 1.0)

    def notify(self, pointer):
        """Never blocks, never grows: a newer seq replaces the older one."""
        with self._lock:
            if pointer.seq > self.pending.get(pointer.channel_id, 0):
                self.pending[pointer.channel_id] = pointer.seq
        self._wake.set()

    def close(self):
        self.closed = True
        self._wake.set()

    def frames(self):
        yield f'retry: {random.randint(1000, 5000)}\n\n'     # jittered reconnect delay
        while True:
            woken = self._wake.wait(timeout=self.ping_every)
            if self.closed or time.monotonic() > self.expires_at:
                return                                       # the client reconnects
            if not woken:
                yield 'event: ping\ndata: {}\n\n'
                continue
            with self._lock:
                batch, self.pending = self.pending, {}
                self._wake.clear()
            for channel_id, seq in sorted(batch.items()):
                if seq <= self.cursor[channel_id]:
                    continue                                 # already told
                self.cursor[channel_id] = seq
                yield (f'id: {format_cursor(self.cursor)}\nevent: message\n'
                       f'data: {json.dumps({"channel_id": channel_id, "seq": seq})}\n\n')
```

```python
# app/config.py  (Config)
REALTIME_PING_EVERY = 25       # seconds; below the idle timeout of every proxy in the path
REALTIME_WRITE_WAIT = 10       # seconds one socket write may block
REALTIME_MAX_AGE = 900         # seconds; then the stream ends and the client reconnects
REALTIME_POLL_INTERVAL = 0.5   # seconds; SQLite broker only
```

**Authenticate at connect, before the stream starts.** The request is ordinary
HTTP and carries the session cookie, because Caddy serves the gateway on the
app's own origin (which also satisfies `default-src 'self'` in
[core/security.md](core/security.md#content-security-policy)). Refuse with 401
before a `Conn` exists.

**Let go of the request before streaming.** Flask tears the request down when
the view returns, before the first frame is written, and the database
connection goes back to the pool: 30 open streams held none. So `frames()`
never touches `db.session`, `current_user` or `g`, and the response is never
wrapped in `stream_with_context`.

**Re-authenticate on expiry.** A stream open all day outlives its session. At
`expires_at` it ends, the browser reconnects, and authentication and the
subscribe check run again.

**Heartbeat.** SSE has no pong. The server writes a `ping` when a stream has
been quiet for 25 seconds: it keeps proxies from closing the stream, and a
write to a client that has gone fails, which is how the gateway finds out
(within two pings). The client cannot ping back; its protection against a
stream that looks alive and is frozen is the timed fetch in
[Reconnection and Resume](#reconnection-and-resume).

---

## Ordering

**Every event carries a server-assigned sequence number, scoped to its
channel.** Not a timestamp: clocks go backwards, and two messages in one
millisecond are common.

```python
# app/models/channel.py
class Channel(OrgScoped, BaseModel):
    __tablename__ = 'channel'
    last_seq = db.Column(db.BigInteger, nullable=False, default=0)

    def next_seq(self) -> int:
        """The row lock serialises writers per channel until COMMIT."""
        return db.session.execute(
            sa.update(Channel).where(Channel.id == self.id)
            .values(last_seq=Channel.last_seq + 1)
            .returning(Channel.last_seq)).scalar_one()


class Message(OrgScoped, BaseModel):
    __tablename__ = 'message'
    channel_id = db.Column(BigIntFK, db.ForeignKey('channel.id', ondelete='CASCADE'), nullable=False)
    seq = db.Column(db.BigInteger, nullable=False)       # per-channel, gapless
    body = db.Column(db.Text, nullable=False)

    __table_args__ = (
        db.UniqueConstraint('channel_id', 'seq', name='uq_message_channel_seq'),
        {'sqlite_autoincrement': True},     # ids are never reused: the poller's cursor
    )

    @classmethod
    def post(cls, channel, body):
        with transaction():
            msg = cls(org_id=channel.org_id, channel_id=channel.id,
                      seq=channel.next_seq(), body=body)
            db.session.add(msg)
            broker().publish(Pointer(channel.org_id, channel.id, msg.seq))
        return msg
```

Two more model methods are used below: `Channel.heads(ids)` selects
`{id: last_seq}` for those ids, and `Message.after(channel, seq, limit=200)`
returns the channel's rows with a greater `seq`, in order.

Take the number from the counter on the channel row, not from `MAX(seq) + 1`.
Under eight concurrent writers on PostgreSQL the `MAX` form lost 478 of 800
inserts to the unique constraint; the counter kept all 800, gapless, on both
engines. A rolled-back write gives its number back. Load the channel through
the tenant-scoped query first, because the `UPDATE` is not filtered.

A global sequence does not work: a client that gets seq 41 having last seen 39
knows it missed 40 only if the numbers belong to the channel it is reading.

---

## Fanout Across Processes

A write handled by a main-app worker must reach a stream held by a gateway
worker. The path between them is one small interface with two implementations:

```python
# app/platform/realtime.py
@dataclass(frozen=True)
class Pointer:
    org_id: int
    channel_id: int
    seq: int


class Broker(Protocol):
    def publish(self, pointer: Pointer) -> None: ...   # inside the writer's transaction
    def listen(self) -> Iterator[Pointer]: ...         # blocks; one thread per gateway process


class PostgresBroker:
    def __init__(self, app):
        url = sa.engine.make_url(app.config['DATABASE_URL']).set(drivername='postgresql')
        self.dsn = url.render_as_string(hide_password=False)

    def publish(self, pointer):
        db.session.execute(sa.text("SELECT pg_notify('realtime', :p)"),
                           {'p': json.dumps(asdict(pointer))})

    def listen(self):
        import psycopg
        with psycopg.connect(self.dsn, autocommit=True) as conn:    # never from the pool
            conn.execute('LISTEN realtime')
            for note in conn.notifies():
                yield Pointer(**json.loads(note.payload))


class PollingBroker:
    """SQLite: the committed row is the publication."""
    def __init__(self, app, model):
        with app.app_context():
            self.engine = db.engine
        self.t, self.interval = model.__table__, app.config['REALTIME_POLL_INTERVAL']

    def publish(self, pointer):
        pass

    def listen(self):
        t = self.t
        with self.engine.connect() as conn:
            last = conn.execute(sa.select(sa.func.coalesce(sa.func.max(t.c.id), 0))).scalar()
        while True:
            with self.engine.connect() as conn:
                rows = conn.execute(
                    sa.select(t.c.id, t.c.org_id, t.c.channel_id, t.c.seq)
                    .where(t.c.id > last).order_by(t.c.id).limit(500)).all()
            for row in rows:
                last = row.id
                yield Pointer(row.org_id, row.channel_id, row.seq)
            if not rows:
                time.sleep(self.interval)


def init_realtime(app):
    from app.models.channel import Message
    app.extensions['realtime_broker'] = (
        PostgresBroker(app) if app.config['IS_POSTGRES'] else PollingBroker(app, Message))


def broker() -> Broker:
    return current_app.extensions['realtime_broker']
```

- **`LISTEN`/`NOTIFY`** adds nothing to run, and PostgreSQL sends at commit and
  drops on rollback. It needs a connection of its own, and a `NOTIFY` nobody is
  listening for is gone.
- **Polling by id** is correct only where ids commit in order. SQLite's single
  writer guarantees that; PostgreSQL does not, so never poll there.

Both are lossy and **neither needs to be reliable**, because of
[The Central Rule](#the-central-rule).
[core/portability.md](core/portability.md#what-is-not-portable) says not to
build on `LISTEN`/`NOTIFY` if you may stay on SQLite; the interface is what
makes it safe, and a SQLite deployment is one server, so polling is enough.

**Publish the pointer, not the payload.** A frame says `{channel_id, seq}` and
the client reads the body over HTTP. Nobody is pushed content they may no
longer see, because the read checks again.

**Publish inside the transaction.** Announcing before commit sends clients to
fetch a row that does not exist yet.

The hub is the in-process half: `_by_channel`, a dict of channel id to a set of
`Conn`, behind one lock. `add`, `remove` and `conns` are the plain operations
on it. `ensure_listening(broker)` starts `_run` on a daemon thread once per
process, on the first subscribe, so it starts after Gunicorn forks.

```python
# app/platform/realtime.py  (Hub)
    def publish(self, pointer):
        with self._lock:                        # copy under the lock, send outside it
            targets = [c for c in self._by_channel.get(pointer.channel_id, ())
                       if c.org_id == pointer.org_id]
        for conn in targets:
            conn.notify(pointer)

    def _run(self, broker):
        while True:
            try:
                for pointer in broker.listen():
                    self.publish(pointer)
            except Exception as e:
                log.error('realtime_broker_lost', error=str(e))
            for conn in self.conns():           # we were deaf: make every client
                conn.close()                    # reconnect and catch up by cursor
            time.sleep(random.uniform(1, 5))
```

---

## Reconnection and Resume

1. The page is rendered over HTTP and knows the last `seq` it shows.
2. It opens the stream with that cursor: `?channel=7&cursor=7:41`. On a
   reconnect the browser sends the last `id:` it received as `Last-Event-ID`,
   in the same format, and that wins.
3. The gateway registers the connection, then compares the cursor with each
   channel's `last_seq`, and sends the head where the client is behind.
4. On any frame the client fetches `GET /channels/7/messages?after_seq=41`.
   A frame only says something is newer, so a gap needs no special case.

```python
# app/controllers/realtime.py
@bp.route('/stream')
def stream():
    if not current_user.is_authenticated:
        abort(401)
    wanted = set(request.args.getlist('channel', type=int))
    if g.org is None or not 0 < len(wanted) <= MAX_CHANNELS:        # 50
        abort(404)
    if len(Channel.readable_by(current_user, wanted)) != len(wanted):
        abort(404)                              # one refusal refuses the stream

    cursor = parse_cursor(request.headers.get('Last-Event-ID') or request.args.get('cursor'))
    conn = Conn(current_user.id, g.org.id, wanted, cursor, current_app.config)
    hub.ensure_listening(broker())
    hub.add(conn)                               # BEFORE reading heads: nothing slips between
    try:
        for channel_id, head in Channel.heads(wanted).items():
            if head > conn.cursor[channel_id]:
                conn.notify(Pointer(g.org.id, channel_id, head))
        # Gunicorn exposes the client socket: bound how long one write may block.
        request.environ['gunicorn.socket'].settimeout(current_app.config['REALTIME_WRITE_WAIT'])
    except Exception:
        hub.remove(conn)
        raise

    response = Response(conn.frames(), mimetype='text/event-stream',
                        headers={'Cache-Control': 'no-cache'})
    response.call_on_close(lambda: hub.remove(conn))    # runs even if no frame was written
    return response
```

`parse_cursor('7:41,9:12')` returns `{7: 41, 9: 12}` and ignores anything that
is not `digits:digits`; `format_cursor` is its inverse. `readable_by(user, ids)`
is a tenant-scoped query for the channels among `ids` the user may read.

The page needs no JavaScript of its own. Vendor the extension beside htmx
(`curl -o app/static/js/htmx-ext-sse.js https://unpkg.com/htmx-ext-sse@2.2.2/sse.js`)
and load it after htmx in the layout:

```html
<!-- app/views/channels/show.html -->
<div hx-ext="sse" sse-connect="/realtime/stream?channel={{ channel.id }}&cursor={{ channel.id }}:{{ cursor }}">
  <ul id="messages" hx-get="/channels/{{ channel.id }}/messages" hx-include="#cursor"
      hx-trigger="sse:message, every 60s" hx-swap="beforeend" hx-sync="this:queue last">
    {% include 'channels/_messages.html' %}
  </ul>
</div>
<input type="hidden" id="cursor" name="after_seq" value="{{ cursor }}">
```

The read, in the main app, loads the channel with `db.get_or_404` so the tenant
filter decides, and returns `Message.after(channel, after_seq)` rendered with
the partial, or an empty 204 when there is nothing new (htmx then swaps
nothing). With rows, the partial also renders the hidden input again with the
new `seq` and `hx-swap-oob="true"`, so the cursor moves with the rows.

- **`hx-sync="this:queue last"`** keeps two hints from fetching the same cursor
  twice. A burst of 30 messages appended exactly 30 rows, in order.
- **`every 60s`** is the path that does not depend on the stream: with the
  gateway stopped, new rows still arrive. Keep the read cheap, as
  [core/performance.md](core/performance.md) asks of a page that refreshes
  itself.
- **Reconnects are spread out** by the random `retry:` the stream opens with.
  If a reconnect meets Caddy's 502 while the gateway restarts, the browser
  gives up and the extension opens a new stream with its own backoff. That one
  carries the page's original cursor, so the gateway sends the head, and the
  fetch, which reads the cursor in the page, returns 204.

A client that is not HTMX follows the same rule: on a frame with
`seq > last + 1` fetch from `last`; with `seq <= last` ignore it.

---

## Backpressure

A slow consumer must not block the writer, and its buffer must not grow.

**The buffer is one slot per subscribed channel** (`Conn.pending`, written by
`notify`), so its bound is `MAX_CHANNELS`. A frame is a pointer, and the newest
pointer makes the older ones redundant. The usual bounded queue that drops the
connection when full is wrong here: a burst of 6,000 rows in 2.5 seconds
overflowed a 64-frame queue and evicted a healthy client. With slots the same
burst reached it as about 20 frames.

**Never block while holding the hub lock.** `publish` copies the targets under
the lock and notifies outside it.

**A stalled write is cut off.** A write that blocks for `REALTIME_WRITE_WAIT`
raises, Gunicorn logs `Socket error processing request` and closes the socket,
and `call_on_close` removes the connection. Frames are about 60 bytes and, in
testing, the kernel took roughly 470 KB before a write blocked, so a client
that silently vanished is more often ended by `REALTIME_MAX_AGE`. Until then it
costs one thread.

---

## Presence

Presence is short-lived and worthless when stale. **It does not go in the
database**: a row per user rewritten every few seconds is a write storm, and on
SQLite it competes for the single writer.

The hub already knows:
`{c.user_id for c in hub.conns() if c.org_id == g.org.id}`. Serve that from the
gateway as `GET /realtime/presence`, authenticated like the stream, and let
pages ask on a timer (`hx-trigger="every 30s"`). Do not push each change to
everyone in an organization: 200 people arriving at nine is 40,000 frames.

This is presence **in one process**. With `-w 1` that is everyone. A second
gateway process sees only its own connections, and then you need a shared store
with expiry; that is the point to add Redis.

---

## Authorization

**Check on subscribe, and check again on read.** The read is the security
boundary: it goes through the tenant filter in
[tenancy.md](tenancy.md#safe-by-default-scoping) on every request. The
subscribe check keeps strangers from holding threads and from learning that a
channel is busy.

- Topics are checked against the session, never taken from the client. The
  client names channel ids; the stream opens only if the tenant-scoped query
  returns every one, and `Hub.publish` matches `org_id` again.
- A removed member loses the read immediately and the stream within
  `REALTIME_MAX_AGE`, when the subscribe check runs again. Until then they
  receive `{channel_id, seq}` and nothing else. The gateway is another process
  and the SQLite broker cannot tell it sooner; lower the age if fifteen minutes
  of that is too much.
- With `tenancy: personal` there is no `org_id`: drop it from `Pointer` and
  `Conn`, and `readable_by` checks ownership by `user_id`.

---

## Testing

The hub and the connection need no network and no server:

```python
# tests/test_realtime.py
CFG = {'REALTIME_PING_EVERY': 0.05, 'REALTIME_MAX_AGE': 900}


def test_burst_is_coalesced_and_the_gap_is_visible():
    hub, conn = Hub(), Conn(1, 1, [7], {7: 4}, CFG)
    hub.add(conn)
    for seq in range(5, 5005):
        hub.publish(Pointer(1, 7, seq))                  # never blocks
    assert conn.pending == {7: 5004}                     # one slot, not 5000 frames
    frames = conn.frames()
    assert next(frames).startswith('retry:')
    assert 'id: 7:5004' in next(frames)                  # 4 -> 5004: the client refetches
```

Worth testing explicitly:

- Another organization's connection, and another channel's, receive nothing.
- Two connections for one user both receive.
- A connection past `expires_at`, and a closed one, end the stream.
- `/realtime/stream` answers 401 without a session and 404 for a channel in
  another organization, alone or mixed with one the user may read.
- `seq` is gapless and unique per channel under concurrent writers. Run it
  against PostgreSQL with real threads; SQLite cannot fail this test.
- The read returns exactly the rows after the cursor, and 204 when there are none.

Load-test the gateway early with idle streams, not busy ones. The failure is
every thread taken by connections doing nothing.

---

## Operating It

The gateway is a second systemd unit, `yourapp-gateway.service`: a copy of the
unit in [core/deployment.md](core/deployment.md#production-setup) without
`ExecStartPre` (the main unit owns migrations), with `LimitNOFILE=65536` (one
stream is one file descriptor, and the default of 1024 leaves little room above
500 streams) and the Gunicorn command from [Process Shape](#process-shape) as
`ExecStart`. Caddy puts it on the app's origin:

```
# /etc/caddy/Caddyfile
yourapp.com {
    encode zstd gzip
    reverse_proxy /realtime/* localhost:8010
    reverse_proxy localhost:8000
    import be_right_back
}
```

Caddy needs no flush setting: frames passed through `encode` as they were
written. `scripts/deploy.sh` restarts both units. With `staging: true` the
staging copy gets its own gateway unit and port; with `deploy: docker` the
gateway is a second service on the same image, and the operator's proxy routes
`/realtime/*` to it.

**Deploys.** On `SIGTERM` Gunicorn stops accepting and ends open streams after
`--graceful-timeout`. At the default a restart takes 30 seconds; at 3 the
streams end and each browser returns after its own `retry:` delay, so the new
process is not hit by everyone at once.

| Watch | For |
|---|---|
| `connections` in the gateway's `/health` | Approaching `-w` × `--threads`. At the ceiling new streams hang. |
| `realtime_broker_lost` in the log | The listener lost the database. Every stream was closed and reconnected. |
| `Socket error processing request` in the log | Writes timing out after `REALTIME_WRITE_WAIT`: stalled clients. |
| Stream requests in the Caddy access log | A spike is a restart loop, or a proxy idle timeout below `REALTIME_PING_EVERY`. |

---

## Checklist

- [ ] `realtime: true` recorded; `init_realtime(app)` in both factories
- [ ] Gateway is its own unit and port, `gthread`, never the main app, with no route that writes
- [ ] `--threads` above the expected number of open streams; `LimitNOFILE` raised
- [ ] Caddy routes `/realtime/*` to the gateway on the app's own origin
- [ ] `seq` comes from the channel's counter row and `broker().publish()` carries ids only, both inside the write's transaction
- [ ] On PostgreSQL, `psycopg` is installed and the listener has its own connection
- [ ] The stream handler authenticates, checks every channel, and registers before reading heads
- [ ] `frames()` touches no `db.session`, `current_user` or `g`
- [ ] The page fetches by cursor on every hint and on a timer; the read is tenant-scoped and returns 204 when nothing is new
- [ ] `scripts/deploy.sh` restarts the gateway too

---

**Next:** [HTMX](core/htmx.md) | [Deployment](core/deployment.md) | [Tenancy](tenancy.md) | [Jobs](jobs.md)
