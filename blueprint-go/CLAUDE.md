# CLAUDE.md - Blueprint Go

> AI agent guide for Blueprint Go. See [README.md](README.md) for setup and [patterns/](patterns/) for detailed documentation.

---

## Quick Reference

### Key Paths

| Purpose | Path |
|---------|------|
| Entry point | `cmd/server/main.go` |
| App factory | `internal/app/app.go` |
| Database | `internal/system/db/database.go` |
| Auth middleware | `internal/system/auth/middleware.go` |
| Module loader | `internal/system/module/loader.go` |
| Core module | `internal/modules/core/` |

### Run Commands

```bash
make run          # Start server (localhost:8000)
make build        # Build binary
make test         # Run tests
make migrate-up   # Apply migrations
```

---

## Architecture

```
Request → Gin Router → Auth Middleware → Handler → Repository → GORM → SQLite
                                              ↓
                                         Template → Response
```

### Module Structure

```
internal/modules/yourmodule/
├── manifest.go         # Module metadata
├── module.go           # Module interface implementation
├── models/
│   └── item.go         # Model + Repository (fat model pattern)
├── handlers/
│   └── item.go         # HTTP handlers (thin controllers)
└── views/
    ├── templates/yourmodule/
    │   ├── index.html
    │   └── partials/_list.html
    └── lang/
        ├── en.json
        └── es.json
```

---

## Core Patterns

### Module Interface

```go
type Module interface {
    Manifest() Manifest
    RegisterRoutes(r *gin.RouterGroup)
}

type DatabaseInitializer interface {
    InitDatabase(db *gorm.DB) error
}
```

### Fat Model (Repository Pattern)

```go
// models/item.go
type ItemRepository struct {
    db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
    return &ItemRepository{db: db}
}

func (r *ItemRepository) Create(name string) (*Item, error) {
    item := &Item{Name: name}
    err := r.db.Create(item).Error
    return item, err
}

func (r *ItemRepository) GetByID(id uint) (*Item, error) {
    var item Item
    err := r.db.First(&item, id).Error
    return &item, err
}
```

### Thin Handler

```go
// handlers/item.go
func (h *ItemHandler) Create(c *gin.Context) {
    name := c.PostForm("name")
    item, err := h.itemRepo.Create(name)
    if err != nil {
        c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": err})
        return
    }
    c.Redirect(http.StatusFound, "/items")
}
```

### Auth Middleware

```go
// Protect routes
protected := r.Group("")
protected.Use(auth.RequireLogin())

admin := r.Group("/admin")
admin.Use(auth.RequireLogin(), auth.RequireAdmin())

// Get current user
user := auth.GetUser(c)
```

### Templates

```html
{{define "content"}}
<h1>{{T "Items"}}</h1>
<button hx-get="/items/new" hx-target="#modal">{{T "Add"}}</button>
{{end}}
```

### Translations

```json
// views/lang/en.json
{ "Items": "Items", "Add": "Add" }
```

---

## Creating a Module

### 1. Create Structure

```bash
MODULE=yourmodule
mkdir -p internal/modules/$MODULE/{models,handlers,views/templates/$MODULE,views/lang}
```

### 2. Create manifest.go

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
    Color:     "#007bff",
}
```

### 3. Create module.go

```go
package yourmodule

import (
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    "blueprint-go/internal/system/module"
)

type YourModule struct{}

func (m *YourModule) Manifest() module.Manifest {
    return Manifest
}

func (m *YourModule) RegisterRoutes(r *gin.RouterGroup) {
    r.GET("/", m.Index)
}

func (m *YourModule) InitDatabase(db *gorm.DB) error {
    return db.AutoMigrate(&models.Item{})
}

func (m *YourModule) Index(c *gin.Context) {
    c.HTML(200, "yourmodule/index.html", nil)
}
```

### 4. Register in main.go

```go
import "blueprint-go/internal/modules/yourmodule"

loader.Register(&yourmodule.YourModule{})
```

---

## Conventions

| Convention | Example |
|------------|---------|
| Module names | lowercase: `yourmodule` |
| Table names | `modulename_items` |
| Blueprint names | `yourmodule_bp` |
| Template path | `yourmodule/index.html` |
| Partial prefix | `_list.html` |
| Translation key | English text: `"Save"` |

---

## Detailed Guides

| Topic | Documentation |
|-------|---------------|
| Module system | [patterns/module-system.md](patterns/module-system.md) |
| Database/GORM | [patterns/database.md](patterns/database.md) |
| Authentication | [patterns/auth.md](patterns/auth.md) |
| Translations | [patterns/i18n.md](patterns/i18n.md) |
| HTMX/Alpine.js | [patterns/frontend.md](patterns/frontend.md) |
| Testing | [patterns/testing.md](patterns/testing.md) |
| Deployment | [patterns/deployment.md](patterns/deployment.md) |

---

## Built-in Groups

- `ADMIN` - Full access
- `USER` - Standard users
- `ALL` - All authenticated users

---

## File Checklist for New Module

- [ ] `internal/modules/yourmodule/manifest.go`
- [ ] `internal/modules/yourmodule/module.go`
- [ ] `internal/modules/yourmodule/models/*.go`
- [ ] `internal/modules/yourmodule/handlers/*.go`
- [ ] `internal/modules/yourmodule/views/templates/yourmodule/*.html`
- [ ] `internal/modules/yourmodule/views/lang/en.json`
- [ ] Register in `cmd/server/main.go`
