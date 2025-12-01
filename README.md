# Flask Blueprint

> Documentation blueprint for building production-ready Flask applications with modular architecture and MVC pattern.

## What is This?

This is a **documentation-only blueprint** that guides AI agents and developers in building Flask applications following best practices. It contains no actual code - just comprehensive guides on architecture, patterns, and implementation.

## Quick Start

When starting a new project, have Claude read:

1. `CLAUDE.md` - Master blueprint with Quick Start section
2. `patterns/*.md` - Detailed implementation guides (as needed)

## Structure

```
blueprint-python/
├── CLAUDE.md                    # Master blueprint - read this first
├── patterns/
│   ├── mvc.md                   # Models, Views, Controllers
│   ├── database.md              # SQLAlchemy, Alembic, queries
│   ├── i18n.md                  # Internationalization
│   ├── auth.md                  # Magic links, sessions
│   ├── htmx.md                  # HTMX patterns
│   ├── frontend.md              # Bootstrap + HTMX
│   ├── security.md              # CSRF, rate limiting
│   ├── testing.md               # pytest patterns
│   └── deployment.md            # Gunicorn, Docker, cloud
└── README.md                    # This file
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Python 3.11+ / Flask 3.x |
| Database | PostgreSQL or SQLite |
| ORM | SQLAlchemy 2.0 |
| Templates | Jinja2 |
| Frontend | Bootstrap 5 + HTMX |
| Testing | pytest |
| Server | Gunicorn |

## Philosophy

- **Keep it simple** - No over-engineering
- **MVC Pattern** - Fat models, thin controllers
- **Server-rendered HTML** - HTMX for interactivity
- **No build pipeline** - No npm, webpack, etc.
- **i18n from day 1** - Global-ready

## Usage as Submodule

```bash
# Add to your project
git submodule add [your-repo-url] blueprint

# Point Claude to it
# In your project's CLAUDE.md:
# "See blueprint/CLAUDE.md for architecture patterns"
```

## License

Copyright (c) Your Organization.
