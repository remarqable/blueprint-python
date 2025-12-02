# Authentication Patterns

> Complete guide to authentication and authorization in Blueprint Go.

---

## Overview

Blueprint uses **session-based authentication** with:

- Cookie sessions via `gin-contrib/sessions`
- Password hashing via `bcrypt`
- Group-based authorization
- Middleware for route protection

---

## Session Setup

```go
// internal/system/auth/session.go

import (
    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
    "github.com/gin-gonic/gin"
)

// Setup initializes the session middleware
func Setup(r *gin.Engine, secretKey string) {
    store := cookie.NewStore([]byte(secretKey))
    store.Options(sessions.Options{
        Path:     "/",
        MaxAge:   86400 * 7, // 7 days
        HttpOnly: true,
        Secure:   false, // Set true in production with HTTPS
        SameSite: http.SameSiteLaxMode,
    })
    r.Use(sessions.Sessions("session", store))
}
```

---

## Login/Logout

### Login Handler

```go
// internal/modules/core/handlers/auth.go

func (h *AuthHandler) Login(c *gin.Context) {
    email := c.PostForm("email")
    password := c.PostForm("password")

    // Find user
    user, err := h.userRepo.GetByEmail(email)
    if err != nil || user == nil {
        c.HTML(http.StatusOK, "core/login.html", gin.H{
            "error": "Invalid credentials",
        })
        return
    }

    // Check password
    if !user.CheckPassword(password) {
        c.HTML(http.StatusOK, "core/login.html", gin.H{
            "error": "Invalid credentials",
        })
        return
    }

    // Create session
    session := sessions.Default(c)
    session.Set("user_id", user.ID)
    session.Save()

    c.Redirect(http.StatusFound, "/dashboard")
}
```

### Logout Handler

```go
func (h *AuthHandler) Logout(c *gin.Context) {
    session := sessions.Default(c)
    session.Clear()
    session.Save()
    c.Redirect(http.StatusFound, "/login")
}
```

---

## Password Hashing

```go
// internal/modules/core/models/user.go

import "golang.org/x/crypto/bcrypt"

// SetPassword hashes and stores the password
func (u *User) SetPassword(password string) error {
    hash, err := bcrypt.GenerateFromPassword(
        []byte(password),
        bcrypt.DefaultCost,
    )
    if err != nil {
        return err
    }
    u.PasswordHash = string(hash)
    return nil
}

// CheckPassword verifies a password against the hash
func (u *User) CheckPassword(password string) bool {
    err := bcrypt.CompareHashAndPassword(
        []byte(u.PasswordHash),
        []byte(password),
    )
    return err == nil
}
```

---

## Middleware

### RequireLogin

Protects routes that require authentication:

```go
// internal/system/auth/middleware.go

func RequireLogin() gin.HandlerFunc {
    return func(c *gin.Context) {
        session := sessions.Default(c)
        userID := session.Get("user_id")

        if userID == nil {
            c.Redirect(http.StatusFound, "/login")
            c.Abort()
            return
        }

        // Load user and set in context
        user, err := loadUser(userID.(uint))
        if err != nil || user == nil {
            session.Clear()
            session.Save()
            c.Redirect(http.StatusFound, "/login")
            c.Abort()
            return
        }

        c.Set("user", user)
        c.Next()
    }
}
```

### RequireAdmin

Protects admin-only routes:

