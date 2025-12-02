# Blueprint Go

A modular Go web application framework using Gin, GORM, and server-rendered HTML.

## Features

- **Modular Architecture** - Self-contained modules with models, handlers, views, and translations
- **Fat Models** - Business logic in repository pattern, thin controllers
- **Server-Rendered HTML** - html/template with HTMX and Alpine.js for interactivity
- **Session Authentication** - Cookie-based auth with group permissions
- **i18n Support** - Module-scoped JSON translation files
- **Single-Tenant** - Designed for single organization deployments

## Tech Stack

| Component | Technology |
|-----------|------------|
| Web Framework | [Gin](https://gin-gonic.com/) |
| ORM | [GORM](https://gorm.io/) |
| Database | SQLite (default) |
| Migrations | [Goose](https://github.com/pressly/goose) |
| Templates | html/template |
| Frontend | HTMX + Alpine.js + Bootstrap 5 |
| Sessions | gin-contrib/sessions |

## Quick Start

### Prerequisites

- Go 1.21+
- GCC (for SQLite CGO)

### Installation

```bash
# Clone repository
git clone <repo-url>
cd blueprint-go

# Install dependencies
go mod download

# Copy environment file
cp .env.example .env

# Run the application
make run
```

### Access

- **URL**: http://localhost:8000
- **Default Admin**: admin@example.com / admin

## Project Structure

```
blueprint-go/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── app/                    # Application factory
│   ├── system/                 # Core framework
│   │   ├── db/                 # GORM database
│   │   ├── auth/               # Authentication
│   │   ├── module/             # Module loader
│   │   ├── i18n/               # Translations
│   │   └── template/           # Template engine
│   └── modules/                # Application modules
│       └── core/               # Core module (required)
│           ├── models/         # User, Group, UserSetting
│           ├── handlers/       # Auth, Dashboard
│           └── views/          # Templates, translations
├── migrations/                 # Goose SQL migrations
├── patterns/                   # Documentation
└── Makefile
```

## Creating a Module

1. **Create directory structure**:
```bash
mkdir -p internal/modules/yourmodule/{models,handlers,views/templates/yourmodule,views/lang}
```

2. **Create manifest.go**:
```go
package yourmodule

import "blueprint-go/internal/system/module"

var Manifest = module.Manifest{
    Name:      "YourModule",
    Version:   "1.0",
    MainRoute: "/yourmodule",
    Type:      module.ModuleTypeApp,
    Depends:   []string{"core"},
    IconClass: "fa-solid fa-cube",
}
```

3. **Create module.go** implementing `Module` interface
4. **Register in main.go**

See [patterns/module-system.md](patterns/module-system.md) for complete guide.

## Available Commands

```bash
make run            # Run development server
make build          # Build binary
make test           # Run tests
make test-coverage  # Run tests with coverage
make migrate-up     # Apply migrations
make migrate-down   # Rollback migration
make migrate-status # Show migration status
make fmt            # Format code
make lint           # Lint code
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECRET_KEY` | Session encryption key | `dev` |
| `DATABASE_URL` | SQLite file path | `app.db` |
| `DEBUG` | Enable debug mode | `false` |
| `PORT` | HTTP port | `8000` |

## Documentation

- [CLAUDE.md](CLAUDE.md) - AI agent guide (conventions, patterns)
- [patterns/module-system.md](patterns/module-system.md) - Module architecture
- [patterns/database.md](patterns/database.md) - GORM patterns
- [patterns/auth.md](patterns/auth.md) - Authentication
- [patterns/i18n.md](patterns/i18n.md) - Internationalization
- [patterns/frontend.md](patterns/frontend.md) - HTMX/Alpine.js
- [patterns/testing.md](patterns/testing.md) - Testing guide
- [patterns/deployment.md](patterns/deployment.md) - Deployment

## License

MIT
