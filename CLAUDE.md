# CLAUDE.md — Flask SaaS Blueprint

> Production-ready Flask SaaS applications: MVC, multi-tenancy, plugins, i18n,
> and deployment. Optimized for fast iteration, clean code, and AI agent execution.

---

## ⚠️ READ THIS FIRST: Scoped Reading

This blueprint describes **one architecture with optional layers** — never two
alternatives to blend. Reading a layer that does not apply to the project
produces code that does not run.

1. Establish the [Project Configuration](#project-configuration) below.
2. Read **all of `patterns/core/`** — it applies to every project.
3. Read a **layer doc only if its condition is met.**

**Never read both branches of the same decision in one session.** If
`tenancy: personal`, do not open `patterns/tenancy.md` at all — not for
reference, not for context.

| Layer doc | Read only when |
|-----------|----------------|
| `patterns/tenancy.md` | `tenancy: shared` |
| `patterns/plugins.md` | `plugins: true` (requires `tenancy: shared` for the per-tenant model) |
| `patterns/theming.md` | `theming: true` |
| `patterns/storage.md` | `uploads: true` |
| `patterns/jobs.md` | `jobs: true` |

Two further decisions select *sections within* core docs rather than layer
docs: `auth` picks one strategy inside `patterns/core/auth.md`, and `deploy`
picks one shape inside `patterns/core/deployment.md`. The same rule applies —
implement the chosen branch, do not blend.

---

## 📋 Project Configuration

> **AI Agents:** Ask the questions below before creating any files, then record
> the answers here. This block is authoritative for everything that follows.

```yaml
tenancy: shared        # shared | personal
plugins: false         # true | false
theming: false         # true | false
audit_logging: false   # true | false
database: sqlite       # sqlite | postgres  (switchable later via DATABASE_URL)
auth: magic_links      # magic_links | password
uploads: false         # true | false
jobs: false            # true | false
deploy: vps            # vps | docker
```

### The questions to ask

**1. Will data ever be shared between users?**

- **Yes → `tenancy: shared`** (default). Users belong to organizations; business
  tables carry `org_id`. Covers every B2B app, and every B2C app that might one
  day add teams, sharing, or collaboration.
- **No → `tenancy: personal`.** Data belongs to one user and never to a group.
  A personal notes app, a private finance tracker.

Ask it this way rather than "B2C or B2B?" — that asks about your go-to-market
label, not your data model. Retrofitting `org_id` later means backfilling an
organization per user and rewriting every query and every permission check.
Carrying it from day one costs one indexed column. **When unsure, choose
`shared`.**

**2. Do tenants need to install optional features?** → `plugins`

**3. Do tenants need to customize the UI?** → `theming`

**4. Track who changed what?** → `audit_logging` (adds `created_by`/`updated_by`)

**5. SQLite or PostgreSQL?** → `database`. SQLite is the default and is a
one-variable switch later, provided you follow
[core/portability.md](patterns/core/portability.md). Choose PostgreSQL now only
if you already know you need high write concurrency, JSONB querying, or full-text
search.

**6. Can every login depend on outbound email?** → `auth`. Magic links are the
default and the least code — but every login sends an email. If the product
must install, authenticate, or recover accounts with **no email service
configured** (self-hostable products especially), choose `password`. See
[core/auth.md](patterns/core/auth.md#overview).

**7. Do users upload files?** → `uploads` (logos, avatars, images,
attachments). Adds the storage interface and the `Upload` model.

**8. Does work happen outside requests?** → `jobs` (email batches, image
processing, scheduled cleanup, tenant purge). Adds the DB-backed queue and a
worker process.

**9. Who runs this in production?** → `deploy`. `vps` (default) when you
operate the server; `docker` when the application is distributed for others
to run.

---

## 🚀 Quick Start

### Prerequisites
- [uv](https://docs.astral.sh/uv/) (`curl -LsSf https://astral.sh/uv/install.sh | sh`) —
  manages the virtualenv, the lockfile, and Python 3.11+ itself
- Make

### Step 1: Structure

```bash
mkdir yourapp && cd yourapp
mkdir -p app/{models,controllers,views/{layouts,partials,users,settings,errors},static/{css,js,img},lang,middleware,platform}
mkdir -p migrations/versions tests/{test_models,test_controllers} config scripts
touch app/__init__.py app/models/__init__.py app/controllers/__init__.py
touch app/middleware/__init__.py app/platform/__init__.py
```

If `plugins: true`, also `mkdir -p plugins`.
If `theming: true`, also `mkdir -p app/views/themes`.

### Step 2: Frontend assets

```bash
mkdir -p bin
make css                 # downloads the Tailwind standalone binary, builds app.css
curl -o app/static/js/htmx.min.js   https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
curl -o app/static/js/alpine.min.js https://unpkg.com/alpinejs@3.14.9/dist/cdn.min.js
echo "bin/" >> .gitignore
```

`app/static/css/input.css` holds the design tokens and component layer —
see [core/frontend.md](patterns/core/frontend.md#design-tokens). Commit the built
`app.css` so deployment stays a `git pull` with no toolchain on the server.

### Step 3: Dependencies

```bash
cat > pyproject.toml <<'EOF'
[project]
name = "yourapp"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
    "Flask",
    "Flask-SQLAlchemy",
    "Flask-Login",
    "Flask-Migrate",
    "SQLAlchemy",
    "structlog",
    "python-dotenv",
    "gunicorn",
]

[dependency-groups]
dev = ["pytest", "pytest-cov"]

[tool.uv]
package = false
EOF

make install     # uv sync: creates .venv, writes uv.lock, installs everything
```

Commit `uv.lock` — deploys run `uv sync --locked --no-dev`, so the server
installs exactly what you tested with. No `pip`, no `requirements.txt`, no
activation: `uv run <cmd>` (used by every Makefile target) resolves the
project environment from anywhere in the repo.

### Step 4: Environment

```bash
cat > config/local.env <<'EOF'
APP_ENV=dev
PORT=8000
DATABASE_URL=sqlite:///app.db
SECRET_KEY=dev-secret-change-in-production
DEV_MAGIC=true
BASE_DOMAIN=localhost
EOF
```

### Step 5: Platform layer

| File | Source |
|------|--------|
| `app/config.py` | [§ Configuration](#configuration) |
| `app/extensions.py` | [§ Extensions](#extensions) |
| `app/models/types.py` | [core/portability.md](patterns/core/portability.md#portable-column-types) |
| `app/models/base.py` | [core/database.md](patterns/core/database.md#base-model) |
| `app/platform/logger.py` | [§ Logging](#logging) |
| `app/platform/errors.py` | [§ Error Handling](#error-handling) |
| `app/platform/i18n.py` | [core/i18n.md](patterns/core/i18n.md) |
| `app/platform/tenant.py` | [tenancy.md](patterns/tenancy.md) — only if `tenancy: shared` |

### Step 6: First model, controller, view

- `app/models/user.py` → [core/mvc.md](patterns/core/mvc.md#models-fat-models)
- `app/controllers/main.py` → [core/mvc.md](patterns/core/mvc.md#controllers-thin-controllers)
- `app/views/layouts/base.html` → [core/mvc.md](patterns/core/mvc.md#views-templates)
- `app/__init__.py` → [§ Application Factory](#application-factory)

### Step 7: Run

```bash
make run       # runs `flask db upgrade`, then the dev server on :8000
```

✅ **Verify:** `curl localhost:8000/health` returns `{"status": "ok"}`

---

## Philosophy

- **Keep it simple, explicit, and local.** No magic, no over-engineering.
- **MVC**: fat models, thin controllers, dumb templates.
- **Server-rendered HTML + HTMX.** No SPA. One CSS build, run locally, never on the server.
- **Safe by default.** Tenant isolation is enforced in one place, not in every
  query. Security that depends on remembering is not security.
- **i18n from day 1.**
- **SQLite by default, PostgreSQL when you need it** — one variable, no rewrite.
- **Zero yak-shaving dev loop.** `make run` boots a working app.

### Fat models, thin controllers

**Models**: business logic, validation, queries, domain rules.
**Controllers**: parse input → call model → render or return JSON.
**Views**: Jinja2, minimal logic, HTMX attributes.

---

## Folder Structure

```
yourapp/
├── app/
│   ├── __init__.py           # application factory
│   ├── config.py
│   ├── extensions.py         # db, login_manager, migrate, SQLite pragmas
│   ├── models/
│   │   ├── base.py           # BaseModel, OrgScoped, utcnow
│   │   ├── types.py          # portable column types
│   │   ├── user.py
│   │   └── organization.py   # tenancy: shared only
│   ├── controllers/          # thin Flask blueprints
│   ├── views/                # Jinja2 (templates/ is called views/ here)
│   │   ├── layouts/  partials/  errors/
│   │   └── themes/           # theming: true only
│   ├── static/
│   │   ├── css/input.css     # Tailwind source: tokens + component layer
│   │   ├── css/app.css       # BUILT — committed, never hand-edited
│   │   └── js/               # htmx.min.js, alpine.min.js — vendored
│   ├── lang/                 # en.json, es.json
│   ├── middleware/           # auth, csrf, ratelimit
│   └── platform/             # logger, i18n, errors, tenant, plugins, theming
├── plugins/                  # plugins: true only
├── migrations/versions/
├── tests/
├── scripts/deploy.sh
├── config/local.env
├── run.py  wsgi.py  Makefile  pyproject.toml  uv.lock
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Backend | Python 3.11+ / Flask 3.x | Simple, mature, Jinja2 built-in |
| Packaging | uv | One tool for venv, lockfile, and Python installs |
| Database | SQLite → PostgreSQL | No server to start; switchable via `DATABASE_URL` |
| ORM | SQLAlchemy 2.0 | Declarative, portable across both engines |
| Migrations | Alembic (Flask-Migrate) | Run once at deploy, never in-process |
| Frontend | Tailwind CSS v4 + Alpine.js + HTMX | Utility-first, no npm, progressive enhancement |
| i18n | JSON catalogs | Simple, runtime-loaded |
| Logging | structlog | Structured JSON in production |
| Auth | Flask-Login + magic links *or* passwords | Magic links need email; passwords don't — see `auth:` |
| Testing | pytest | SQLite in-memory, PostgreSQL in CI |
| Serving | Gunicorn + systemd + Caddy, or Docker | VPS you operate vs app you distribute — see `deploy:` |

---

## Configuration

```python
# app/config.py
import os


class Config:
    APP_ENV = os.environ.get('APP_ENV', 'dev')
    SECRET_KEY = os.environ.get('SECRET_KEY', 'dev-secret-change-in-production')
    DEBUG = APP_ENV == 'dev'
    BASE_DOMAIN = os.environ.get('BASE_DOMAIN', 'localhost')

    DATABASE_URL = os.environ.get('DATABASE_URL', 'sqlite:///app.db')
    SQLALCHEMY_DATABASE_URI = DATABASE_URL
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    IS_SQLITE = DATABASE_URL.startswith('sqlite')
    IS_POSTGRES = DATABASE_URL.startswith('postgresql')

    SQLALCHEMY_ENGINE_OPTIONS = {
        'pool_size': 5, 'max_overflow': 10, 'pool_timeout': 30,
        'pool_recycle': 300, 'pool_pre_ping': True,
    } if IS_POSTGRES else {'connect_args': {'timeout': 30}}

    SESSION_COOKIE_SECURE = APP_ENV != 'dev'
    SESSION_COOKIE_HTTPONLY = True
    SESSION_COOKIE_SAMESITE = 'Lax'

    CSRF_ENABLED = True
    RUN_MIGRATIONS_ON_STARTUP = False    # migrations belong in deploy, not boot
    DEV_MAGIC = os.environ.get('DEV_MAGIC', 'false').lower() == 'true'
```

---

## Extensions

```python
# app/extensions.py
import sqlalchemy as sa
from sqlalchemy import event
from flask_sqlalchemy import SQLAlchemy
from flask_login import LoginManager
from flask_migrate import Migrate

# Named constraints are required for SQLite migrations. Set this before the
# first migration or Alembic will want to rename every constraint you have.
NAMING_CONVENTION = {
    'ix': 'ix_%(table_name)s_%(column_0_name)s',
    'uq': 'uq_%(table_name)s_%(column_0_name)s',
    'ck': 'ck_%(table_name)s_%(constraint_name)s',
    'fk': 'fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s',
    'pk': 'pk_%(table_name)s',
}

db = SQLAlchemy(metadata=sa.MetaData(naming_convention=NAMING_CONVENTION))
migrate = Migrate()
login_manager = LoginManager()
login_manager.login_view = 'auth.login'
login_manager.login_message_category = 'warning'


@login_manager.user_loader
def load_user(user_id):
    from .models.user import User
    return db.session.get(User, int(user_id))


def init_sqlite_pragmas(app):
    """SQLite enforces no foreign keys by default -- every ondelete='CASCADE'
    silently does nothing without this. No-op on PostgreSQL.
    See core/portability.md."""
    if not app.config['IS_SQLITE']:
        return

    @event.listens_for(db.engine, 'connect')
    def _pragmas(dbapi_conn, _record):
        cur = dbapi_conn.cursor()
        cur.execute('PRAGMA foreign_keys=ON')
        cur.execute('PRAGMA journal_mode=WAL')
        cur.execute('PRAGMA synchronous=NORMAL')
        cur.execute('PRAGMA busy_timeout=5000')
        cur.close()
```

---

## Application Factory

```python
# app/__init__.py
from flask import Flask
from .config import Config
from .extensions import db, migrate, login_manager, init_sqlite_pragmas
from .platform.logger import init_logger, get_logger


def create_app(config_class=Config):
    app = Flask(__name__, template_folder='views', static_folder='static')
    app.config.from_object(config_class)

    init_logger(app.config['APP_ENV'])
    log = get_logger()

    db.init_app(app)
    migrate.init_app(app, db)
    login_manager.init_app(app)

    with app.app_context():
        init_sqlite_pragmas(app)

    # Migrations are NOT run here. N Gunicorn workers would race the same
    # upgrade on boot. See core/database.md § Migrations.
    if app.config.get('RUN_MIGRATIONS_ON_STARTUP'):
        from flask_migrate import upgrade
        with app.app_context():
            upgrade()

    from .middleware.csrf import init_csrf
    init_csrf(app)

    # tenancy: shared only -- resolves g.org and installs the tenant filter
    from .platform.tenant import init_tenant
    init_tenant(app)

    from .controllers import main, auth, users, settings
    for module in (main, auth, users, settings):
        app.register_blueprint(module.bp)

    # plugins: true only -- boot-time registration, per-request tenant gating
    from .platform.plugins import load_plugins
    load_plugins(app)

    register_error_handlers(app)
    log.info('app_started', env=app.config['APP_ENV'])
    return app


def register_error_handlers(app):
    from flask import render_template

    @app.errorhandler(404)
    def not_found(error):
        return render_template('errors/404.html'), 404

    @app.errorhandler(500)
    def internal_error(error):
        db.session.rollback()
        return render_template('errors/500.html'), 500
```

Order matters: `init_tenant` before blueprints so `g.org` exists in every
`before_request`; `load_plugins` last so plugin routes register after core ones.

---

## Logging

```python
# app/platform/logger.py
import logging, sys
import structlog


def init_logger(env: str = 'dev'):
    logging.basicConfig(format='%(message)s', stream=sys.stdout, level=logging.INFO)
    renderer = (structlog.dev.ConsoleRenderer(colors=True) if env == 'dev'
                else structlog.processors.JSONRenderer())
    structlog.configure(
        processors=[
            structlog.stdlib.add_log_level,
            structlog.processors.TimeStamper(fmt='%Y-%m-%d %H:%M:%S' if env == 'dev' else 'iso'),
            renderer,
        ],
        wrapper_class=structlog.stdlib.BoundLogger,
        logger_factory=structlog.stdlib.LoggerFactory(),
    )


def get_logger():
    return structlog.get_logger()
```

---

## Error Handling

```python
# app/platform/errors.py
class AppError(Exception):
    code = 'E_UNKNOWN'
    http_status = 500

    def __init__(self, message: str = 'An unexpected error occurred'):
        self.message = message
        super().__init__(self.message)


class ValidationError(AppError):
    code, http_status = 'E_INVALID_INPUT', 400


class NotFoundError(AppError):
    code, http_status = 'E_NOT_FOUND', 404


class UnauthorizedError(AppError):
    code, http_status = 'E_UNAUTHORIZED', 401


class ForbiddenError(AppError):
    code, http_status = 'E_FORBIDDEN', 403
```

---

## 📚 Pattern Index

### Core — read all of these

| Doc | Covers |
|-----|--------|
| [core/mvc.md](patterns/core/mvc.md) | Models, views, controllers in detail |
| [core/database.md](patterns/core/database.md) | SQLAlchemy conventions, migrations, queries |
| [core/portability.md](patterns/core/portability.md) | **SQLite ↔ PostgreSQL: types, pragmas, migration** |
| [core/auth.md](patterns/core/auth.md) | Magic links, OAuth, sessions |
| [core/security.md](patterns/core/security.md) | CSRF, rate limiting, checklist |
| [core/i18n.md](patterns/core/i18n.md) | Internationalization |
| [core/htmx.md](patterns/core/htmx.md) | Interactive patterns |
| [core/frontend.md](patterns/core/frontend.md) | Tailwind v4, Alpine, component layer, a11y |
| [core/mobile-navigation.md](patterns/core/mobile-navigation.md) | Responsive navigation |
| [core/testing.md](patterns/core/testing.md) | pytest, fixtures, isolation tests |
| [core/typing.md](patterns/core/typing.md) | Type hints and mypy |
| [core/audit.md](patterns/core/audit.md) | `created_by` / `updated_by` |
| [core/deployment.md](patterns/core/deployment.md) | systemd + Gunicorn + Caddy |

### Layers — read only when the condition holds

| Doc | Condition | Covers |
|-----|-----------|--------|
| [tenancy.md](patterns/tenancy.md) | `tenancy: shared` | Organizations, resolution, automatic scoping, roles |
| [plugins.md](patterns/plugins.md) | `plugins: true` | Per-tenant installable features and routes |
| [theming.md](patterns/theming.md) | `theming: true` | Per-tenant branding and template overrides |
| [storage.md](patterns/storage.md) | `uploads: true` | File uploads, image variants, safe serving |
| [jobs.md](patterns/jobs.md) | `jobs: true` | DB-backed queue, worker process, retries |

---

## 🤖 AI Agent Instructions

**Bootstrapping a new project:**
1. Ask the [configuration questions](#the-questions-to-ask). Do not guess.
2. Record answers in [Project Configuration](#project-configuration).
3. Read all of `patterns/core/`. Read layer docs **only** where the condition holds.
4. Execute Quick Start steps 1–7 in order.
5. Verify `make run` serves `/health`.

**Adding a feature:** find the relevant pattern doc via the index above and
follow it. Do not invent a second way to do something the blueprint already
covers.

**Non-negotiables:**
- `BigIntPK` for primary keys — a plain `BigInteger` PK breaks every INSERT on SQLite.
- `utcnow()` — never `datetime.utcnow()`.
- Named constraints — set the naming convention before the first migration.
- SQLite pragmas — without them `ondelete` is silently ignored.
- Migrations at deploy, never inside `create_app`.
- With `tenancy: shared`, business models inherit `OrgScoped` and queries are
  never manually filtered by `org_id`.
- Run `make css` after touching any template, and commit the result — an
  uncompiled class renders unstyled in production and nowhere else.
- Logical properties (`ms-`/`me-`/`ps-`/`pe-`/`text-start`), never `ml-`/`mr-`/
  `text-left` — physical properties silently break RTL.
- Every interactive component carries its own ARIA; Tailwind ships none.
- With `plugins: true`, plugin templates use `plugin_url_for`, never `url_for`,
  and no two plugin versions are ever registered on the same URL prefix.

---

## License

MIT — see [LICENSE](LICENSE).
