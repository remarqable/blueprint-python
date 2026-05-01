# CLAUDE.md - Flask SaaS Blueprint (MVC Pattern)

> **Master guide for bootstrapping production-ready Flask SaaS applications.**
> Single-file blueprint combining architecture, MVC patterns, database setup, i18n, security, and deployment.
> Optimized for **fast iteration**, **clean code**, and **AI agent execution**.

---

## 🚀 QUICK START (New Project Bootstrap)

**👉 AI Agents: Execute this section first**

### Step 0: Ask Configuration Questions

Before creating any files, ask the user these questions:

**1. App Type** (determines multi-tenancy model)
- **B2C (User-based)**: Each user owns their own data. Use `user_id` foreign keys.
- **B2B (Organization-based)**: Users belong to organizations. Use `org_id` foreign keys.

**2. Audit Logging**
- **No audit**: Simple apps. Just `created_at`/`updated_at` timestamps.
- **Yes, add audit**: Track `created_by`, `updated_by` on models.

Record the answers in the Project Configuration section below.

### Prerequisites Checklist
- [ ] Python 3.11+
- [ ] Make

### Bootstrap Sequence (Follow in Order)

**Step 1: Initialize Project**
```bash
mkdir yourapp && cd yourapp

# Create folder structure
mkdir -p app/{models,controllers,views/{layouts,partials,users,settings,errors},static/{css,js,img},lang,middleware,platform}
mkdir -p migrations/versions tests/{test_models,test_controllers} config patterns scripts
touch app/__init__.py app/models/__init__.py app/controllers/__init__.py
touch app/middleware/__init__.py app/platform/__init__.py
```
✅ **Verify:** `ls app` shows folder structure

**Step 2: Create requirements.in**
```bash
cat > requirements.in <<EOF
# Core
Flask
Flask-SQLAlchemy
Flask-Login
Flask-Migrate
SQLAlchemy

# Utilities
structlog
python-dotenv

# Production server
gunicorn

# Testing
pytest
pytest-cov
EOF
```

**Step 3: Create Makefile**
```makefile
.PHONY: venv install compile run test clean deploy

venv:
	python3 -m venv venv
	@echo "Run 'source venv/bin/activate' to activate."

install: venv
	./venv/bin/pip install -r requirements.txt

compile:
	./venv/bin/pip install pip-tools
	./venv/bin/pip-compile requirements.in -o requirements.txt

run:
	./venv/bin/python run.py

test:
	./venv/bin/pytest tests/ -v

clean:
	rm -rf venv
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true

deploy:
	./scripts/deploy.sh
```

**Step 4: Setup Environment**
```bash
# Create virtual environment and install
make venv
source venv/bin/activate
make compile
make install

# Create config/local.env
cat > config/local.env <<EOF
APP_ENV=dev
PORT=8000
DATABASE_URL=sqlite:///app.db
SECRET_KEY=dev-secret-change-in-production
DEV_MAGIC=true
EOF
```
✅ **Verify:** `pip list` shows Flask installed

**Step 5: Platform Layer**

Create these files (see detailed sections below):
- `app/extensions.py` → § Extensions Setup
- `app/config.py` → § Configuration
- `app/platform/logger.py` → § Logging
- `app/platform/i18n.py` → [patterns/i18n.md](patterns/i18n.md)
- `app/platform/errors.py` → § Error Handling

✅ **Verify:** Files exist, no import errors

**Step 6: First Model + Controller**
- Create `app/models/base.py` → § Models Pattern (use B2C or B2B based on config)
- Create `app/models/user.py` → § Models Pattern
- Create `app/controllers/main.py` → § Controllers Pattern
- Create `app/__init__.py` → § Application Factory (includes auto-migrations)
- Create `app/views/layouts/base.html` → § Views Pattern

✅ **Verify:** `python -c "from app import create_app; create_app()"` succeeds

**Step 7: Run**
```bash
# Create run.py
cat > run.py <<EOF
from dotenv import load_dotenv
load_dotenv('config/local.env')
from app import create_app
app = create_app()
if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000, debug=True)
EOF

make run
```
✅ **Verify:** Server runs at http://localhost:8000

---

## 📋 PROJECT CONFIGURATION

> **AI Agents:** Update this section after asking the user configuration questions.

```yaml
# Project Configuration (fill in after asking user)
app_type: B2C  # or B2B
audit_logging: false  # or true
```

### What This Means

