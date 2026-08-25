# Blueprint-Python

> A documentation blueprint for building production-ready Flask SaaS
> applications — MVC, multi-tenancy, plugins, theming, i18n, and deployment.

Point an AI agent (or a developer) at this repo and get a consistent, working
Flask application instead of a pile of plausible-looking code.

## What This Is

Documentation, not a framework. There is no package to install and no code to
import — just a set of patterns precise enough to build from, with the failure
modes called out where they bite.

## Quick Start

Point Claude at [CLAUDE.md](CLAUDE.md). It will ask five configuration questions,
record the answers, and build from there.

```bash
git submodule add https://github.com/remarqable/blueprint-python.git blueprint
# then in your project's CLAUDE.md:
#   See blueprint/CLAUDE.md for architecture patterns.
```

## Structure

One architecture, plus layers you opt into. Read all of `core/`; read a layer
only when its condition holds.

```
blueprint-python/
├── CLAUDE.md                      # master blueprint — start here
├── patterns/
│   ├── core/                      # applies to every project
│   │   ├── mvc.md                 # models, views, controllers
│   │   ├── database.md            # SQLAlchemy conventions, migrations
│   │   ├── portability.md         # SQLite ↔ PostgreSQL
│   │   ├── auth.md                # magic links, OAuth, sessions
│   │   ├── security.md            # CSRF, rate limiting, checklist
│   │   ├── i18n.md   htmx.md   frontend.md   mobile-navigation.md
│   │   ├── testing.md   typing.md   audit.md
│   │   └── deployment.md          # systemd + Gunicorn + Caddy
│   ├── tenancy.md                 # layer: tenancy: shared
│   ├── plugins.md                 # layer: plugins: true
│   └── theming.md                 # layer: theming: true
├── scripts/                       # deploy.sh, provision-server.sh
└── Makefile
```

## Configuration

The blueprint asks five questions up front, because they change the generated
code:

| Question | Setting |
|----------|---------|
| Will data ever be shared between users? | `tenancy: shared \| personal` |
| Do tenants install optional features? | `plugins` |
| Do tenants customize the UI? | `theming` |
| Track who changed what? | `audit_logging` |
| SQLite or PostgreSQL? | `database` |

The first one is the one people get wrong. It is not "B2C or B2B?" — that asks
about your go-to-market label rather than your data model. Adding `org_id` later
means backfilling an organization per user and rewriting every query and
permission check; carrying it from day one costs one indexed column.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Python 3.11+ / Flask 3.x |
| Database | SQLite (default) → PostgreSQL, switchable via `DATABASE_URL` |
| ORM | SQLAlchemy 2.0 |
| Migrations | Alembic, applied at deploy |
| Frontend | Bootstrap 5 + HTMX, no build pipeline |
| Testing | pytest — SQLite in-memory locally, PostgreSQL in CI |
| Serving | Gunicorn + systemd + Caddy (automatic HTTPS) |

## Philosophy

- **Keep it simple, explicit, and local.** No magic.
- **Fat models, thin controllers, dumb templates.**
- **Server-rendered HTML + HTMX.** No SPA, no npm.
- **Safe by default.** Tenant isolation is enforced in one place, not remembered
  in every query.
- **SQLite by default.** PostgreSQL is one environment variable away.
- **i18n from day 1.**

## Deployment

A single VPS: systemd for process management, Gunicorn as the WSGI server, Caddy
for reverse proxy and automatic HTTPS. No Docker, no orchestration.

```bash
make deploy
```

## License

MIT — see [LICENSE](LICENSE).
