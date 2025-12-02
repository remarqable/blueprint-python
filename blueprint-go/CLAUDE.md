# CLAUDE.md - Modular Go Blueprint

> **Master guide for building modular Go web applications with Gin, GORM, and HTMX.**
> Documentation-only blueprint for AI/LLM agents to follow when creating Go applications.
> Optimized for **extensibility**, **clean code**, and **AI agent execution**.

---

## Quick Start (New Module Bootstrap)

**AI Agents: Execute this section to create a new module (10 minutes)**

### Prerequisites
- [ ] Existing Blueprint Go application running
- [ ] Understanding of the module you want to create

### Bootstrap Sequence (Follow in Order)

**Step 1: Create Module Directory** (1 min)
```bash
MODULE_NAME="yourmodule"  # lowercase, no spaces
mkdir -p internal/modules/$MODULE_NAME/{models,handlers,views/templates/$MODULE_NAME/partials,lang}
```
Verify: `ls internal/modules/$MODULE_NAME` shows folder structure

**Step 2: Create Manifest** (1 min)
```go
// internal/modules/yourmodule/manifest.go
package yourmodule

var Manifest = ModuleManifest{
    Name:        "YourModule",
    Version:     "1.0.0",
    MainRoute:   "/yourmodule",
    Type:        "App",  // or "System"
    Depends:     []string{"core"},
    IconClass:   "fa-solid fa-cube",
    Color:       "#007bff",
    Description: "Short description of your module",
}

type ModuleManifest struct {
    Name        string   `json:"name"`
    Version     string   `json:"version"`
    MainRoute   string   `json:"main_route"`
    Type        string   `json:"type"`
    Depends     []string `json:"depends"`
    IconClass   string   `json:"icon_class"`
    Color       string   `json:"color"`
    Description string   `json:"description"`
}
```

**Step 3: Create Module Class** (2 min)
```go
// internal/modules/yourmodule/module.go
package yourmodule

import (
    "yourapp/internal/modules/yourmodule/handlers"
    "yourapp/internal/modules/yourmodule/models"
    "gorm.io/gorm"
)

type Module struct {
    db      *gorm.DB
    handler *handlers.Handler
}

func New(db *gorm.DB) *Module {
    repo := models.NewItemRepository(db)
    return &Module{
        db:      db,
        handler: handlers.NewHandler(repo),
    }
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    m.handler.RegisterRoutes(r)
}

func (m *Module) InitDatabase() error {
    return m.db.AutoMigrate(&models.Item{})
}
```

**Step 4: Create Model** (2 min)
```go
// internal/modules/yourmodule/models/item.go
package models

import (
    "time"
    "gorm.io/gorm"
)

type Item struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    Name      string         `gorm:"size:255;not null" json:"name"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Item) TableName() string {
    return "yourmodule_items"
}

type ItemRepository struct {
    db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
    return &ItemRepository{db: db}
}

func (r *ItemRepository) GetAll() ([]Item, error) {
    var items []Item
    err := r.db.Find(&items).Error
    return items, err
}

func (r *ItemRepository) Create(item *Item) error {
    return r.db.Create(item).Error
}
```

**Step 5: Create Handler** (2 min)
```go
// internal/modules/yourmodule/handlers/routes.go
package handlers

import (
    "net/http"
    "yourapp/internal/modules/yourmodule/models"
    "github.com/gin-gonic/gin"
)

type Handler struct {
    itemRepo *models.ItemRepository
}

func NewHandler(repo *models.ItemRepository) *Handler {
    return &Handler{itemRepo: repo}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
    r.GET("/", h.Index)
    r.POST("/", h.Create)
}

func (h *Handler) Index(c *gin.Context) {
    items, _ := h.itemRepo.GetAll()
    c.HTML(http.StatusOK, "yourmodule/index.html", gin.H{
        "items": items,
    })
}
```

**Step 6: Create Template** (2 min)
```html
<!-- internal/modules/yourmodule/views/templates/yourmodule/index.html -->
{{define "content"}}
<div class="container mt-4">
    <h1>{{T "Your Module"}}</h1>
    <p>{{T "Welcome to your new module!"}}</p>
