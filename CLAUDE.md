# CLAUDE.md - Flask SaaS Blueprint (MVC Pattern)

> **Master guide for bootstrapping production-ready Flask SaaS applications.**
> Single-file blueprint combining architecture, MVC patterns, database setup, i18n, security, and deployment.
> Optimized for **fast iteration**, **clean code**, and **AI agent execution**.

---

## 🚀 QUICK START (New Project Bootstrap)

**👉 AI Agents: Execute this section first (20 minutes to running app)**

### Prerequisites Checklist
- [ ] Python 3.11+
- [ ] Make
- [ ] PostgreSQL 15+ (optional - SQLite works out of the box)

### Bootstrap Sequence (Follow in Order)

**Step 1: Initialize Project** (2 min)
```bash
mkdir yourapp && cd yourapp
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

mkdir -p app/{models,controllers,views/{layouts,partials,users,settings,errors},static/{css,js,img},lang,middleware,platform}
mkdir -p migrations/versions tests/{test_models,test_controllers} config patterns
touch app/__init__.py app/models/__init__.py app/controllers/__init__.py
touch app/middleware/__init__.py app/platform/__init__.py
```
✅ **Verify:** `ls app` shows folder structure

**Step 2: Install Dependencies** (1 min)
```bash
# Create requirements.txt
cat > requirements.txt <<EOF
Flask>=3.0.0
Flask-SQLAlchemy>=3.1.0
Flask-Login>=0.6.3
Flask-Migrate>=4.0.5
SQLAlchemy>=2.0.0
structlog>=24.1.0
python-dotenv>=1.0.0
gunicorn>=21.0.0
pytest>=8.0.0
EOF

pip install -r requirements.txt

# Optional: Add PostgreSQL support when needed
# pip install psycopg2-binary
```
✅ **Verify:** `pip list` shows Flask installed

**Step 3: Database Setup** (1 min)
```bash
# SQLite (default - no setup required)
# Database file created automatically at app.db

# Optional: PostgreSQL (for production or advanced features)
# docker run --name app-db \
#   -e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=app \
#   -p 5432:5432 -d postgres:15-alpine
# Then set: DATABASE_URL="postgresql://app:app@localhost:5432/app"
```
✅ **Verify:** Continue to next step (SQLite needs no verification)

**Step 4: Platform Layer** (10 min)

Create these files (see detailed sections below):
- `app/extensions.py` → § Extensions Setup
- `app/config.py` → § Configuration
- `app/platform/logger.py` → § Logging
- `app/platform/i18n.py` → [patterns/i18n.md](patterns/i18n.md)
- `app/platform/errors.py` → § Error Handling

✅ **Verify:** Files exist, no import errors

**Step 5: First Model + Controller** (10 min)
- Create `app/models/base.py` → § Models Pattern
- Create `app/models/user.py` → § Models Pattern
- Create `app/controllers/main.py` → § Controllers Pattern
- Create `app/__init__.py` → § Application Factory
- Create `app/views/layouts/base.html` → § Views Pattern

✅ **Verify:** `python -c "from app import create_app; create_app()"` succeeds

**Step 6: Environment & Run** (2 min)
```bash
# Create config/local.env
cat > config/local.env <<EOF
APP_ENV=dev
PORT=8000
DATABASE_URL=sqlite:///app.db
SECRET_KEY=dev-secret-change-in-production
DEV_MAGIC=true
EOF

# Create run.py
cat > run.py <<EOF
from dotenv import load_dotenv
load_dotenv('config/local.env')
from app import create_app
app = create_app()
if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000, debug=True)
EOF

python run.py
```
✅ **Verify:** Server runs at http://localhost:8000