```go
func RequireAdmin() gin.HandlerFunc {
    return func(c *gin.Context) {
        user := GetUser(c)
        if user == nil || !user.IsAdmin {
            c.HTML(http.StatusForbidden, "error.html", gin.H{
                "error": "Admin access required",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### RequireGroup

Protects routes for specific groups:

```go
func RequireGroup(groupName string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := GetUser(c)
        if user == nil {
            c.Redirect(http.StatusFound, "/login")
            c.Abort()
            return
        }

        // Check if user belongs to group
        hasGroup := false
        for _, group := range user.Groups {
            if group.Name == groupName {
                hasGroup = true
                break
            }
        }

        if !hasGroup {
            c.HTML(http.StatusForbidden, "error.html", gin.H{
                "error": "Access denied",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### OptionalAuth

Loads user if logged in, but doesn't require it:

```go
func OptionalAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        session := sessions.Default(c)
        userID := session.Get("user_id")

        if userID != nil {
            user, _ := loadUser(userID.(uint))
            if user != nil {
                c.Set("user", user)
            }
        }
        c.Next()
    }
}
```

---

## Context Helpers

```go
// internal/system/auth/context.go

const UserContextKey = "user"

// GetUser retrieves the current user from context
func GetUser(c *gin.Context) *models.User {
    if user, exists := c.Get(UserContextKey); exists {
        if u, ok := user.(*models.User); ok {
            return u
        }
    }
    return nil
}

// IsAuthenticated checks if a user is logged in
func IsAuthenticated(c *gin.Context) bool {
    return GetUser(c) != nil
}

// IsAdmin checks if current user is admin
func IsAdmin(c *gin.Context) bool {
    user := GetUser(c)
    return user != nil && user.IsAdmin
}
```

---

## Usage in Routes

### Protected Route

```go
func (m *CoreModule) RegisterRoutes(r *gin.RouterGroup) {
    // Public routes
    r.GET("/login", h.LoginPage)
    r.POST("/login", h.Login)

    // Protected routes
    protected := r.Group("")
    protected.Use(auth.RequireLogin())
    {
        protected.GET("/dashboard", h.Dashboard)
        protected.GET("/settings", h.Settings)
        protected.POST("/logout", h.Logout)
    }

    // Admin routes
    admin := r.Group("/admin")
    admin.Use(auth.RequireLogin(), auth.RequireAdmin())
    {
        admin.GET("/users", h.AdminUsers)
        admin.POST("/users/:id/delete", h.DeleteUser)
    }
}
```

### In Handler

```go
func (h *DashboardHandler) Index(c *gin.Context) {
    user := auth.GetUser(c)

    c.HTML(http.StatusOK, "core/dashboard.html", gin.H{
        "user":    user,
        "isAdmin": user.IsAdmin,
    })
}
```

### In Templates

```html
{% if .user %}
    <span>Welcome, {{ .user.FirstName }}</span>
    {% if .user.IsAdmin %}
        <a href="/admin">Admin Panel</a>
    {% endif %}
{% endif %}
```

---

## Group-Based Access Control

### Built-in Groups

```go
// System groups created on initialization
const (
    GroupAdmin = "ADMIN"
    GroupUser  = "USER"
    GroupAll   = "ALL"
)
```

### User-Group Association

```go
// Add user to group
func (r *UserRepository) AddToGroup(userID uint, groupName string) error {
    user, err := r.GetByID(userID)
    if err != nil {
        return err
    }

    group, err := r.groupRepo.GetByName(groupName)
    if err != nil {
        return err
    }

    return r.db.Model(user).Association("Groups").Append(group)
}

// Check group membership
func (u *User) HasGroup(groupName string) bool {
    for _, g := range u.Groups {
        if g.Name == groupName {
            return true
        }
    }
    return false
}
```

---

## Security Best Practices

1. **Use HTTPS in production** - Set `Secure: true` on cookies
2. **Strong secret keys** - Use cryptographically random keys
3. **Session expiration** - Set appropriate `MaxAge`
4. **Password requirements** - Enforce minimum length/complexity
5. **Rate limiting** - Prevent brute force attacks
6. **CSRF protection** - Use tokens for state-changing requests
7. **Never log passwords** - Only log authentication events

---

## Session Configuration

### Development

```go
store.Options(sessions.Options{
    Path:     "/",
    MaxAge:   86400 * 7,
    HttpOnly: true,
    Secure:   false,  // HTTP allowed
    SameSite: http.SameSiteLaxMode,
})
```

### Production

```go
store.Options(sessions.Options{
    Path:     "/",
    MaxAge:   86400 * 7,
    HttpOnly: true,
    Secure:   true,   // HTTPS only
    SameSite: http.SameSiteStrictMode,
})
```

---

## Error Handling

Never reveal whether email exists:

```go
// Bad - reveals email existence
if user == nil {
    return "Email not found"
}
if !user.CheckPassword(password) {
    return "Wrong password"
}

// Good - generic message
if user == nil || !user.CheckPassword(password) {
    return "Invalid credentials"
}
```