</div>
{{end}}
```

**Step 7: Create Translations** (1 min)
```json
// internal/modules/yourmodule/lang/en.json
{
    "Your Module": "Your Module",
    "Welcome to your new module!": "Welcome to your new module!"
}
```

**Step 8: Register Module & Run**
Register in your module loader and run the application.

**Expected Time: 10 minutes to working module**

---

## Table of Contents

### Foundation
- [Philosophy](#philosophy)
- [Architecture Overview](#architecture-overview)
- [Folder Structure](#folder-structure)
- [Tech Stack](#tech-stack)

### Module System
- [Module System](#module-system) → [patterns/module-system.md](patterns/module-system.md)
- [Manifest Reference](#manifest-reference)
- [Module Lifecycle](#module-lifecycle)

### MVC Pattern
- [Models](#models) → [patterns/mvc.md](patterns/mvc.md)
- [Handlers (Controllers)](#handlers)
- [Views](#views)

### Platform Components
- [Database](#database) → [patterns/database.md](patterns/database.md)
- [Authentication](#authentication) → [patterns/auth.md](patterns/auth.md)
- [Internationalization](#internationalization) → [patterns/i18n.md](patterns/i18n.md)
- [Frontend](#frontend) → [patterns/frontend.md](patterns/frontend.md)
- [HTMX](#htmx) → [patterns/htmx.md](patterns/htmx.md)

### Security & Quality
- [Security](#security) → [patterns/security.md](patterns/security.md)
- [Audit Trail](#audit-trail) → [patterns/audit.md](patterns/audit.md)
- [Type Safety](#type-safety) → [patterns/typing.md](patterns/typing.md)

### Operations
- [Testing](#testing) → [patterns/testing.md](patterns/testing.md)
- [Deployment](#deployment) → [patterns/deployment.md](patterns/deployment.md)

---

## Philosophy

### Core Principles

1. **Modular by design** - Features are self-contained modules that can be enabled/disabled
2. **Fat Models, Thin Controllers** - Business logic lives in repositories, handlers are thin
3. **Server-rendered HTML + HTMX** - No SPA complexity, server-driven interactivity
4. **CSS-first, minimal Alpine.js** - Use CSS for animations, Alpine only when necessary
5. **Locality of behavior** - Keep styles close to where they're used
6. **i18n from day 1** - Module-scoped JSON translations
7. **Convention over configuration** - Standard patterns reduce decisions

### Single-Tenant Architecture

This blueprint is designed for **single-tenant applications**:

- One database serves one organization/deployment
- All users belong to the same implicit organization
- Access control via groups (ADMIN, USER, etc.)
- No multi-tenant data isolation required

**When to Use:**
- Internal tools
- Department-specific apps
- Single organization SaaS
- Self-hosted applications

### Frontend Philosophy

**CSS-First Approach:**
- Use CSS transitions/animations for visual effects
- Use CSS `:hover` states instead of JavaScript
- Use `<details>/<summary>` for accordions
- Only use Alpine.js when CSS cannot achieve the goal

**Locality of Behavior:**
- Keep component CSS in template `<style>` blocks
- Only extract to CSS files when used in 3+ places
- Prefix component classes with module name

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Gin Application                          │
│                       (main.go)                              │
└────────────────────────────┬────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐  ┌───────────────┐  ┌────────────────┐
│  Module Loader  │  │   Database    │  │   i18n System  │
│                 │  │    (GORM)     │  │    (JSON)      │
└────────┬────────┘  └───────────────┘  └────────────────┘
         │
         │ discovers & loads
         ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/modules/                          │
├─────────────┬─────────────┬──────────────────────────────────┤
│    core     │   tasks     │        [yourmodule]              │
│  (System)   │   (App)     │           (App)                  │
│  required   │  optional   │      your modules here           │
└─────────────┴─────────────┴──────────────────────────────────┘
```

### Request Flow

```
HTTP Request → Gin Router → Middleware (Auth, CSRF) → Handler → Repository → GORM → Database
                                                         ↓
HTTP Response ← Template Engine ← gin.H data ←───────────┘
```

### Loading Order
1. **Core module** (required) - Base templates, auth infrastructure
2. **All other modules** (alphabetically) - Application modules

---

## Folder Structure