**If B2C:**
- Use `user_id` foreign keys on data models
- No Organization model needed
- See [patterns/database.md#b2c-user-owned-data](patterns/database.md#b2c-user-owned-data)

**If B2B:**
- Create Organization model
- Use `org_id` foreign keys on data models
- Users have `org_id` and `role` fields
- See [patterns/database.md#b2b-organization-based](patterns/database.md#b2b-organization-based)

**If audit_logging: true:**
- Add `created_by` and `updated_by` to BaseModel
- Track user who made changes

---

## 📋 TABLE OF CONTENTS

### Foundation (Read Once)
- [Philosophy](#philosophy) - Core principles
- [Folder Structure](#folder-structure) - MVC layout
- [Tech Stack](#tech-stack) - Dependencies & rationale
- [Conventions](#conventions) - Code style

### Platform Components (Implement in Order)
1. [Extensions Setup](#extensions-setup) - SQLAlchemy, Login Manager
2. [Configuration](#configuration) - Environment variables
3. [Logging](#logging) - Structured logging
4. [Application Factory](#application-factory) - Flask app setup with auto-migrations
5. [Internationalization](#internationalization) → [patterns/i18n.md](patterns/i18n.md)
6. [Sessions & Auth](#sessions--auth) → [patterns/auth.md](patterns/auth.md)
7. [Error Handling](#error-handling) - Custom exceptions

### MVC Pattern
- [Models](#models) - Fat models (business logic + DB)
- [Views](#views) - Jinja2 templates with HTMX
- [Controllers](#controllers) - Thin Flask blueprints

### Features (Cross-referenced)
- [HTMX Patterns](#htmx-patterns) → [patterns/htmx.md](patterns/htmx.md)
- [Frontend Architecture](#frontend-architecture) → [patterns/frontend.md](patterns/frontend.md)
- [Multi-Tenancy](#multi-tenancy) → [patterns/database.md](patterns/database.md#multi-tenancy)
- [Security](#security) → [patterns/security.md](patterns/security.md)

### Operations
- [Testing](#testing) → [patterns/testing.md](patterns/testing.md)
- [Deployment](#deployment) → [patterns/deployment.md](patterns/deployment.md)

### Pattern Guides
- [MVC Pattern Guide](patterns/mvc.md) - Models, Views, Controllers in detail
- [Database Patterns](patterns/database.md) - SQLAlchemy, auto-migrations, multi-tenancy
- [Soft Delete](patterns/soft-delete.md) - SoftDeleteMixin, query patterns, hard-delete rules
- [Async Processing](patterns/async-processing.md) - Threads, in-memory queues, scheduler, asyncio
- [Audit Trail](patterns/audit.md) - created_by / updated_by tracking
- [i18n Guide](patterns/i18n.md) - Complete internationalization
- [Auth & Sessions](patterns/auth.md) - Magic links, OAuth, sessions
- [HTMX Cookbook](patterns/htmx.md) - Interactive patterns
- [Frontend Guide](patterns/frontend.md) - Bootstrap + HTMX
- [CSS Architecture](patterns/css.md) - File layout, variables, naming conventions
- [Color System](patterns/colors.md) - Design tokens, dark theme, CSS variables
- [Mobile Navigation](patterns/mobile-navigation.md) - Device detection, mobile templates
- [Module System](patterns/module-system.md) - Pluggy-based plugin loading (optional)
- [Testing Guide](patterns/testing.md) - pytest patterns
- [Typing Guide](patterns/typing.md) - mypy --strict patterns
- [Security Guide](patterns/security.md) - CSRF, rate limiting, security checklist
- [Deployment Guide](patterns/deployment.md) - systemd + Caddy on Digital Ocean

---

## Philosophy

### Core Principles

- **Keep it simple, explicit, and local.** No magic, no over-engineering.
- **MVC Pattern**: Models (fat), Views (templates), Controllers (thin).
- **Server-rendered HTML + HTMX**: No SPA complexity, progressive enhancement.
- **i18n from day 1**: Global-ready from the start.
- **Security and observability by default**: CSRF, rate limiting, logging.
- **Zero yak-shaving dev loop**: `make run` boots a working app.
- **SQLite by default**: No database server needed. Switch to PostgreSQL when you need it.
- **Auto-migrations**: Migrations run automatically at startup.

### Fat Models, Thin Controllers

**Models** contain:
- Business logic (validation, calculations)
- Database access (CRUD, queries)
- Domain rules

**Controllers** contain:
- Parse input
- Call model methods
- Render view or return JSON

**Views** contain:
- Jinja2 templates
- Minimal logic (loops, conditions)
- HTMX attributes for interactivity

---

## Folder Structure

```
yourapp/
├── app/
│   ├── __init__.py              # Application factory (with auto-migrations)
│   ├── config.py                # Configuration management
│   ├── extensions.py            # SQLAlchemy, LoginManager init
│   ├── models/                  # Domain models (fat models)
│   │   ├── __init__.py
│   │   ├── base.py              # BaseModel with timestamps
│   │   ├── user.py              # User model (CRUD + business logic)
│   │   └── setting.py           # Setting model (key-value config)
│   ├── controllers/             # Flask blueprints (thin)
│   │   ├── __init__.py
│   │   ├── main.py              # Home, about pages, health check
│   │   ├── auth.py              # Login, logout, magic links
│   │   ├── users.py             # Profile management
│   │   └── settings.py          # User settings
│   ├── views/                   # Jinja2 templates
│   │   ├── layouts/
│   │   │   ├── base.html        # Main layout (navbar, footer)
│   │   │   └── minimal.html     # Auth pages (no navbar)
│   │   ├── partials/
│   │   │   ├── _navbar.html     # Shared navbar
│   │   │   └── _toast.html      # Toast notifications
│   │   ├── main/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── settings/
│   │   └── errors/
│   ├── static/                  # Static assets
│   │   ├── css/
│   │   │   ├── bootstrap.min.css
│   │   │   └── app.css          # Brand overrides (~50-100 lines)
│   │   ├── js/
│   │   │   ├── htmx.min.js
│   │   │   └── bootstrap.bundle.min.js
│   │   └── img/
│   ├── lang/                    # i18n translation files
│   │   ├── en.json
│   │   └── es.json
│   ├── middleware/              # Request hooks
│   │   ├── __init__.py
│   │   ├── auth.py
│   │   ├── csrf.py
│   │   └── ratelimit.py
│   └── platform/                # Infrastructure layer
│       ├── __init__.py
│       ├── logger.py            # Structured logging (structlog)
│       ├── i18n.py              # Internationalization
│       └── errors.py            # Custom exceptions
├── migrations/                  # Alembic migrations
│   └── versions/
├── tests/                       # pytest tests
│   ├── conftest.py              # Fixtures
│   ├── test_models/
│   └── test_controllers/
├── scripts/
│   └── deploy.sh                # Production deployment script
├── config/
│   └── local.env.example
├── patterns/                    # Documentation
├── run.py                       # Development entry point
├── wsgi.py                      # Production entry point (Gunicorn)
├── requirements.in              # Package names (no versions)
├── requirements.txt             # Compiled with pinned versions
├── Makefile
├── README.md
└── CLAUDE.md                    # This file
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Backend** | Python 3.11+ | Type hints, performance, ecosystem |
| **Web Framework** | Flask 3.x | Simple, mature, Jinja2 built-in |
| **Database** | SQLite (default) | No setup; PostgreSQL when needed |
| **ORM** | SQLAlchemy 2.0 | Declarative models, Postgres-compatible |
| **Migrations** | Alembic (Flask-Migrate) | Auto-run at startup |
| **Templates** | Jinja2 | Auto-escaping, fast, Flask-native |
| **Frontend** | Bootstrap 5 + HTMX | No build step, progressive enhancement |
| **i18n** | JSON catalogs | Simple, runtime-loaded |
| **Logging** | structlog | Structured JSON logging |
| **Auth** | Flask-Login + Magic links | Easy to start, extensible |
| **Testing** | pytest | Transaction rollback pattern |
| **Server** | Gunicorn + systemd | Production WSGI server |
| **Reverse Proxy** | Caddy | Automatic HTTPS |

---

## Conventions

### Python Code
- **Type hints** for function signatures
- **Docstrings** for public methods
- **Classes for models**, functions for utilities
- **Files ~300 lines**: Split by concern (`user.py`, `user_validation.py`)

### HTTP
- **Controllers**: Parse input, call models, render view/JSON
- **Use Flask error handlers** for exceptions
- **Add request ID middleware** for traceability

### Templates
- **Jinja2** with auto-escaping (no `|safe` unless trusted)
- **Partials prefix**: `_partial_name.html`
- **All user-visible strings in i18n catalogs**

### Frontend
- **Bootstrap 5** for CSS (no Tailwind, no custom frameworks)
- **HTMX** for interactivity (no custom JavaScript)
- **One global CSS file** (`static/css/app.css`) for brand overrides only
- **No build pipeline** (no npm, webpack, vite)

---

## Extensions Setup

```python
# app/extensions.py
"""Flask extensions initialization."""

from flask_sqlalchemy import SQLAlchemy
from flask_login import LoginManager
from flask_migrate import Migrate

db = SQLAlchemy()
migrate = Migrate()

login_manager = LoginManager()
login_manager.login_view = 'auth.login'
login_manager.login_message_category = 'warning'


@login_manager.user_loader
def load_user(user_id):
    from .models.user import User
    return User.query.get(int(user_id))
```

---

## Configuration

```python
# app/config.py
"""Configuration management."""

import os


class Config:
    """Base configuration."""

    APP_ENV = os.environ.get('APP_ENV', 'dev')
    SECRET_KEY = os.environ.get('SECRET_KEY', 'dev-secret-change-in-production')
    DEBUG = APP_ENV == 'dev'

    # Database (SQLite default, PostgreSQL optional)
    DATABASE_URL = os.environ.get('DATABASE_URL', 'sqlite:///app.db')
    SQLALCHEMY_DATABASE_URI = DATABASE_URL
    SQLALCHEMY_TRACK_MODIFICATIONS = False

    # PostgreSQL connection pooling (only if using Postgres)
    if DATABASE_URL.startswith('postgresql'):
        SQLALCHEMY_ENGINE_OPTIONS = {
            'pool_size': 5,
            'max_overflow': 10,
            'pool_timeout': 30,
            'pool_recycle': 300,
            'pool_pre_ping': True,
        }

    # Session
    SESSION_COOKIE_SECURE = APP_ENV != 'dev'
    SESSION_COOKIE_HTTPONLY = True
    SESSION_COOKIE_SAMESITE = 'Lax'

    # Auth
    DEV_MAGIC = os.environ.get('DEV_MAGIC', 'false').lower() == 'true'
```

---

## Logging

```python
# app/platform/logger.py
"""Structured logging with structlog."""

import structlog
import logging
import sys


def init_logger(env: str = 'dev'):
    """Initialize logger based on environment."""
    logging.basicConfig(format='%(message)s', stream=sys.stdout, level=logging.INFO)

    if env == 'dev':
        structlog.configure(
            processors=[
                structlog.stdlib.add_log_level,
                structlog.processors.TimeStamper(fmt='%Y-%m-%d %H:%M:%S'),
                structlog.dev.ConsoleRenderer(colors=True),
            ],
            wrapper_class=structlog.stdlib.BoundLogger,
            logger_factory=structlog.stdlib.LoggerFactory(),
        )
    else:
        structlog.configure(
            processors=[
                structlog.stdlib.add_log_level,
                structlog.processors.TimeStamper(fmt='iso'),
                structlog.processors.JSONRenderer(),
            ],
            wrapper_class=structlog.stdlib.BoundLogger,
            logger_factory=structlog.stdlib.LoggerFactory(),
        )


def get_logger():
    """Get configured logger."""
    return structlog.get_logger()
```

---

## Application Factory

**Note:** Migrations run automatically at startup.

```python
# app/__init__.py
"""Flask application factory with auto-migrations."""

from flask import Flask
from flask_migrate import upgrade
from .config import Config
from .extensions import db, migrate, login_manager
from .platform.logger import init_logger, get_logger


def create_app(config_class=Config):
    """Create and configure the Flask application."""
    app = Flask(__name__, template_folder='views', static_folder='static')
    app.config.from_object(config_class)

    # Initialize logging
    init_logger(app.config['APP_ENV'])
    log = get_logger()

    # Initialize extensions
    db.init_app(app)
    migrate.init_app(app, db)
    login_manager.init_app(app)

    # Auto-run migrations on startup
    with app.app_context():
        upgrade()
        log.info('migrations_applied')

    # Register blueprints
    from .controllers import main, auth, users, settings
    app.register_blueprint(main.bp)
    app.register_blueprint(auth.bp)
    app.register_blueprint(users.bp)
    app.register_blueprint(settings.bp)

    # Register error handlers
    register_error_handlers(app)

    return app


def register_error_handlers(app):
    """Register error handlers."""
    from flask import render_template

    @app.errorhandler(404)
    def not_found(error):
        return render_template('errors/404.html'), 404

    @app.errorhandler(500)
    def internal_error(error):
        db.session.rollback()
        return render_template('errors/500.html'), 500
```

---

## Error Handling

```python
# app/platform/errors.py
"""Custom error classes."""


class AppError(Exception):
    """Base application error."""
    code = 'E_UNKNOWN'
    http_status = 500

    def __init__(self, message: str = 'An unexpected error occurred'):
        self.message = message
        super().__init__(self.message)


class ValidationError(AppError):
    """Validation error (400)."""
    code = 'E_INVALID_INPUT'
    http_status = 400


class NotFoundError(AppError):
    """Not found error (404)."""
    code = 'E_NOT_FOUND'
    http_status = 404


class UnauthorizedError(AppError):
    """Unauthorized error (401)."""
    code = 'E_UNAUTHORIZED'
    http_status = 401
```

---

## Models

→ **See complete guide:** [patterns/mvc.md](patterns/mvc.md#models-fat-models)

→ **For multi-tenancy (B2C vs B2B):** [patterns/database.md](patterns/database.md#multi-tenancy)

---

## Controllers

→ **See complete guide:** [patterns/mvc.md](patterns/mvc.md#controllers-thin-controllers)

---

## Views

→ **See complete guide:** [patterns/mvc.md](patterns/mvc.md#views-templates)

---

## HTMX Patterns

→ **See complete guide:** [patterns/htmx.md](patterns/htmx.md)

---

## Frontend Architecture

**Bootstrap 5 + HTMX. No build pipeline.**

### Principles

- ✅ Reuse Bootstrap classes (never invent custom classes)
- ✅ One `app.css` for brand overrides (~50-100 lines max)
- ✅ No npm, webpack, or build tools
- ✅ HTMX for all interactivity
- ✅ Progressive enhancement (works without JS)

→ **For complete guide:** See [patterns/frontend.md](patterns/frontend.md)

---

## Multi-Tenancy

Based on your Project Configuration:

### B2C: User-Owned Data

```python
class Setting(BaseModel):
    __tablename__ = 'setting'

    user_id = db.Column(db.BigInteger, db.ForeignKey('user.id'), nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text, nullable=False)

    __table_args__ = (db.UniqueConstraint('user_id', 'key'),)
```

### B2B: Organization-Based

```python
class OrgScopedModel(BaseModel):
    """Base for organization-scoped models."""
    __abstract__ = True

    org_id = db.Column(db.BigInteger, db.ForeignKey('organization.id'), nullable=False, index=True)
```

→ **For complete patterns:** See [patterns/database.md](patterns/database.md#multi-tenancy)

---

## Security

→ **See complete guide:** [patterns/security.md](patterns/security.md)

---

## Testing

**Fast, isolated tests using transaction rollback.**

```python
# tests/conftest.py
import pytest
from app import create_app
from app.extensions import db


@pytest.fixture
def app():
    app = create_app()
    app.config['SQLALCHEMY_DATABASE_URI'] = 'sqlite:///:memory:'
    app.config['TESTING'] = True

    with app.app_context():
        db.create_all()
        yield app
        db.drop_all()


@pytest.fixture
def client(app):
    return app.test_client()
```

→ **For complete testing guide:** See [patterns/testing.md](patterns/testing.md)

---

## Deployment

**Stack:** Digital Ocean Droplet + systemd + Gunicorn + Caddy

### Quick Deploy

```bash
make deploy
```

This runs `scripts/deploy.sh` which:
1. Checks for uncommitted changes
2. Pushes to origin
3. Pulls on server
4. Installs dependencies
5. Restarts systemd service
6. Verifies health check

→ **For complete deployment guide:** See [patterns/deployment.md](patterns/deployment.md)

---

## Makefile

```makefile
.PHONY: venv install compile run test clean deploy

venv:        # Create virtual environment
install:     # Install dependencies
compile:     # Compile requirements.in -> requirements.txt
run:         # Run development server
test:        # Run tests
clean:       # Remove venv and cached files
deploy:      # Deploy to production
```

---

## 🤖 AI Agent Instructions

### For Claude/AI Assistants

**When bootstrapping new project:**
1. **Ask configuration questions first** (B2C/B2B, audit logging)
2. Record answers in Project Configuration section
3. Execute Quick Start steps 1-7 in order
4. Use appropriate model patterns based on configuration
5. Verify: `make run` succeeds

**When adding features:**
1. Scan Table of Contents for relevant section
2. Jump to section via anchor link
3. If section says "See patterns/X.md", read that file

**When troubleshooting:**
1. Check relevant platform component section
2. Review patterns/ for edge cases

### Files to Read (in order)
1. `CLAUDE.md` - master blueprint (this file)
2. `patterns/*.md` - Only when referenced

**Never skip:**
- Philosophy (defines patterns)
- Project Configuration (determines model patterns)
- Quick Start (ensures nothing missed)
- MVC Pattern sections (core architecture)

---

## License

Copyright (c) Your Organization.

---

**That's it.** One file to guide any Flask SaaS project with MVC, i18n, and production-ready patterns.
**Build fast, read easily, scale calmly.**
