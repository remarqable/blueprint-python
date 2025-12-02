# MVC Pattern Reference

> Complete guide to Models, Views, and Controllers in Go/Gin applications

---

## Table of Contents

- [Philosophy](#philosophy)
- [Models (Fat Models)](#models-fat-models)
- [Views (Templates)](#views-templates)
- [Controllers (Thin Handlers)](#controllers-thin-handlers)
- [Request Flow](#request-flow)
- [Best Practices](#best-practices)

---

## Philosophy

### Fat Models, Thin Controllers

**Models** contain:
- Business logic (validation, calculations)
- Database access (CRUD, queries) via Repository pattern
- Domain rules

**Controllers (Handlers)** contain:
- Parse input (query params, form data, JSON)
- Call repository methods
- Render view or return JSON
- Handle HTTP-specific concerns (status codes, headers)

**Views** contain:
- html/template files
- Minimal logic (loops, conditions)
- HTMX attributes for interactivity

---

## Models (Fat Models)

### Base Model Pattern

```go
// system/db/models.go
package db

import (
    "time"
    "gorm.io/gorm"
)

// BaseModel provides common fields for all models
type BaseModel struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

### User Model (Primary Example)

```go
// modules/core/models/user.go
package models

import (
    "errors"
    "regexp"
    "strings"

    "golang.org/x/crypto/bcrypt"
    "gorm.io/gorm"
    "yourapp/system/db"
)

func init() {
    db.Register("core", "User", "users")
}

// User model with authentication support
type User struct {
    db.BaseModel
    Email        string  `gorm:"uniqueIndex;size:255;not null" json:"email"`
    PasswordHash string  `gorm:"size:255" json:"-"`
    FirstName    string  `gorm:"size:100" json:"first_name"`
    LastName     string  `gorm:"size:100" json:"last_name"`
    IsActive     bool    `gorm:"default:true" json:"is_active"`
    IsAdmin      bool    `gorm:"default:false" json:"is_admin"`
    Groups       []Group `gorm:"many2many:user_groups;" json:"groups,omitempty"`
}

func (User) TableName() string {
    return "users"
}

// Validation
func (u *User) Validate() error {
    u.Email = strings.TrimSpace(strings.ToLower(u.Email))
    u.FirstName = strings.TrimSpace(u.FirstName)
    u.LastName = strings.TrimSpace(u.LastName)

    if u.Email == "" {
        return errors.New("email is required")
    }
    if !isValidEmail(u.Email) {
        return errors.New("invalid email format")
    }
    if len(u.FirstName) > 100 {
        return errors.New("first name too long (max 100)")
    }
    return nil
}

func isValidEmail(email string) bool {
    pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
    matched, _ := regexp.MatchString(pattern, email)
    return matched
}

// Password methods
func (u *User) SetPassword(password string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    u.PasswordHash = string(hash)
    return nil
}

func (u *User) CheckPassword(password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
    return err == nil
}

// Display helpers
func (u *User) FullName() string {
    return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

func (u *User) Initials() string {
    initials := ""
    if u.FirstName != "" {
        initials += string(u.FirstName[0])
    }
    if u.LastName != "" {
        initials += string(u.LastName[0])
    }
    if initials == "" && u.Email != "" {
        initials = string(u.Email[0])
    }
    return strings.ToUpper(initials)
}
```

### Repository Pattern

```go
// modules/core/models/user_repository.go
package models

import (
    "gorm.io/gorm"
)

// UserRepository handles all User database operations
type UserRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
    return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(email, password, firstName, lastName string, isAdmin bool) (*User, error) {
    user := &User{
        Email:     email,
        FirstName: firstName,
        LastName:  lastName,
        IsAdmin:   isAdmin,
        IsActive:  true,
    }

    if err := user.Validate(); err != nil {
        return nil, err
    }

    if err := user.SetPassword(password); err != nil {
        return nil, err
    }

    if err := r.db.Create(user).Error; err != nil {
        return nil, err
    }

    return user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uint) (*User, error) {
    var user User
    err := r.db.Preload("Groups").First(&user, id).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(email string) (*User, error) {
    var user User
    err := r.db.Preload("Groups").Where("email = ?", strings.ToLower(email)).First(&user).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}

// GetAll retrieves all users with pagination
func (r *UserRepository) GetAll(limit, offset int) ([]User, error) {
    var users []User
    err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error
    return users, err
}

// Update updates a user
func (r *UserRepository) Update(id uint, updates map[string]interface{}) (*User, error) {
    user, err := r.GetByID(id)
    if err != nil {
        return nil, err
    }

    if err := r.db.Model(user).Updates(updates).Error; err != nil {
        return nil, err
    }

    return user, nil
}

// Delete soft-deletes a user
func (r *UserRepository) Delete(id uint) error {
    return r.db.Delete(&User{}, id).Error
}

// CreateSampleData creates initial data (idempotent)
func (r *UserRepository) CreateSampleData() error {
    var count int64
    r.db.Model(&User{}).Count(&count)
    if count > 0 {
        return nil
    }

    _, err := r.Create("admin@example.com", "admin", "Admin", "User", true)
    return err
}
```

### Setting Model (Key-Value Pattern)

```go
// modules/core/models/user_setting.go
package models

import (
    "gorm.io/gorm"
    "yourapp/system/db"
)

func init() {
    db.Register("core", "UserSetting", "user_settings")
}

type UserSetting struct {
    db.BaseModel
    UserID uint   `gorm:"index;not null" json:"user_id"`
    Key    string `gorm:"size:100;not null" json:"key"`
    Value  string `gorm:"type:text" json:"value"`
}

func (UserSetting) TableName() string {
    return "user_settings"
}

// UserSettingRepository handles user settings
type UserSettingRepository struct {
    db *gorm.DB
}

func NewUserSettingRepository(db *gorm.DB) *UserSettingRepository {
    return &UserSettingRepository{db: db}
}

// Get retrieves a specific setting
func (r *UserSettingRepository) Get(userID uint, key string) (*UserSetting, error) {
    var setting UserSetting
    err := r.db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error
    return &setting, err
}

// GetValue retrieves value with default fallback
func (r *UserSettingRepository) GetValue(userID uint, key, defaultValue string) string {
    setting, err := r.Get(userID, key)
    if err != nil {
        return defaultValue
    }
    return setting.Value
}

// GetMap retrieves all settings for a user as a map
func (r *UserSettingRepository) GetMap(userID uint) map[string]string {
    var settings []UserSetting
    r.db.Where("user_id = ?", userID).Find(&settings)

    result := make(map[string]string)
    for _, s := range settings {
        result[s.Key] = s.Value
    }
    return result
}

// Set creates or updates a setting (upsert)
func (r *UserSettingRepository) Set(userID uint, key, value string) error {
    var setting UserSetting
    err := r.db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error

    if err == gorm.ErrRecordNotFound {
        setting = UserSetting{UserID: userID, Key: key, Value: value}
        return r.db.Create(&setting).Error
    }

    setting.Value = value
    return r.db.Save(&setting).Error
}
```

---

## Views (Templates)

### Base Layout

```html
<!-- modules/core/views/templates/base.html -->
{{define "base"}}
<!DOCTYPE html>
<html lang="{{.lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Your App{{end}}</title>

    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" rel="stylesheet">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>

    {{block "styles" .}}{{end}}
</head>
<body hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>
    {{template "header" .}}

    <main class="container py-4">
        {{template "flashes" .}}
        {{block "content" .}}{{end}}
    </main>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    {{block "scripts" .}}{{end}}
</body>
</html>
{{end}}
```

### Flash Messages Template

```html
<!-- modules/core/views/templates/partials/_flashes.html -->
{{define "flashes"}}
{{range .flashes}}
<div class="alert alert-{{if eq .Type "error"}}danger{{else}}{{.Type}}{{end}} alert-dismissible fade show">
    {{.Message}}
    <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
</div>
{{end}}
{{end}}
```

### User Profile Page

```html
<!-- modules/core/views/templates/core/profile.html -->
{{define "title"}}{{.user.FullName}} - Profile{{end}}

{{define "content"}}
<div class="row justify-content-center">
    <div class="col-md-8">
        <div class="card">
            <div class="card-body">
                <div class="d-flex align-items-center mb-4">
                    <div class="rounded-circle bg-primary text-white d-flex align-items-center justify-content-center me-3"
                         style="width: 80px; height: 80px; font-size: 2rem;">
                        {{.user.Initials}}
                    </div>
                    <div class="flex-grow-1">
                        <h2 class="mb-0">{{.user.FullName}}</h2>
                        <p class="text-muted mb-0">{{.user.Email}}</p>
                    </div>
                    <a href="/profile/edit" class="btn btn-outline-primary">
                        {{T "Edit Profile"}}
                    </a>
                </div>

                <hr>

                <dl class="row mb-0">
                    <dt class="col-sm-3">{{T "Member Since"}}</dt>
                    <dd class="col-sm-9">{{.user.CreatedAt.Format "January 2, 2006"}}</dd>
                </dl>
            </div>
        </div>
    </div>
</div>
{{end}}
```

### Settings Form with HTMX

```html
<!-- modules/core/views/templates/core/settings.html -->
{{define "content"}}
<div class="row justify-content-center">
    <div class="col-md-8">
        <h2 class="mb-4">{{T "Settings"}}</h2>

        <div class="card">
            <div class="card-body">
                <!-- Theme Setting with HTMX -->
                <div class="mb-3">
                    <label for="theme" class="form-label">{{T "Theme"}}</label>
                    <select id="theme" name="value" class="form-select"
                            hx-post="/settings"
                            hx-trigger="change"
                            hx-vals='{"key": "theme"}'
                            hx-target="#theme-status"
                            hx-swap="innerHTML">
                        <option value="light" {{if eq .settings.theme "light"}}selected{{end}}>Light</option>
                        <option value="dark" {{if eq .settings.theme "dark"}}selected{{end}}>Dark</option>
                    </select>
                    <div id="theme-status" class="form-text text-success"></div>
                </div>
            </div>
        </div>
    </div>
</div>
{{end}}
```

---

## Controllers (Thin Handlers)

### Main Handler

```go
// modules/core/handlers/main.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type MainHandler struct{}

func NewMainHandler() *MainHandler {
    return &MainHandler{}
}

func (h *MainHandler) Index(c *gin.Context) {
    c.HTML(http.StatusOK, "core/index.html", gin.H{})
}

func (h *MainHandler) Health(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

### Users Handler

```go
// modules/core/handlers/users.go
package handlers

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "yourapp/modules/core/models"
    "yourapp/system/auth"
    "yourapp/system/db"
)

type UsersHandler struct {
    userRepo *models.UserRepository
}

func NewUsersHandler() *UsersHandler {
    return &UsersHandler{
        userRepo: models.NewUserRepository(db.Get()),
    }
}

func (h *UsersHandler) Profile(c *gin.Context) {
    user := auth.GetUser(c)
    c.HTML(http.StatusOK, "core/profile.html", gin.H{
        "user": user,
    })
}

func (h *UsersHandler) EditProfile(c *gin.Context) {
    user := auth.GetUser(c)
    errors := make(map[string]string)

    if c.Request.Method == http.MethodPost {
        firstName := strings.TrimSpace(c.PostForm("first_name"))
        lastName := strings.TrimSpace(c.PostForm("last_name"))

        // Validation
        if firstName == "" {
            errors["first_name"] = "First name is required"
        }

        if len(errors) == 0 {
            _, err := h.userRepo.Update(user.ID, map[string]interface{}{
                "first_name": firstName,
                "last_name":  lastName,
            })

            if err != nil {
                errors["form"] = "Failed to update profile"
            } else {
                auth.AddFlash(c, "success", "Profile updated successfully")
                c.Redirect(http.StatusSeeOther, "/profile")
                return
            }
        }
    }

    c.HTML(http.StatusOK, "core/edit_profile.html", gin.H{
        "user":   user,
        "errors": errors,
    })
}
```

### Settings Handler (HTMX)

```go
// modules/core/handlers/settings.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "yourapp/modules/core/models"
    "yourapp/system/auth"
    "yourapp/system/db"
)

type SettingsHandler struct {
    settingRepo *models.UserSettingRepository
}

func NewSettingsHandler() *SettingsHandler {
    return &SettingsHandler{
        settingRepo: models.NewUserSettingRepository(db.Get()),
    }
}

func (h *SettingsHandler) Index(c *gin.Context) {
    user := auth.GetUser(c)
    settings := h.settingRepo.GetMap(user.ID)

    // Provide defaults
    defaults := map[string]string{
        "theme":    "light",
        "language": "en",
    }
    for key, defaultVal := range defaults {
        if _, exists := settings[key]; !exists {
            settings[key] = defaultVal
        }
    }

    c.HTML(http.StatusOK, "core/settings.html", gin.H{
        "settings": settings,
    })
}

func (h *SettingsHandler) Update(c *gin.Context) {
    user := auth.GetUser(c)
    key := strings.TrimSpace(c.PostForm("key"))
    value := strings.TrimSpace(c.PostForm("value"))

    if key == "" {
        c.String(http.StatusBadRequest, "Key required")
        return
    }

    h.settingRepo.Set(user.ID, key, value)

    // HTMX response
    if c.GetHeader("HX-Request") == "true" {
        c.String(http.StatusOK, "Saved")
        return
    }

    c.Redirect(http.StatusSeeOther, "/settings")
}
```

---

## Request Flow

### Typical Request Lifecycle

1. **Request arrives** → Gin router
2. **Global middleware** runs (logging, recovery)
3. **Auth middleware** runs (session, user loading)
4. **Route matched** → Handler function called
5. **Handler**:
   - Extract user from `auth.GetUser(c)`
   - Parse input (`c.PostForm`, `c.Query`, `c.BindJSON`)
   - Call repository method
   - Handle errors
   - Render template or return JSON
6. **Response sent** to client

### Middleware Chain

```
Request
   │
   ▼
┌──────────────────┐
│  Logger          │
└────────┬─────────┘
         │
   ▼
┌──────────────────┐
│  Recovery        │
└────────┬─────────┘
         │
   ▼
┌──────────────────┐
│  Session         │
└────────┬─────────┘
         │
   ▼
┌──────────────────┐
│  Auth (optional) │
└────────┬─────────┘
         │
   ▼
┌──────────────────┐
│  Handler         │
└────────┬─────────┘
         │
   ▼
Response
```

---

## Best Practices

### Models

✅ **Do:**
- Keep all business logic in models/repositories
- Use repository pattern for database access
- Validate in model methods before save
- Use descriptive method names (`GetByEmail`, not `Find`)
- Return errors, don't panic

❌ **Don't:**
- Don't access Gin context in models
- Don't log in models (return errors instead)
- Don't hardcode values (use constants or config)
- Don't use raw SQL unless necessary (GORM handles most cases)

### Handlers (Controllers)

✅ **Do:**
- Keep handlers thin (just HTTP orchestration)
- Use `auth.GetUser(c)` for authenticated user
- Check `HX-Request` header for HTMX requests
- Return appropriate HTTP status codes
- Use flash messages for redirects

❌ **Don't:**
- Don't put business logic in handlers
- Don't write raw SQL in handlers
- Don't return internal error details to users
- Don't use `c.Abort()` without a response

### Views

✅ **Do:**
- Use partials for reusable components (prefix with `_`)
- Use HTMX attributes for interactivity
- Use Bootstrap classes for styling
- Use `{{T "text"}}` for all user-visible text
- Keep CSS in template or component file (locality of behavior)

❌ **Don't:**
- Don't put complex logic in templates
- Don't use inline styles unless component-specific
- Don't hardcode text strings
- Don't use `{{. | safe}}` unless content is trusted

---

**Next:** [Database Patterns](database.md) | [HTMX Patterns](htmx.md) | [Testing](testing.md)