```
yourapp/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
│
├── internal/
│   ├── system/                  # Core framework (don't modify often)
│   │   ├── db/
│   │   │   ├── database.go      # GORM instance
│   │   │   └── audit.go         # Audit fields
│   │   ├── auth/
│   │   │   ├── middleware.go    # Auth middleware
│   │   │   └── session.go       # Session management
│   │   ├── i18n/
│   │   │   └── translation.go   # Translation functions
│   │   ├── middleware/
│   │   │   ├── csrf.go          # CSRF protection
│   │   │   ├── ratelimit.go     # Rate limiting
│   │   │   └── security.go      # Security headers
│   │   └── module/
│   │       └── interfaces.go    # Module interfaces
│   │
│   └── modules/                 # Dynamic modules
│       ├── core/                # System module (required)
│       │   ├── manifest.go
│       │   ├── module.go
│       │   ├── models/
│       │   │   ├── user.go
│       │   │   └── group.go
│       │   ├── handlers/
│       │   │   └── routes.go
│       │   ├── views/
│       │   │   └── templates/
│       │   │       ├── base.html
│       │   │       ├── header.html
│       │   │       └── ...
│       │   └── lang/
│       │       ├── en.json
│       │       └── es.json
│       │
│       └── [yourmodule]/        # Your modules here
│           ├── manifest.go
│           ├── module.go
│           ├── models/
│           ├── handlers/
│           ├── views/
│           │   └── templates/[yourmodule]/
│           │       ├── index.html
│           │       └── partials/
│           └── lang/
│
├── migrations/                  # Goose migrations
├── static/                      # Static assets
├── go.mod
├── go.sum
└── .env
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Language** | Go 1.21+ | Performance, static typing, single binary |
| **Web Framework** | Gin | Fast, mature, middleware support |
| **Database** | SQLite/PostgreSQL | SQLite for dev, PostgreSQL for prod |
| **ORM** | GORM | Auto-migrations, relationships, soft delete |
| **Migrations** | Goose | SQL-based, version control |
| **Templates** | html/template | Auto-escaping, Go-native |
| **Frontend JS** | HTMX | Server-driven updates |
| **Frontend JS** | Alpine.js (minimal) | Only for client-side state |
| **CSS** | Bootstrap 5 | No build step, responsive |
| **Icons** | FontAwesome 6 | Wide icon selection |
| **Sessions** | gin-contrib/sessions | Cookie/Redis sessions |
| **i18n** | JSON catalogs | Simple, module-scoped |

---

## Module System

→ **See complete guide:** [patterns/module-system.md](patterns/module-system.md)

### Module Interface

```go
// internal/system/module/interfaces.go
type Module interface {
    RegisterRoutes(r *gin.RouterGroup)
}

type DatabaseInitializer interface {
    InitDatabase() error
}

type Manifest interface {
    GetManifest() ModuleManifest
}
```

### Manifest Reference

```go
var Manifest = ModuleManifest{
    Name:        "Tasks",           // Display name
    Version:     "1.0.0",           // Semantic version
    MainRoute:   "/tasks",          // URL prefix
    Type:        "App",             // "App" or "System"
    Depends:     []string{"core"},  // Dependencies
    IconClass:   "fa-solid fa-check",
    Color:       "#28a745",
    Description: "Task management",
}
```

**Type Values:**
- `"App"` - Shows in app switcher, user-facing
- `"System"` - Hidden from app switcher, infrastructure

---

## Models

Models follow the **Fat Model** pattern with repositories:

```go
// internal/modules/yourmodule/models/item.go
package models

import (
    "errors"
    "strings"
    "time"
    "gorm.io/gorm"
)

