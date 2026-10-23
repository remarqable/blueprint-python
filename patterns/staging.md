# Staging

> A second, complete copy of the application on the same VPS, at
> `staging.<domain>`: its own checkout, process, port, database and settings.
> Caddy routes by subdomain. Every change runs there first; production gets
> only what already worked on staging.

Read this layer only when `staging: true`. It requires `deploy: vps`: a
distributed Docker product has no server of yours to stage on.

---

## Table of Contents

- [Why a second copy](#why-a-second-copy)
- [The shape](#the-shape)
- [Server layout](#server-layout)
- [Settings](#settings)
- [What the app does differently](#what-the-app-does-differently)
- [systemd](#systemd)
- [Caddy and DNS](#caddy-and-dns)
- [The database](#the-database)
- [Refreshing staging from production](#refreshing-staging-from-production)
- [Deploying](#deploying)
- [Migrations on a shared branch history](#migrations-on-a-shared-branch-history)
- [Limits of one server](#limits-of-one-server)
- [Checklist](#checklist)

---

## Why a second copy

`make run` on a laptop proves the code runs. It does not prove it runs on the
server, against the server's Postgres, its data volume, its Caddy, its env, or
real data at real scale. Staging does, without touching production:

- **A branch can be tried in the real environment** before it is merged.
- **Risky work** (a migration, a pipeline change, a new provider) is run there
  first, against a copy of production's data.
- **People who aren't engineers can try it** at a real URL, signed in as
  themselves, before it ships.

One VPS is enough. A second process and a second database cost a few hundred
MB of RAM; a second server costs money and drifts from the first. Move staging
to its own server only when it competes with production for CPU (see
[Limits of one server](#limits-of-one-server)).

---

## The shape

Everything is doubled except the server, Postgres itself and Caddy.

| | Production | Staging |
|---|---|---|
| URL | `yourapp.com` | `staging.yourapp.com` |
| Checkout | `/opt/yourapp` (always `main`) | `/opt/yourapp-staging` (any branch) |
| Service | `yourapp` → `127.0.0.1:8000` | `yourapp-staging` → `127.0.0.1:8001` |
| Workers (`jobs: true`) | `yourapp-worker` | `yourapp-staging-worker` |
| Database | `yourapp` | `yourapp_staging` (same Postgres server) |
| Settings | `/opt/yourapp/.env` | `/opt/yourapp-staging/.env` (`STAGING=1`) |
| Files (`uploads: true`) | `/opt/yourapp/data` | `/opt/yourapp-staging/data` |
| Who can open it | everyone | platform admins only |

**Same repository, second checkout.** Staging is never a fork or a copy of the
code. It is the same repo checked out at whatever branch is being tried, so
"it worked on staging" means "this commit works".

**Nothing is shared at runtime.** Two processes never read the other's
database, files, sessions or job queue. A bug on staging cannot write to
production, because staging holds no credential that can.

---

## Server layout

```
/opt/yourapp/              # production: main
├── .env
├── .venv/
└── data/
/opt/yourapp-staging/      # staging: the branch being tried
├── .env
├── .venv/
└── data/
```

Both are owned by the same app user (`yourapp`). Provision staging once, after
production:

```bash
sudo -u yourapp git clone https://github.com/yourorg/yourapp.git /opt/yourapp-staging
cd /opt/yourapp-staging
sudo -u yourapp uv sync --locked --no-dev
sudo -u yourapp mkdir -p data
sudo -u yourapp cp /opt/yourapp/.env .env      # then edit: see Settings
chmod 600 .env
```

---

## Settings

Staging's `.env` starts as a copy of production's and changes these lines.
Every one of them matters; most incidents with staging come from one left as
production's value.

```bash
APP_ENV=staging
STAGING=1
PORT=8001
BASE_DOMAIN=staging.yourapp.com
SECRET_KEY=<a new one: secrets.token_hex(32)>   # never production's
DATABASE_URL=postgresql+psycopg://yourapp_staging:<password>@localhost/yourapp_staging
DATA_DIR=/opt/yourapp-staging/data

# Outbound side effects: off, or pointed somewhere safe
MAIL_REDIRECT_TO=team@yourapp.com      # every email goes here instead (see below)
SMS_ENABLED=false
WEBHOOKS_ENABLED=false
ANALYTICS_ENABLED=false

# Paid providers: separate keys with their own low limits, or fixtures
OPENAI_API_KEY=<staging key>
```

- **A different `SECRET_KEY`.** Sharing it would let a staging session cookie
  sign in to production.
- **Its own database user**, with rights on `yourapp_staging` only. Staging
  must be *unable* to connect to the production database, not merely
  configured not to.
- **Outbound side effects off.** Once staging holds a copy of production data,
  it holds real customers' email addresses and phone numbers. A job that
  sends reminders will send them, from staging, to real people. Redirect all
  mail to the team and switch off SMS, webhooks and payment capture.
- **Paid providers with their own keys** and spending limits, so a runaway
  test is capped and billed separately.

```python
# app/config.py
STAGING = os.environ.get('STAGING', '') == '1'
MAIL_REDIRECT_TO = os.environ.get('MAIL_REDIRECT_TO')
```

```python
# app/platform/mail.py -- the one place mail is sent
def send(to: str, subject: str, body: str) -> None:
    redirect = current_app.config['MAIL_REDIRECT_TO']
    if redirect:
        subject = f'[staging → {to}] {subject}'
        to = redirect
    _transport.send(to, subject, body)
```

---

## What the app does differently

One flag, `STAGING`, read in one place each. Never `if request.host ==
'staging...'` scattered through controllers.

**Admins only.** Staging may hold a copy of production data; the public must
not reach it, and search engines must not index it.

```python
# app/middleware/staging.py -- registered in create_app when STAGING is on
OPEN = {'auth.login', 'auth.verify', 'auth.logout', 'main.health', 'static'}

def register_staging_gate(app):
    if not app.config['STAGING']:
        return

    @app.before_request
    def _admins_only():
        if request.endpoint in OPEN:
            return None
        if not current_user.is_authenticated:
            return redirect(url_for('auth.login', next=request.full_path))
        if not current_user.is_platform_admin:
            abort(403)
        return None

    @app.after_request
    def _noindex(resp):
        resp.headers['X-Robots-Tag'] = 'noindex, nofollow'
        return resp
```

- **`robots.txt` disallows everything**, and the sitemap is not served.
  Caddy also sends `X-Robots-Tag` (below), so even the login page is covered.
- **A bar on every page** ("Staging: changes here don't affect the live
  site"), so nobody mistakes the tab they are in. The layout shows it when
  `config.STAGING` is set.
- **Host-only cookies, with their own names.** Never set
  `SESSION_COOKIE_DOMAIN=.yourapp.com`: a parent-domain cookie is sent to both
  sites. Give staging its own `SESSION_COOKIE_NAME` (`yourapp_staging`) so
  signing in to one never signs you out of the other.
- **Absolute URLs use `BASE_DOMAIN`** (emails, canonical links, OAuth
  callbacks), so a link generated on staging points at staging.
- **OAuth providers** need staging's callback URL registered as a second
  redirect URI, or a separate staging app with its own client ID.

---

## systemd

The units are production's with the paths, port and name changed.

```ini
# /etc/systemd/system/yourapp-staging.service
[Unit]
Description=YourApp (staging)
After=network.target postgresql.service

[Service]
User=yourapp
Group=yourapp
WorkingDirectory=/opt/yourapp-staging
Environment="PATH=/opt/yourapp-staging/.venv/bin"
EnvironmentFile=/opt/yourapp-staging/.env
ExecStartPre=/opt/yourapp-staging/.venv/bin/flask db upgrade
ExecStart=/opt/yourapp-staging/.venv/bin/gunicorn wsgi:app -w 2 -b 127.0.0.1:8001
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

- **Fewer workers** than production (`-w 2`): staging serves a handful of
  people.
- **With `jobs: true`**, a `yourapp-staging-worker.service` runs
  `flask jobs run` from the staging checkout, against the staging database.
  Keep its concurrency low; it shares the CPU with production's workers.

---

## Caddy and DNS

**DNS:** `staging.yourapp.com` points at the same server as `yourapp.com`, as
an `A` record to the server's IP or a `CNAME` to `yourapp.com`. Caddy gets
its certificate on the first request once the name resolves.

```
# /etc/caddy/Caddyfile
yourapp.com {
    reverse_proxy 127.0.0.1:8000
    import be_right_back
}

staging.yourapp.com {
    encode gzip
    header X-Robots-Tag "noindex, nofollow"
    reverse_proxy 127.0.0.1:8001
    import be_right_back
}
```

Routing is by host name alone: no path prefixes and no app-side host
checks. Both sites share the `be_right_back` snippet from
[deployment.md](core/deployment.md#be-right-back-page).

```bash
caddy validate --config /etc/caddy/Caddyfile && systemctl reload caddy
```

---

## The database

A separate database on the same Postgres server, owned by its own user:

```sql
CREATE USER yourapp_staging WITH PASSWORD '...';
CREATE DATABASE yourapp_staging OWNER yourapp_staging;
REVOKE CONNECT ON DATABASE yourapp FROM yourapp_staging;
```

With `database: sqlite` the same holds with a file: `data/app.db` under the
staging checkout, never a path into production's directory.

---

## Refreshing staging from production

Staging is most useful with production's data, and that data goes stale. A
refresh replaces the staging database with a copy of production's, then
makes the copy safe to run.

```bash
# scripts/refresh-staging.sh -- run on the server, as root
set -euo pipefail
STAMP=$(date +%Y%m%d-%H%M)

systemctl stop yourapp-staging 'yourapp-staging-worker*'

# Keep the old staging database: a refresh must be undoable.
sudo -u postgres psql -c "ALTER DATABASE yourapp_staging RENAME TO yourapp_staging_$STAMP"
sudo -u postgres createdb -O yourapp_staging yourapp_staging
sudo -u postgres pg_dump -Fc --no-owner --no-acl yourapp \
  | sudo -u postgres pg_restore --no-owner --role=yourapp_staging -d yourapp_staging

# Make it safe: nothing queued in production runs again here.
sudo -u postgres psql -d yourapp_staging <<'SQL'
DELETE FROM job WHERE status IN ('queued', 'running');
DELETE FROM user_session;               -- production's sign-ins don't carry over
SQL

systemctl start yourapp-staging 'yourapp-staging-worker*'
echo "Staging refreshed. Previous copy: yourapp_staging_$STAMP"
```

- **Rename, don't drop.** Staging may hold work of its own (test results,
  experiments). The renamed copy can be put back with two `ALTER DATABASE ...
  RENAME` statements, and dropped once nobody needs it.
- **Clear the job queue.** A copied `queued` job would run a second time from
  staging: a second email, a second charge, a second crawl.
- **Scrub what staging must not use.** Clear payment tokens, API keys users
  stored, and anything else that would let staging act on a customer's
  behalf. Keep the list next to the script and add to it with every table
  that holds such data.
- **Files** (`uploads: true`): copy `data/` with `rsync -a --delete`, or point
  staging at production's public files read-only. Never let staging write
  into production's storage.
- **Run it on purpose**, not on a timer. A refresh discards whatever was being
  tried on staging.

---

## Deploying

Two scripts, the same steps, different targets.

```bash
scripts/deploy-staging.sh            # main
scripts/deploy-staging.sh my-branch  # a branch being tried (pushed first)
scripts/deploy.sh                    # production: main only
```

```bash
#!/bin/bash
# scripts/deploy-staging.sh -- deploy a branch to staging; production untouched
set -euo pipefail
REMOTE="root@yourapp.com"
DIR=/opt/yourapp-staging
BRANCH="${1:-main}"

git push -q origin "$BRANCH"
ssh "$REMOTE" "set -e; cd $DIR
  sudo -u yourapp git fetch -q origin
  sudo -u yourapp git checkout -q $BRANCH
  sudo -u yourapp git reset -q --hard origin/$BRANCH
  sudo -u yourapp sh -c 'git rev-parse --short HEAD > .git_sha'
  sudo -u yourapp uv sync --locked --no-dev -q
  systemctl restart yourapp-staging 'yourapp-staging-worker*'
  sleep 3; systemctl is-active yourapp-staging"
curl -fsS https://staging.yourapp.com/health && echo
```

`git reset --hard origin/$BRANCH`, not `git pull`: staging switches branches
and a branch may have been rebased. The checkout is never edited by hand.

**The flow:**

1. Work on a branch.
2. `deploy-staging.sh my-branch`, then try it at `staging.yourapp.com`, on
   real data if the change depends on it.
3. Merge to `main`, deploy `main` to staging once more (a merged `main` is a
   new commit), then `deploy.sh` to production.
4. Leave staging on `main` when nothing is being tried, so the next person
   starts from what production runs.

**An optional gate:** `deploy.sh` can refuse a commit staging hasn't run.
Staging's `/health` reports the commit it runs, so the production deploy
compares it with `HEAD` and stops unless they match (or `FORCE=1` is set for
an emergency fix).

```bash
STAGED=$(curl -fsS https://staging.yourapp.com/health | jq -r .version)
if [ "$STAGED" != "$(git rev-parse --short HEAD)" ] && [ "${FORCE:-}" != 1 ]; then
  echo "Staging runs $STAGED, not this commit. Deploy it there first (or FORCE=1)."
  exit 1
fi
```

---

## Migrations on a shared branch history

Staging runs `flask db upgrade` on every deploy (`ExecStartPre`), so a
branch's migration is applied to the staging database when the branch is
deployed there. That has two consequences.

- **Switching staging back does not undo the migration.** Before deploying
  another branch, either downgrade (`flask db downgrade <revision>`) or
  [refresh from production](#refreshing-staging-from-production).
  Otherwise Alembic finds a revision in the database that the checked-out
  code doesn't know, and the service fails to start.
- **Two branches with migrations have two heads.** Merging them needs a merge
  revision (`flask db merge heads`). Check `flask db heads` shows one head
  before deploying to either site.

Staging catches a migration that fails on real data (a `NOT NULL` column on a
table with rows, a unique index over duplicates) before production does. That
is one of the main reasons to have it: refresh staging, deploy the branch, and
watch the migration run against production's data first.

---

## Limits of one server

- **CPU and memory are shared.** A heavy job on staging slows production.
  Keep staging's workers few, and run load tests somewhere else.
- **One Postgres.** A long query or a big restore on staging competes for
  production's I/O. Schedule refreshes for quiet hours.
- **Not a backup.** Staging is not backed up. Production's backups are
  unchanged by any of this.
- **Graduate** to a separate server when staging needs production's size to
  be a fair test, or when its load becomes noticeable. The pattern is the same
  with a different `REMOTE`, and the database refresh goes over the network
  (`pg_dump -h`).

---

## Checklist

- [ ] `staging.yourapp.com` resolves to the server; Caddy serves it with
      `X-Robots-Tag: noindex`
- [ ] `/opt/yourapp-staging` is a checkout of the same repo; `yourapp-staging`
      listens on `127.0.0.1:8001`
- [ ] Its own database and database user, unable to connect to production's
- [ ] Its own `SECRET_KEY` and session cookie name; cookies host-only
- [ ] `STAGING=1`: admins only, a staging bar on every page, robots.txt
      disallows all
- [ ] Mail redirected, SMS/webhooks/payments off, paid providers on separate
      keys
- [ ] `deploy-staging.sh [branch]` works; `deploy.sh` deploys only `main`
- [ ] `refresh-staging.sh` keeps the old database and clears the job queue
