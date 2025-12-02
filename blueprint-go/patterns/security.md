# Security Patterns

> CSRF protection, rate limiting, input validation, and security checklist

---

## Table of Contents

- [Security Checklist](#security-checklist)
- [CSRF Protection](#csrf-protection)
- [Rate Limiting](#rate-limiting)
- [Input Validation](#input-validation)
- [SQL Injection Prevention](#sql-injection-prevention)
- [XSS Prevention](#xss-prevention)
- [Session Security](#session-security)
- [Security Headers](#security-headers)
- [Secrets Management](#secrets-management)
- [Password Hashing](#password-hashing)

---

## Security Checklist

### Must-Have (Before Production)

- [ ] **CSRF protection** on all state-changing requests
- [ ] **Rate limiting** on auth endpoints
- [ ] **Input validation** (length, format, type)
- [ ] **Parameterized queries** (GORM handles this)
- [ ] **Auto-escaping templates** (html/template default)
- [ ] **Secure sessions** (HttpOnly, Secure, SameSite)
- [ ] **HTTPS only** in production
- [ ] **Security headers** configured
- [ ] **Password hashing** with bcrypt

### Nice-to-Have

- [ ] Content Security Policy (CSP)
- [ ] Two-factor authentication
- [ ] Audit logging
- [ ] Penetration testing
- [ ] Request signing for APIs

---

## CSRF Protection

### Gin Middleware Implementation

```go
// internal/system/middleware/csrf.go
package middleware

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "net/http"

    "github.com/gin-contrib/sessions"
    "github.com/gin-gonic/gin"
)

const csrfTokenKey = "_csrf_token"

// GenerateCSRFToken creates a new CSRF token
func GenerateCSRFToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}

// CSRFMiddleware validates CSRF tokens on state-changing requests
func CSRFMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        session := sessions.Default(c)

        // Generate token if not exists
        token := session.Get(csrfTokenKey)
        if token == nil {
            token = GenerateCSRFToken()
            session.Set(csrfTokenKey, token)
            session.Save()
        }

        // Make token available to templates
        c.Set("csrf_token", token)

        // Skip validation for safe methods
        if c.Request.Method == "GET" ||
           c.Request.Method == "HEAD" ||
           c.Request.Method == "OPTIONS" {
            c.Next()
            return
        }

        // Skip for API routes (use token auth instead)
        if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:5] == "/api/" {
            c.Next()
            return
        }

        // Get submitted token from form or header
        submitted := c.PostForm("csrf_token")
        if submitted == "" {
            submitted = c.GetHeader("X-CSRF-Token")
        }

        // Validate token
        if !validateToken(token.(string), submitted) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "CSRF token invalid",
            })
            return
        }

        c.Next()
    }
}

func validateToken(expected, submitted string) bool {
    if submitted == "" {
        return false
    }
    return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}
```

### Register Middleware

```go
// main.go or router setup
func setupRouter() *gin.Engine {
    r := gin.Default()

    // Session store
    store := cookie.NewStore([]byte(os.Getenv("SECRET_KEY")))
    r.Use(sessions.Sessions("session", store))

    // CSRF protection
    r.Use(middleware.CSRFMiddleware())

    return r
}
```

### Usage in Templates

```html
<form method="POST" action="/profile/edit">
    <input type="hidden" name="csrf_token" value="{{.csrf_token}}">
    <!-- form fields -->
    <button type="submit">Save</button>
</form>
```

### HTMX Requests

```html
<!-- Add token to all HTMX requests -->
<body hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>
```

### Template Helper

```go
// Make csrf_token available to all templates
func templateFuncs(c *gin.Context) template.FuncMap {
    return template.FuncMap{
        "csrfToken": func() string {
            if token, exists := c.Get("csrf_token"); exists {
                return token.(string)
            }
            return ""
        },
    }
}
```

---

## Rate Limiting

### Simple In-Memory Rate Limiter

```go
// internal/system/middleware/ratelimit.go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

type rateLimitEntry struct {
    count       int
    windowStart time.Time
}

type RateLimiter struct {
    limits map[string]*rateLimitEntry
    mu     sync.RWMutex
}

func NewRateLimiter() *RateLimiter {
    rl := &RateLimiter{
        limits: make(map[string]*rateLimitEntry),
    }
    // Cleanup old entries periodically
    go rl.cleanup()
    return rl
}

func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(time.Minute)
    for range ticker.C {
        rl.mu.Lock()
        now := time.Now()
        for key, entry := range rl.limits {
            if now.Sub(entry.windowStart) > time.Hour {
                delete(rl.limits, key)
            }
        }
        rl.mu.Unlock()
    }
}

// RateLimit returns middleware that limits requests
func (rl *RateLimiter) RateLimit(limit int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := c.HandlerName() + ":" + getClientIP(c)

        if !rl.checkLimit(key, limit, window) {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded",
            })
            return
        }

        c.Next()
    }
}

func (rl *RateLimiter) checkLimit(key string, limit int, window time.Duration) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    entry, exists := rl.limits[key]

    if !exists {
        rl.limits[key] = &rateLimitEntry{count: 1, windowStart: now}
        return true
    }

    // Window expired, reset
    if now.Sub(entry.windowStart) > window {
        rl.limits[key] = &rateLimitEntry{count: 1, windowStart: now}
        return true
    }

    // Check limit
    if entry.count >= limit {
        return false
    }

    entry.count++
    return true
}

func getClientIP(c *gin.Context) string {
    // Check X-Forwarded-For for proxied requests
    forwarded := c.GetHeader("X-Forwarded-For")
    if forwarded != "" {
        return forwarded
    }
    return c.ClientIP()
}
```

### Usage

```go
// Initialize rate limiter
var limiter = middleware.NewRateLimiter()

// Apply to specific routes
r.POST("/login", limiter.RateLimit(10, time.Minute), handlers.Login)
r.POST("/register", limiter.RateLimit(5, time.Minute), handlers.Register)

// API routes
api := r.Group("/api")
api.Use(limiter.RateLimit(100, time.Minute))
```

### Production: Use Redis

```go
// go get github.com/ulule/limiter/v3
// go get github.com/ulule/limiter/v3/drivers/store/redis

import (
    "github.com/ulule/limiter/v3"
    mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
    sredis "github.com/ulule/limiter/v3/drivers/store/redis"
    "github.com/redis/go-redis/v9"
)

func setupRateLimiter() gin.HandlerFunc {
    // Redis client
    client := redis.NewClient(&redis.Options{
        Addr: os.Getenv("REDIS_URL"),
    })

    // Create store
    store, _ := sredis.NewStore(client)

    // Define rate: 100 requests per minute
    rate := limiter.Rate{
        Period: time.Minute,
        Limit:  100,
    }

    instance := limiter.New(store, rate)
    return mgin.NewMiddleware(instance)
}

// Usage
r.Use(setupRateLimiter())
```

### Per-User Rate Limiting

```go
// Rate limit by user ID instead of IP
func (rl *RateLimiter) UserRateLimit(limit int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := auth.GetUser(c)
        if user == nil {
            c.Next()
            return
        }

        key := c.HandlerName() + ":user:" + fmt.Sprint(user.ID)

        if !rl.checkLimit(key, limit, window) {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded",
            })
            return
        }

        c.Next()
    }
}
```

---

## Input Validation

### Repository-Level Validation

```go
// internal/modules/users/models/user.go
package models

import (
    "errors"
    "regexp"
    "strings"
)

var emailRegex = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)

type UserRepository struct {
    db *gorm.DB
}

func (r *UserRepository) Create(email, name, password string) (*User, error) {
    // Sanitize
    email = strings.TrimSpace(strings.ToLower(email))
    name = strings.TrimSpace(name)

    // Validate required fields
    if email == "" {
        return nil, errors.New("email is required")
    }
    if name == "" {
        return nil, errors.New("name is required")
    }
    if password == "" {
        return nil, errors.New("password is required")
    }

    // Validate format
    if !emailRegex.MatchString(email) {
        return nil, errors.New("invalid email format")
    }

    // Validate length
    if len(email) > 255 {
        return nil, errors.New("email too long")
    }
    if len(name) > 100 {
        return nil, errors.New("name too long")
    }
    if len(password) < 8 {
        return nil, errors.New("password must be at least 8 characters")
    }

    // Check uniqueness
    var existing User
    if err := r.db.Where("email = ?", email).First(&existing).Error; err == nil {
        return nil, errors.New("email already registered")
    }

    // Hash password and create
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return nil, err
    }

    user := &User{
        Email:        email,
        Name:         name,
        PasswordHash: string(hashedPassword),
    }

    if err := r.db.Create(user).Error; err != nil {
        return nil, err
    }

    return user, nil
}
```

### Handler-Level Validation

```go
// internal/modules/users/handlers/routes.go
func (h *UsersHandler) EditProfile(c *gin.Context) {
    user := auth.GetUser(c)

    name := strings.TrimSpace(c.PostForm("name"))
    avatarURL := strings.TrimSpace(c.PostForm("avatar_url"))

    // Limit length at handler level
    if len(name) > 100 {
        name = name[:100]
    }
    if len(avatarURL) > 500 {
        avatarURL = avatarURL[:500]
    }

    // URL validation
    if avatarURL != "" && !strings.HasPrefix(avatarURL, "http://") &&
       !strings.HasPrefix(avatarURL, "https://") {
        c.HTML(http.StatusBadRequest, "users/edit.html", gin.H{
            "error": "Invalid avatar URL",
            "user":  user,
        })
        return
    }

    // Update via repository
    if err := h.userRepo.UpdateProfile(user.ID, name, avatarURL); err != nil {
        c.HTML(http.StatusBadRequest, "users/edit.html", gin.H{
            "error": err.Error(),
            "user":  user,
        })
        return
    }

    c.Redirect(http.StatusSeeOther, "/profile")
}
```

### Validation Helper Package

```go
// internal/system/validation/validation.go
package validation

import (
    "errors"
    "regexp"
    "strings"
    "unicode/utf8"
)

var (
    EmailRegex = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)
    URLRegex   = regexp.MustCompile(`^https?://`)
)

func Required(value, field string) error {
    if strings.TrimSpace(value) == "" {
        return errors.New(field + " is required")
    }
    return nil
}

func MaxLength(value string, max int, field string) error {
    if utf8.RuneCountInString(value) > max {
        return errors.New(field + " is too long")
    }
    return nil
}

func MinLength(value string, min int, field string) error {
    if utf8.RuneCountInString(value) < min {
        return errors.New(field + " is too short")
    }
    return nil
}

func Email(value string) error {
    if !EmailRegex.MatchString(value) {
        return errors.New("invalid email format")
    }
    return nil
}

func URL(value string) error {
    if value != "" && !URLRegex.MatchString(value) {
        return errors.New("invalid URL format")
    }
    return nil
}

// ValidateAll runs multiple validators and returns first error
func ValidateAll(validators ...func() error) error {
    for _, v := range validators {
        if err := v(); err != nil {
            return err
        }
    }
    return nil
}
```

### Usage

```go
import "yourapp/internal/system/validation"

func (h *Handler) Create(c *gin.Context) {
    name := c.PostForm("name")
    email := c.PostForm("email")

    err := validation.ValidateAll(
        func() error { return validation.Required(name, "name") },
        func() error { return validation.MaxLength(name, 100, "name") },
        func() error { return validation.Required(email, "email") },
        func() error { return validation.Email(email) },
    )

    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Proceed with validated data
}
```

---

## SQL Injection Prevention

### GORM Handles This

```go
// SAFE: GORM parameterizes automatically
user, _ := userRepo.GetByEmail(email)  // Uses db.Where("email = ?", email)
users, _ := userRepo.Search(search)    // Uses db.Where("name LIKE ?", "%"+search+"%")

// SAFE: Using Raw with parameters
var results []Stats
db.Raw("SELECT category, COUNT(*) as count FROM items WHERE user_id = ? GROUP BY category", userID).Scan(&results)

// NEVER DO THIS: String interpolation
db.Raw(fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email))  // VULNERABLE!
```

### Query Builder Pattern

```go
// SAFE: Build queries programmatically
func (r *ItemRepository) Search(opts SearchOptions) ([]Item, error) {
    query := r.db.Model(&Item{})

    if opts.Name != "" {
        query = query.Where("name LIKE ?", "%"+opts.Name+"%")
    }
    if opts.CategoryID > 0 {
        query = query.Where("category_id = ?", opts.CategoryID)
    }
    if opts.IsActive != nil {
        query = query.Where("is_active = ?", *opts.IsActive)
    }

    var items []Item
    err := query.Find(&items).Error
    return items, err
}
```

---

## XSS Prevention

### html/template Auto-Escaping

```html
<!-- SAFE: Auto-escaped (default) -->
<p>{{.user.Name}}</p>
<p>{{.userInput}}</p>

<!-- DANGEROUS: Only use with trusted content -->
<p>{{.trustedHTML | safeHTML}}</p>
```

### Define safeHTML Function

```go
// Only use when you trust the content completely
func templateFuncs() template.FuncMap {
    return template.FuncMap{
        "safeHTML": func(s string) template.HTML {
            return template.HTML(s)
        },
    }
}
```

### Sanitize User HTML

```go
// go get github.com/microcosm-cc/bluemonday

import "github.com/microcosm-cc/bluemonday"

// Create policy once
var htmlPolicy = bluemonday.UGCPolicy()

func sanitizeHTML(input string) string {
    return htmlPolicy.Sanitize(input)
}

// Usage
func (h *Handler) SaveComment(c *gin.Context) {
    content := c.PostForm("content")

    // Sanitize before storing
    sanitized := sanitizeHTML(content)

    comment := &Comment{
        Content: sanitized,
        UserID:  auth.GetUser(c).ID,
    }
    h.commentRepo.Create(comment)
}
```

### Content Security Policy

```go
// CSP middleware
func SecurityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent inline scripts (mitigates XSS)
        c.Header("Content-Security-Policy",
            "default-src 'self'; "+
            "script-src 'self' https://unpkg.com; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "font-src 'self' https://fonts.gstatic.com")

        c.Next()
    }
}
```

---

## Session Security

### Configuration

```go
// internal/system/config/config.go
package config

import (
    "os"
    "time"
)

type Config struct {
    SecretKey     string
    SessionMaxAge int
    SecureCookie  bool
    Environment   string
}

func Load() *Config {
    env := os.Getenv("GO_ENV")
    if env == "" {
        env = "development"
    }

    return &Config{
        SecretKey:     os.Getenv("SECRET_KEY"),
        SessionMaxAge: 86400, // 24 hours
        SecureCookie:  env == "production",
        Environment:   env,
    }
}
```

### Session Store Setup

```go
import (
    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
)

func setupSession(r *gin.Engine, cfg *config.Config) {
    store := cookie.NewStore([]byte(cfg.SecretKey))

    store.Options(sessions.Options{
        Path:     "/",
        MaxAge:   cfg.SessionMaxAge,
        HttpOnly: true,                    // No JavaScript access
        Secure:   cfg.SecureCookie,        // HTTPS only in production
        SameSite: http.SameSiteLaxMode,    // CSRF protection
    })

    r.Use(sessions.Sessions("session", store))
}
```

### Session Regeneration

```go
// After login, regenerate session to prevent fixation
func (h *AuthHandler) Login(c *gin.Context) {
    // ... validate credentials ...

    session := sessions.Default(c)

    // Clear old session
    session.Clear()

    // Set new session data
    session.Set("user_id", user.ID)
    session.Save()

    c.Redirect(http.StatusSeeOther, "/dashboard")
}
```

### Redis Session Store (Production)

```go
// go get github.com/gin-contrib/sessions/redis

import (
    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/redis"
)

func setupRedisSession(r *gin.Engine, cfg *config.Config) {
    store, _ := redis.NewStore(10, "tcp", cfg.RedisURL, "", []byte(cfg.SecretKey))

    store.Options(sessions.Options{
        Path:     "/",
        MaxAge:   cfg.SessionMaxAge,
        HttpOnly: true,
        Secure:   cfg.SecureCookie,
        SameSite: http.SameSiteLaxMode,
    })

    r.Use(sessions.Sessions("session", store))
}
```

---

## Security Headers

```go
// internal/system/middleware/security.go
package middleware

import (
    "github.com/gin-gonic/gin"
)

func SecurityHeadersMiddleware(production bool) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent clickjacking
        c.Header("X-Frame-Options", "SAMEORIGIN")

        // Prevent MIME sniffing
        c.Header("X-Content-Type-Options", "nosniff")

        // XSS filter (legacy browsers)
        c.Header("X-XSS-Protection", "1; mode=block")

        // Referrer policy
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

        // Permissions policy
        c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

        // HSTS (only in production with HTTPS)
        if production {
            c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }

        c.Next()
    }
}
```

### Usage

```go
func setupRouter(cfg *config.Config) *gin.Engine {
    r := gin.Default()

    isProduction := cfg.Environment == "production"
    r.Use(middleware.SecurityHeadersMiddleware(isProduction))

    return r
}
```

---

## Secrets Management

### Development

```bash
# .env (never commit)
SECRET_KEY=dev-secret-key-change-in-production
DATABASE_URL=sqlite:///app.db
REDIS_URL=localhost:6379
```

### Loading Environment

```go
// go get github.com/joho/godotenv

import "github.com/joho/godotenv"

func main() {
    // Load .env in development
    if os.Getenv("GO_ENV") != "production" {
        godotenv.Load()
    }

    secretKey := os.Getenv("SECRET_KEY")
    if secretKey == "" {
        log.Fatal("SECRET_KEY environment variable is required")
    }

    // Continue with app setup
}
```

### Production

**Never commit secrets to git.**

```bash
# Environment variables (set in deployment platform)
export SECRET_KEY="$(openssl rand -hex 32)"
export DATABASE_URL="postgresql://user:pass@localhost/dbname"
export REDIS_URL="redis://localhost:6379"
```

### Generating Secrets

```go
import (
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
)

// Generate secure secret key
func GenerateSecretKey() string {
    b := make([]byte, 32)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// Generate API key
func GenerateAPIKey() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}
```

### .gitignore

```
# Secrets
.env
*.env
!.env.example

# Keys
*.pem
*.key
credentials.json
service-account.json
```

---

## Password Hashing

### Using bcrypt

```go
import "golang.org/x/crypto/bcrypt"

// Hash password
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

// Verify password
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

### In User Repository

```go
func (r *UserRepository) Create(email, password string) (*User, error) {
    hashedPassword, err := HashPassword(password)
    if err != nil {
        return nil, err
    }

    user := &User{
        Email:        email,
        PasswordHash: hashedPassword,
    }

    if err := r.db.Create(user).Error; err != nil {
        return nil, err
    }

    return user, nil
}

func (r *UserRepository) Authenticate(email, password string) (*User, error) {
    var user User
    if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
        return nil, errors.New("invalid credentials")
    }

    if !CheckPassword(password, user.PasswordHash) {
        return nil, errors.New("invalid credentials")
    }

    return &user, nil
}
```

### Password Requirements

```go
func ValidatePassword(password string) error {
    if len(password) < 8 {
        return errors.New("password must be at least 8 characters")
    }
    if len(password) > 72 {
        // bcrypt has a 72-byte limit
        return errors.New("password too long")
    }

    // Optional: Add complexity requirements
    hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
    hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
    hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)

    if !hasUpper || !hasLower || !hasNumber {
        return errors.New("password must contain uppercase, lowercase, and number")
    }

    return nil
}
```

---

**Next:** [Testing](testing.md) | [Deployment](deployment.md)