**Expected Time: 20 minutes to running app**

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
4. [Application Factory](#application-factory) - Flask app setup
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
- [Multi-Tenancy](#multi-tenancy) → [patterns/database.md](patterns/database.md#multi-tenancy-with-row-level-security)
- [Security](#security) → [patterns/security.md](patterns/security.md)

### Operations
- [Testing](#testing) → [patterns/testing.md](patterns/testing.md)
- [Deployment](#deployment) → [patterns/deployment.md](patterns/deployment.md)

### Pattern Guides
- [MVC Pattern Guide](patterns/mvc.md) - Models, Views, Controllers in detail
- [Database Patterns](patterns/database.md) - SQLAlchemy, Alembic, JSONB, FTS
- [i18n Guide](patterns/i18n.md) - Complete internationalization
- [Auth & Sessions](patterns/auth.md) - Magic links, OAuth, sessions
- [HTMX Cookbook](patterns/htmx.md) - Interactive patterns
- [Frontend Guide](patterns/frontend.md) - Bootstrap + HTMX
- [Testing Guide](patterns/testing.md) - pytest patterns
- [Security Guide](patterns/security.md) - CSRF, rate limiting, security checklist
- [Deployment Guide](patterns/deployment.md) - Production deployment

---

## Philosophy

### Core Principles

- **Keep it simple, explicit, and local.** No magic, no over-engineering.
- **MVC Pattern**: Models (fat), Views (templates), Controllers (thin).
- **Server-rendered HTML + HTMX**: No SPA complexity, progressive enhancement.
- **i18n from day 1**: Global-ready from the start.
- **Security and observability by default**: CSRF, rate limiting, logging.
- **Zero yak-shaving dev loop**: `python run.py` boots a working app.

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
│   ├── __init__.py              # Application factory
│   ├── config.py                # Configuration management
│   ├── extensions.py            # SQLAlchemy, LoginManager init
│   ├── models/                  # Domain models (fat models)
│   │   ├── __init__.py
│   │   ├── base.py              # BaseModel with timestamps
│   │   ├── user.py              # User model (CRUD + business logic)
│   │   └── setting.py           # Setting model (key-value config)
│   ├── controllers/             # Flask blueprints (thin)
│   │   ├── __init__.py
│   │   ├── main.py              # Home, about pages
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
├── config/
│   └── local.env.example
├── patterns/                    # Documentation
├── run.py                       # Development entry point
├── wsgi.py                      # Production entry point (Gunicorn)
├── requirements.txt
├── Makefile
├── Dockerfile
├── README.md
└── CLAUDE.md                    # This file
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Backend** | Python 3.11+ | Type hints, performance, ecosystem |
| **Web Framework** | Flask 3.x | Simple, mature, Jinja2 built-in |
| **Database** | SQLite (default) / PostgreSQL | No setup needed; Postgres for advanced features |
| **ORM** | SQLAlchemy 2.0 | Declarative models, excellent Postgres support |
| **Migrations** | Alembic (Flask-Migrate) | SQLAlchemy's official migration tool |
| **Templates** | Jinja2 | Auto-escaping, fast, Flask-native |
| **Frontend** | Bootstrap 5 + HTMX | No build step, progressive enhancement |
| **i18n** | JSON catalogs | Simple, runtime-loaded |
| **Logging** | structlog | Structured JSON logging |
| **Auth** | Flask-Login + Magic links | Easy to start, extensible |
| **Testing** | pytest | Transaction rollback pattern |
| **Server** | Gunicorn | Production WSGI server |

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

    # Database (PostgreSQL or SQLite)
    DATABASE_URL = os.environ.get('DATABASE_URL', 'sqlite:///app.db')
    SQLALCHEMY_DATABASE_URI = DATABASE_URL
    SQLALCHEMY_TRACK_MODIFICATIONS = False

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

```python
# app/__init__.py
"""Flask application factory."""

from flask import Flask
from .config import Config
from .extensions import db, migrate, login_manager
from .platform.logger import init_logger


def create_app(config_class=Config):
    """Create and configure the Flask application."""
    app = Flask(__name__, template_folder='views', static_folder='static')
    app.config.from_object(config_class)

    # Initialize logging
    init_logger(app.config['APP_ENV'])

    # Initialize extensions
    db.init_app(app)
    migrate.init_app(app, db)
    login_manager.init_app(app)

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

### Default: User-Owned Data (B2C)

**Most SaaS apps** start here:
- Use `user_id` foreign keys
- Application-layer filtering in queries
- Simpler, faster to build

```python
class Setting(BaseModel):
    __tablename__ = 'setting'

    user_id = db.Column(db.BigInteger, db.ForeignKey('user.id'), nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text, nullable=False)

    __table_args__ = (db.UniqueConstraint('user_id', 'key'),)
```

→ **For multi-tenant (B2B) patterns:** See [patterns/database.md](patterns/database.md#multi-tenancy)

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

### Entry Points

```python
# run.py (Development)
from dotenv import load_dotenv
load_dotenv('config/local.env')
from app import create_app

app = create_app()

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000, debug=True)
```

```python
# wsgi.py (Production)
from app import create_app
app = create_app()
```

### Production Command

```bash
gunicorn wsgi:app -w 4 -b 0.0.0.0:8000
```

→ **For complete deployment guide:** See [patterns/deployment.md](patterns/deployment.md)

---

## Makefile

```makefile
.PHONY: run install test migrate

run:
	python run.py

install:
	pip install -r requirements.txt

test:
	pytest tests/ -v --cov=app

migrate:
	flask db upgrade

migrate-new:
	flask db migrate -m "$(msg)"
```

---

## 🤖 AI Agent Instructions

### For Claude/AI Assistants

**When bootstrapping new project:**
1. Read § Quick Start (top of file)
2. Execute steps 1-6 in order
3. Reference detailed sections as needed
4. Verify: `python run.py` succeeds

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
- Quick Start (ensures nothing missed)
- MVC Pattern sections (core architecture)

---

## License

Copyright (c) Your Organization.

---

**That's it.** One file to guide any Flask SaaS project with MVC, i18n, and production-ready patterns.
**Build fast, read easily, scale calmly.**
