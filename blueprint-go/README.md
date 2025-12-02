# Blueprint Go

> Documentation-only blueprint for building modular Go web applications with Gin, GORM, and HTMX.
> Use as a git submodule for AI/LLM-guided development.

## What This Is

Blueprint Go is a **documentation-only reference** - patterns and templates for AI/LLM agents to follow when building Go web applications. It is NOT runnable code.

**Similar to:** [blueprint-python](../README.md) - the Python version of this blueprint

## Usage as Git Submodule

```bash
# Add to your project
git submodule add <repo-url> blueprint-go

# AI agents read CLAUDE.md and patterns/ for guidance
```

## Features

- **Modular Architecture** - Self-contained modules with models, handlers, views, translations
- **Fat Models, Thin Controllers** - Business logic in repositories
- **Server-Rendered HTML** - html/template + HTMX, no SPA complexity
- **CSS-First Frontend** - Minimal Alpine.js, CSS animations preferred
- **Locality of Behavior** - Keep styles close to usage
- **i18n from Day 1** - Module-scoped JSON translations
- **Security Built-In** - CSRF, rate limiting, validation patterns

## Tech Stack (Recommended)

| Component | Technology |
|-----------|------------|
| **Language** | Go 1.21+ |
| **Web Framework** | [Gin](https://gin-gonic.com/) |
| **ORM** | [GORM](https://gorm.io/) |
| **Database** | SQLite (dev) / PostgreSQL (prod) |
| **Migrations** | [Goose](https://github.com/pressly/goose) |
| **Templates** | html/template |
| **Frontend** | HTMX + Alpine.js (minimal) + Bootstrap 5 |
| **Sessions** | gin-contrib/sessions |

## Documentation Structure

```
blueprint-go/
├── CLAUDE.md                   # Master AI guide (~1000 lines)
├── README.md                   # This file
├── patterns/                   # Detailed pattern guides
│   ├── module-system.md        # Module interfaces, loading
│   ├── mvc.md                  # Fat models, thin handlers
│   ├── database.md             # GORM patterns
│   ├── auth.md                 # Session auth, groups
│   ├── frontend.md             # HTMX, minimal Alpine, CSS-first
│   ├── htmx.md                 # Server-driven interactivity
│   ├── i18n.md                 # JSON translations
│   ├── security.md             # CSRF, rate limiting
│   ├── audit.md                # Created/updated by tracking
│   ├── typing.md               # Go type patterns
│   ├── testing.md              # Unit/integration tests
│   └── deployment.md           # Docker, production
└── templates/                  # Starter files
    └── new-module/             # Copy for new modules
        ├── manifest.go
        ├── module.go
        ├── models/item.go
        ├── handlers/routes.go
        ├── views/templates/
        └── lang/en.json
```

## For AI Agents

1. **Start with** [CLAUDE.md](CLAUDE.md) - comprehensive guide
2. **Quick Start section** - 10-minute module bootstrap
3. **Pattern files** for specific topics
4. **templates/new-module/** - starter files to copy

## Module Structure Pattern

```
internal/modules/yourmodule/
├── manifest.go         # Module metadata
├── module.go           # Module interface
├── models/
│   └── item.go         # Model + Repository
├── handlers/
│   └── routes.go       # Gin handlers
├── views/
│   └── templates/yourmodule/
│       ├── index.html
│       └── partials/_list.html
└── lang/
    ├── en.json
    └── es.json
```

## Key Principles

1. **Modular by design** - Self-contained feature modules
2. **Fat Models** - Business logic in repositories, not handlers
3. **Server-rendered** - HTMX for interactivity, no SPA
4. **CSS-first** - Use CSS for animations, Alpine.js minimally
5. **Locality** - Keep styles in templates, not global CSS
6. **i18n ready** - Translations from day 1

## Pattern Files

| Pattern | Description |
|---------|-------------|
| [module-system.md](patterns/module-system.md) | Module interfaces, loading |
| [mvc.md](patterns/mvc.md) | Fat models, thin controllers |
| [database.md](patterns/database.md) | GORM, migrations |
| [auth.md](patterns/auth.md) | Sessions, groups |
| [frontend.md](patterns/frontend.md) | HTMX, CSS-first, minimal Alpine |
| [htmx.md](patterns/htmx.md) | Server-driven UI patterns |
| [i18n.md](patterns/i18n.md) | JSON translations |
| [security.md](patterns/security.md) | CSRF, rate limiting |
| [audit.md](patterns/audit.md) | Created/updated tracking |
| [typing.md](patterns/typing.md) | Go type patterns |
| [testing.md](patterns/testing.md) | Testing guide |
| [deployment.md](patterns/deployment.md) | Docker, production |

## License

MIT
