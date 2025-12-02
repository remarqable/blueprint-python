# Module System

> Complete guide to Blueprint's modular architecture for Go.

---

## Overview

Blueprint uses a **Go interface-based module system** where modules are self-contained packages that implement the `Module` interface. Unlike Python's dynamic discovery, Go modules are explicitly registered at compile time.

---

## Module Interface

### Core Interface

```go
// internal/system/module/hooks.go

type Module interface {
    // Manifest returns the module's metadata
    Manifest() Manifest

    // RegisterRoutes registers the module's routes on the router group
    RegisterRoutes(r *gin.RouterGroup)
}
```

### Optional Interfaces

```go
// DatabaseInitializer - for modules with database tables
type DatabaseInitializer interface {
    InitDatabase(db *gorm.DB) error
}

// TemplateProvider - for modules with custom templates
type TemplateProvider interface {
    TemplatePath() string
}

// TranslationProvider - for modules with translations
type TranslationProvider interface {
    TranslationPath() string
}
```

---

## Manifest Structure

```go
// internal/system/module/manifest.go

type ModuleType string

const (
    ModuleTypeApp    ModuleType = "App"    // User-facing, shown in UI
    ModuleTypeSystem ModuleType = "System" // Hidden, infrastructure
)

type Manifest struct {
    // Required
    Name      string     `json:"name"`       // Display name
    Version   string     `json:"version"`    // Semantic version
    MainRoute string     `json:"main_route"` // URL prefix
    Type      ModuleType `json:"type"`       // App or System
    Depends   []string   `json:"depends"`    // Dependencies

    // Display
    IconClass       string `json:"icon_class"`       // FontAwesome
    Color           string `json:"color"`            // Hex color
    Description     string `json:"description"`      // Short
    LongDescription string `json:"long_description"` // Detailed

    // Runtime (set by loader)
    Enabled bool `json:"enabled"`
}
```

---

## Module Loader

The loader handles module registration and initialization:

```go
// internal/system/module/loader.go

type Loader struct {
    modules   []Module
    manifests []Manifest
    router    *gin.Engine
    db        *gorm.DB
}

func NewLoader(router *gin.Engine, db *gorm.DB) *Loader

func (l *Loader) RegisterModule(m Module)
func (l *Loader) LoadAll() error
func (l *Loader) InitDatabases() error
func (l *Loader) GetManifests() []Manifest
func (l *Loader) GetAppModules() []Manifest
```

---

## Loading Order

1. **Core module** - Always loaded first (provides auth, base templates)
2. **All other modules** - Alphabetically by name

```go
// Sorting in loader.go
sort.Slice(l.modules, func(i, j int) bool {
    mi, mj := l.modules[i].Manifest(), l.modules[j].Manifest()
    if mi.Name == "Core" {
        return true
    }
    if mj.Name == "Core" {
        return false
    }
    return mi.Name < mj.Name
})
```

---

## Creating a Module

### Step 1: Directory Structure

```
internal/modules/yourmodule/
├── manifest.go      # Module metadata
├── module.go        # Module implementation
├── models/          # GORM models
│   └── item.go
├── handlers/        # Gin handlers
│   └── index.go
└── views/
    ├── templates/yourmodule/
    │   └── index.html
    └── lang/
        ├── en.json
        └── es.json
```

### Step 2: Manifest

```go
// internal/modules/yourmodule/manifest.go
package yourmodule

import "blueprint-go/internal/system/module"

var Manifest = module.Manifest{
    Name:            "YourModule",
    Version:         "1.0",
    MainRoute:       "/yourmodule",
    Type:            module.ModuleTypeApp,
    Depends:         []string{"core"},
    IconClass:       "fa-solid fa-cube",
    Color:           "#007bff",
    Description:     "Your module description",
    LongDescription: "Detailed description here.",
}
```

### Step 3: Module Implementation