type Item struct {
    ID          uint           `gorm:"primaryKey" json:"id"`
    Name        string         `gorm:"size:255;not null" json:"name"`
    Description string         `gorm:"type:text" json:"description"`
    IsActive    bool           `gorm:"default:true" json:"is_active"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Item) TableName() string {
    return "yourmodule_items"
}

func (i *Item) Validate() error {
    i.Name = strings.TrimSpace(i.Name)
    if i.Name == "" {
        return errors.New("name is required")
    }
    if len(i.Name) > 255 {
        return errors.New("name too long")
    }
    return nil
}

// Repository encapsulates all database operations
type ItemRepository struct {
    db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
    return &ItemRepository{db: db}
}

func (r *ItemRepository) Create(item *Item) error {
    if err := item.Validate(); err != nil {
        return err
    }
    return r.db.Create(item).Error
}

func (r *ItemRepository) GetByID(id uint) (*Item, error) {
    var item Item
    err := r.db.First(&item, id).Error
    return &item, err
}

func (r *ItemRepository) GetAll() ([]Item, error) {
    var items []Item
    err := r.db.Where("is_active = ?", true).Order("created_at DESC").Find(&items).Error
    return items, err
}

func (r *ItemRepository) Update(item *Item) error {
    if err := item.Validate(); err != nil {
        return err
    }
    return r.db.Save(item).Error
}

func (r *ItemRepository) Delete(id uint) error {
    return r.db.Delete(&Item{}, id).Error
}
```

→ **See complete guide:** [patterns/mvc.md](patterns/mvc.md)

---

## Handlers

Handlers are **thin controllers** - parse input, call repository, render response:

```go
// internal/modules/yourmodule/handlers/routes.go
package handlers

import (
    "net/http"
    "strconv"
    "yourapp/internal/modules/yourmodule/models"
    "yourapp/internal/system/auth"
    "github.com/gin-gonic/gin"
)

type Handler struct {
    itemRepo *models.ItemRepository
}

func NewHandler(repo *models.ItemRepository) *Handler {
    return &Handler{itemRepo: repo}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
    r.Use(auth.RequireLogin())

    r.GET("/", h.Index)
    r.GET("/:id", h.Show)
    r.POST("/", h.Create)
    r.PUT("/:id", h.Update)
    r.DELETE("/:id", h.Delete)
}

func (h *Handler) Index(c *gin.Context) {
    items, err := h.itemRepo.GetAll()
    if err != nil {
        c.HTML(http.StatusInternalServerError, "errors/500.html", nil)
        return
    }

    // Return partial for HTMX
    if c.GetHeader("HX-Request") == "true" {
        c.HTML(http.StatusOK, "yourmodule/partials/_list.html", gin.H{
            "items": items,
        })
        return
    }

    c.HTML(http.StatusOK, "yourmodule/index.html", gin.H{
        "items": items,
    })
}

func (h *Handler) Create(c *gin.Context) {
    item := &models.Item{
        Name:        c.PostForm("name"),
        Description: c.PostForm("description"),
    }

    if err := h.itemRepo.Create(item); err != nil {
        if c.GetHeader("HX-Request") == "true" {
            c.HTML(http.StatusBadRequest, "yourmodule/partials/_error.html", gin.H{
                "error": err.Error(),
            })
            return
        }
        c.HTML(http.StatusBadRequest, "yourmodule/new.html", gin.H{
            "error": err.Error(),
        })
        return
    }

    if c.GetHeader("HX-Request") == "true" {
        c.Header("HX-Trigger", `{"showToast": {"message": "Created!"}}`)
        c.HTML(http.StatusCreated, "yourmodule/partials/_item.html", gin.H{
            "item": item,
        })
        return
    }

    c.Redirect(http.StatusSeeOther, "/yourmodule")
}
```

---

## Views

Templates use Go's `html/template` with template inheritance:

```html
<!-- internal/modules/yourmodule/views/templates/yourmodule/index.html -->
{{define "title"}}Items{{end}}

{{define "styles"}}
<style>
/* Component-specific CSS - locality of behavior */
.yourmodule-card {
    transition: transform 0.2s ease;
}
.yourmodule-card:hover {
    transform: translateY(-2px);
}
</style>
{{end}}

{{define "content"}}
<div class="container mt-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
        <h1>{{T "Items"}}</h1>
        <button class="btn btn-primary"
                hx-get="/yourmodule/new"
                hx-target="#modal-container">
            <i class="fas fa-plus me-1"></i> {{T "Add Item"}}
        </button>
    </div>

    <div id="item-list">
        {{range .items}}
        <div class="card yourmodule-card mb-3" id="item-{{.ID}}">
            <div class="card-body">
                <h5>{{.Name}}</h5>
                <p class="text-muted">{{.Description}}</p>
            </div>
        </div>
        {{end}}
    </div>
</div>

<div id="modal-container"></div>
{{end}}
```

### Template Organization

```
views/templates/yourmodule/
├── index.html              # Main list page
├── show.html               # Detail view
├── new.html                # Create form
├── edit.html               # Edit form
└── partials/
    ├── _list.html          # List partial (HTMX)
    ├── _item.html          # Single item (HTMX)
    ├── _form.html          # Form partial
    └── _modal.html         # Modal partial
```

**Naming Conventions:**
- Full pages: `name.html`
- Partials: `_name.html` (underscore prefix)
- Keep partials in `partials/` subdirectory

---

## Frontend

→ **See complete guide:** [patterns/frontend.md](patterns/frontend.md)

### CSS-First Approach

```html
<!-- DO: Use CSS for hover effects -->
<style>
.card:hover { transform: translateY(-2px); }
</style>

<!-- DON'T: Use Alpine for hover -->
<div x-data="{ hover: false }" @mouseenter="hover = true">
```

### When to Use Alpine.js

Only use Alpine.js for:
1. Click toggles (show/hide panels)
2. Form state before submit
3. Complex conditional rendering

```html
<!-- Alpine only when CSS can't do it -->
<div x-data="{ open: false }">
    <button @click="open = !open">Toggle</button>
    <div x-show="open">Content</div>
</div>
```

### HTMX for Server Interaction

```html
<button hx-get="/items/list"
        hx-target="#item-list"
        hx-swap="innerHTML">
    Refresh
</button>

<form hx-post="/items"
      hx-target="#item-list"
      hx-swap="afterbegin">
    <input type="text" name="name" required>
    <button type="submit">Add</button>
</form>
```

→ **See complete guide:** [patterns/htmx.md](patterns/htmx.md)

---

## Internationalization

Module-scoped JSON translations:

```json
// internal/modules/yourmodule/lang/en.json
{
    "Items": "Items",
    "Add Item": "Add Item",
    "Item created": "Item created"
}

// internal/modules/yourmodule/lang/es.json
{
    "Items": "Artículos",
    "Add Item": "Agregar Artículo",
    "Item created": "Artículo creado"
}
```

**Usage in templates:**
```html
{{T "Items"}}
```

**Fallback Chain:**
1. Module-specific translation
2. Core module translation
3. Original text

→ **See complete guide:** [patterns/i18n.md](patterns/i18n.md)

---

## Authentication

Session-based authentication with group-based access control:

```go
// Middleware
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
    r.Use(auth.RequireLogin())      // All routes require login
    r.GET("/", h.Index)

    admin := r.Group("/admin")
    admin.Use(auth.RequireAdmin())  // Admin-only routes
    admin.GET("/settings", h.AdminSettings)
}

// In handler
func (h *Handler) Index(c *gin.Context) {
    user := auth.GetUser(c)
    if user == nil {
        c.Redirect(http.StatusSeeOther, "/login")
        return
    }
    // ...
}
```

**Built-in Groups:**
- `ALL` - All authenticated users
- `ADMIN` - Administrators

→ **See complete guide:** [patterns/auth.md](patterns/auth.md)

---

## Security

→ **See complete guide:** [patterns/security.md](patterns/security.md)

### Security Checklist

- [ ] CSRF protection on all state-changing requests
- [ ] Rate limiting on auth endpoints
- [ ] Input validation (length, format, type)
- [ ] Parameterized queries (GORM handles)
- [ ] Auto-escaping templates (html/template default)
- [ ] Secure sessions (HttpOnly, Secure, SameSite)
- [ ] HTTPS only in production
- [ ] Security headers configured

### CSRF Protection

```html
<body hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>

<form method="POST">
    <input type="hidden" name="csrf_token" value="{{.csrf_token}}">
</form>
```

### Password Hashing

```go
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

---

## Audit Trail

Track who created and modified records:

```go
type AuditFields struct {
    CreatedByID *uint     `gorm:"index" json:"created_by_id,omitempty"`
    UpdatedByID *uint     `gorm:"index" json:"updated_by_id,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Item struct {
    ID   uint   `gorm:"primaryKey"`
    Name string `gorm:"size:255"`
    AuditFields
}
```

→ **See complete guide:** [patterns/audit.md](patterns/audit.md)

---

## Type Safety

Go is statically typed - the compiler enforces types:

```go
// Compiler catches errors
var count int = "hello"  // Error: cannot use string as int

// Interface satisfaction check
var _ Module = (*YourModule)(nil)

// Generics for reusable code
type Repository[T any] struct {
    db *gorm.DB
}

func (r *Repository[T]) GetByID(id uint) (*T, error) {
    var entity T
    err := r.db.First(&entity, id).Error
    return &entity, err
}
```

→ **See complete guide:** [patterns/typing.md](patterns/typing.md)

---

## Testing

```go
// tests/yourmodule_test.go
package tests

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestItemIndex(t *testing.T) {
    gin.SetMode(gin.TestMode)
    router := setupTestRouter()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/yourmodule", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestItemCreate(t *testing.T) {
    gin.SetMode(gin.TestMode)
    router := setupTestRouter()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/yourmodule", strings.NewReader("name=Test"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusSeeOther, w.Code)
}
```

→ **See complete guide:** [patterns/testing.md](patterns/testing.md)

---

## Deployment

### Development
```bash
go run cmd/server/main.go
```

### Production
```bash
# Build binary
go build -o app cmd/server/main.go

# Run with environment variables
SECRET_KEY=your-secret-key \
DATABASE_URL=postgres://... \
./app
```

### Docker
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app cmd/server/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/app .
COPY --from=builder /app/internal/modules/*/views/templates ./templates
EXPOSE 8080
CMD ["./app"]
```

→ **See complete guide:** [patterns/deployment.md](patterns/deployment.md)

---

## AI Agent Instructions

### For Claude/AI Assistants

**When creating a new module:**
1. Read § Quick Start (top of file)
2. Execute steps 1-8 in order
3. Verify: Module appears in app

**When modifying existing module:**
1. Read the module's `manifest.go` to understand it
2. Follow MVC pattern (fat models, thin handlers)
3. Use `{{T "text"}}` for all strings
4. Add translations to `lang/*.json`
5. Keep CSS in template `<style>` blocks

**When troubleshooting:**
1. Check `internal/modules/*/` folder structure
2. Verify module is registered in loader
3. Check for migration errors
4. Review console output for errors

### Files to Read (in order)
1. `CLAUDE.md` - master blueprint (this file)
2. `patterns/module-system.md` - module architecture
3. `patterns/mvc.md` - model/handler patterns
4. Other `patterns/*.md` as needed

### Best Practices

**DO:**
- Use repository pattern for database operations
- Return partials for HTMX requests
- Validate input in repository methods
- Use CSS for animations
- Keep handlers thin

**DON'T:**
- Put business logic in handlers
- Use Alpine.js for hover effects
- Create global CSS for one-off styles
- Skip input validation
- Commit secrets to git

---

## Conventions

| Convention | Example |
|------------|---------|
| Module names | lowercase: `yourmodule` |
| Table names | `modulename_items` |
| Template path | `yourmodule/index.html` |
| Partial prefix | `_list.html` |
| Translation key | English text: `"Save"` |
| CSS prefix | `.yourmodule-card` |

---

## Pattern Files Reference

| Pattern | Description |
|---------|-------------|
| [module-system.md](patterns/module-system.md) | Module interfaces, loading, dependencies |
| [mvc.md](patterns/mvc.md) | Fat models, thin controllers, repositories |
| [database.md](patterns/database.md) | GORM, migrations, queries |
| [auth.md](patterns/auth.md) | Sessions, groups, middleware |
| [frontend.md](patterns/frontend.md) | HTMX, minimal Alpine, CSS-first |
| [htmx.md](patterns/htmx.md) | Server-driven interactivity patterns |
| [i18n.md](patterns/i18n.md) | JSON translations, fallback chain |
| [security.md](patterns/security.md) | CSRF, rate limiting, validation |
| [audit.md](patterns/audit.md) | Created/updated by tracking |
| [typing.md](patterns/typing.md) | Go type patterns, generics |
| [testing.md](patterns/testing.md) | Unit tests, integration tests |
| [deployment.md](patterns/deployment.md) | Docker, production config |

---

## Templates Reference

The `templates/new-module/` directory contains starter files:

```
templates/new-module/
├── README.md           # Usage instructions
├── manifest.go         # Module metadata
├── module.go           # Module interface
├── models/item.go      # Model + repository
├── handlers/routes.go  # Gin handlers
├── views/templates/    # HTML templates
└── lang/en.json        # Translations
```

Copy and customize for new modules.

---

## File Checklist for New Module

- [ ] `internal/modules/yourmodule/manifest.go`
- [ ] `internal/modules/yourmodule/module.go`
- [ ] `internal/modules/yourmodule/models/*.go`
- [ ] `internal/modules/yourmodule/handlers/*.go`
- [ ] `internal/modules/yourmodule/views/templates/yourmodule/*.html`
- [ ] `internal/modules/yourmodule/lang/en.json`
- [ ] Register in module loader

---

**That's it.** One file to guide any Blueprint Go application.
**Build modules, extend features, scale cleanly.**
