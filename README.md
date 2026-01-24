# Blueprint-Python

> Documentation blueprint for building production-ready Flask SaaS applications with MVC pattern.

## What is This?

This is a **documentation-only blueprint** that guides AI agents and developers in building Flask applications following best practices. It contains no actual code - just comprehensive guides on architecture, patterns, and implementation.

## Quick Start

When starting a new project, have Claude read:

1. `CLAUDE.md` - Master blueprint with Quick Start section
2. `patterns/*.md` - Detailed implementation guides (as needed)

The AI agent will ask you:
- **B2C or B2B?** - Determines multi-tenancy model (user-based or organization-based)
- **Audit logging?** - Whether to track who changed what

## Structure

```
blueprint-python/
├── CLAUDE.md                    # Master blueprint - read this first
├── Makefile                     # venv, install, compile, run, test, clean, deploy
├── requirements.in              # Package names (no versions)
├── scripts/
│   └── deploy.sh                # Production deployment script
├── patterns/
│   ├── mvc.md                   # Models, Views, Controllers
│   ├── database.md              # SQLAlchemy, auto-migrations, multi-tenancy
│   ├── i18n.md                  # Internationalization
│   ├── auth.md                  # Magic links, sessions
│   ├── htmx.md                  # HTMX patterns
│   ├── frontend.md              # Bootstrap + HTMX
│   ├── security.md              # CSRF, rate limiting
│   ├── testing.md               # pytest patterns
│   └── deployment.md            # systemd + Caddy on Digital Ocean
└── README.md                    # This file
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Python 3.11+ / Flask 3.x |
| Database | SQLite (default) / PostgreSQL |
| ORM | SQLAlchemy 2.0 |
| Migrations | Alembic (auto-run at startup) |
| Templates | Jinja2 |
| Frontend | Bootstrap 5 + HTMX |
| Testing | pytest |
| Server | Gunicorn + systemd |
| Reverse Proxy | Caddy (automatic HTTPS) |

## Philosophy

- **Keep it simple** - No over-engineering
- **MVC Pattern** - Fat models, thin controllers
- **Server-rendered HTML** - HTMX for interactivity
- **No build pipeline** - No npm, webpack, etc.
- **SQLite by default** - No database server needed
- **Auto-migrations** - Run at startup, no manual steps
- **i18n from day 1** - Global-ready

## Makefile Targets

```bash
make venv      # Create virtual environment
make install   # Install dependencies
make compile   # Compile requirements.in -> requirements.txt
make run       # Run development server
make test      # Run tests
make clean     # Remove venv and cached files
make deploy    # Deploy to production
```

## Deployment

Single VPS deployment with:
- **Digital Ocean Droplet** (or any VPS)
- **systemd** for process management
- **Gunicorn** as WSGI server
- **Caddy** for reverse proxy + automatic HTTPS

No Docker, no containers, no orchestration complexity.

## Usage as Submodule

```bash
# Add to your project
git submodule add https://github.com/remarqable/blueprint-python.git blueprint

# Point Claude to it
# In your project's CLAUDE.md:
# "See blueprint/CLAUDE.md for architecture patterns"
```

## License

Copyright (c) Your Organization.