```go
// internal/modules/yourmodule/module.go
package yourmodule

import (
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "blueprint-go/internal/modules/yourmodule/handlers"
    "blueprint-go/internal/modules/yourmodule/models"
    "blueprint-go/internal/system/auth"
    "blueprint-go/internal/system/module"
)

type YourModuleModule struct{}

func (m *YourModuleModule) Manifest() module.Manifest {
    return Manifest
}

func (m *YourModuleModule) RegisterRoutes(r *gin.RouterGroup) {
    // Public routes
    r.GET("/public", handlers.PublicPage)

    // Protected routes
    protected := r.Group("/")
    protected.Use(auth.RequireLogin())
    {
        protected.GET("/", handlers.Index)
        protected.POST("/create", handlers.Create)
    }

    // Admin routes
    admin := r.Group("/admin")
    admin.Use(auth.RequireLogin(), auth.RequireAdmin())
    {
        admin.GET("/", handlers.AdminPanel)
    }
}

func (m *YourModuleModule) InitDatabase(db *gorm.DB) error {
    // Auto-migrate models
    if err := db.AutoMigrate(&models.Item{}); err != nil {
        return err
    }

    // Seed sample data
    repo := models.NewItemRepository(db)
    return repo.CreateSampleData()
}

// Singleton instance
var Module = &YourModuleModule{}
```

### Step 4: Register in main.go

```go
// cmd/server/main.go
import "blueprint-go/internal/modules/yourmodule"

func main() {
    // ...
    application.RegisterModule(core.Module)
    application.RegisterModule(yourmodule.Module)  // Add here
    // ...
}
```

---

## Route Registration

Routes are registered on a `*gin.RouterGroup` scoped to the module's `MainRoute`:

```go
func (m *YourModuleModule) RegisterRoutes(r *gin.RouterGroup) {
    // r is already scoped to /yourmodule

    r.GET("/", handlers.Index)        // GET /yourmodule/
    r.GET("/items", handlers.List)    // GET /yourmodule/items
    r.POST("/items", handlers.Create) // POST /yourmodule/items
}
```

### Middleware

Apply middleware to route groups:

```go
func (m *YourModuleModule) RegisterRoutes(r *gin.RouterGroup) {
    // All routes require login
    r.Use(auth.RequireLogin())

    r.GET("/", handlers.Index)

    // Admin-only subroutes
    admin := r.Group("/admin")
    admin.Use(auth.RequireAdmin())
    {
        admin.GET("/", handlers.AdminPanel)
    }
}
```

---

## Database Initialization

Modules implement `DatabaseInitializer` to set up tables:

```go
func (m *YourModuleModule) InitDatabase(db *gorm.DB) error {
    // 1. Auto-migrate models
    err := db.AutoMigrate(
        &models.Item{},
        &models.Category{},
    )
    if err != nil {
        return err
    }

    // 2. Create sample data (idempotent)
    repo := models.NewItemRepository(db)
    if err := repo.CreateSampleData(); err != nil {
        return err
    }

    return nil
}
```

---

## Cross-Module Dependencies

Declare dependencies in manifest:

```go
var Manifest = module.Manifest{
    Depends: []string{"core", "users"},  // Requires core and users modules
}
```

Import from other modules:

```go
import (
    coreModels "blueprint-go/internal/modules/core/models"
)

func SomeHandler(c *gin.Context) {
    userRepo := coreModels.NewUserRepository(db.Get())
    user, _ := userRepo.GetByID(1)
}
```

---

## Disabling Modules

In Go, modules are disabled by not registering them:

```go
// cmd/server/main.go
application.RegisterModule(core.Module)
// application.RegisterModule(yourmodule.Module)  // Commented out = disabled
```

For runtime configuration:

```go
if config.EnableYourModule {
    application.RegisterModule(yourmodule.Module)
}
```

---

## Best Practices

1. **One module per feature domain** - Don't mix unrelated functionality
2. **Explicit dependencies** - List all dependencies in manifest
3. **Idempotent initialization** - `InitDatabase` should be safe to run multiple times
4. **Self-contained** - Module should work independently (except dependencies)
5. **Thin handlers** - Business logic in model repositories
6. **Translate all strings** - Use `{{T "text"}}` in templates
