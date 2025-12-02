# Testing Patterns

> Complete guide to testing Blueprint Go applications.

---

## Overview

Blueprint uses Go's built-in testing framework:

- **testing** package for unit and integration tests
- **httptest** for HTTP handler testing
- **testify** for assertions (optional)
- **SQLite in-memory** for database tests

---

## Test Structure

```
blueprint-go/
├── internal/
│   ├── modules/
│   │   └── core/
│   │       ├── models/
│   │       │   ├── user.go
│   │       │   └── user_test.go     # Model tests
│   │       └── handlers/
│   │           ├── auth.go
│   │           └── auth_test.go     # Handler tests
│   └── system/
│       └── db/
│           ├── database.go
│           └── database_test.go     # DB tests
└── tests/
    └── integration/
        └── auth_test.go             # Integration tests
```

---

## Running Tests

```bash
# Run all tests
make test

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/modules/core/models/...

# Run specific test
go test -v -run TestUserCreate ./internal/modules/core/models/

# Run with coverage
make test-coverage

# View coverage report
go tool cover -html=coverage.out
```

---

## Unit Tests

### Model Tests

```go
// internal/modules/core/models/user_test.go
package models

import (
    "testing"

    "blueprint-go/internal/system/db"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
    testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to open test database: %v", err)
    }

    // Migrate test tables
    testDB.AutoMigrate(&User{}, &Group{})

    return testDB
}

func TestUserCreate(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewUserRepository(testDB)

    user, err := repo.Create("test@example.com", "password123", "John", "Doe", false)

    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if user.Email != "test@example.com" {
        t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
    }
    if user.FirstName != "John" {
        t.Errorf("Expected first name 'John', got '%s'", user.FirstName)
    }
}

func TestUserSetPassword(t *testing.T) {
    user := &User{}

    err := user.SetPassword("mypassword")

    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if user.PasswordHash == "" {
        t.Error("Expected password hash to be set")
    }
    if user.PasswordHash == "mypassword" {
        t.Error("Password should be hashed, not stored in plain text")
    }
}

func TestUserCheckPassword(t *testing.T) {
    user := &User{}
    user.SetPassword("correctpassword")

    tests := []struct {
        name     string
        password string
        expected bool
    }{
        {"correct password", "correctpassword", true},
        {"wrong password", "wrongpassword", false},
        {"empty password", "", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := user.CheckPassword(tt.password)
            if result != tt.expected {
                t.Errorf("CheckPassword(%q) = %v, expected %v",
                    tt.password, result, tt.expected)
            }
        })
    }
}

func TestUserIsAdmin(t *testing.T) {
    tests := []struct {
        name     string
        isAdmin  bool
        expected bool
    }{
        {"admin user", true, true},
        {"regular user", false, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            user := &User{IsAdmin: tt.isAdmin}
            if user.IsAdmin != tt.expected {
                t.Errorf("IsAdmin = %v, expected %v", user.IsAdmin, tt.expected)
            }
        })
    }
}
```

### Repository Tests

```go
func TestUserRepository_GetByEmail(t *testing.T) {
    testDB := setupTestDB(t)
    repo := NewUserRepository(testDB)

    // Create test user
    repo.Create("find@example.com", "password", "Test", "User", false)

    // Test finding user
    user, err := repo.GetByEmail("find@example.com")
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if user == nil {
        t.Fatal("Expected user, got nil")
    }
    if user.Email != "find@example.com" {
        t.Errorf("Expected email 'find@example.com', got '%s'", user.Email)
    }

    // Test not found
    user, err = repo.GetByEmail("nonexistent@example.com")
    if err == nil && user != nil {
        t.Error("Expected nil user for nonexistent email")
    }
}
```

---

## Handler Tests

```go
// internal/modules/core/handlers/auth_test.go
package handlers

import (
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
    gin.SetMode(gin.TestMode)

    testDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    testDB.AutoMigrate(&models.User{}, &models.Group{})

    r := gin.New()

    // Setup session for tests
    store := cookie.NewStore([]byte("test-secret"))
    r.Use(sessions.Sessions("session", store))

    return r, testDB
}

func TestLoginPage(t *testing.T) {
    r, testDB := setupTestRouter(t)
    handler := NewAuthHandler(testDB)

    r.GET("/login", handler.LoginPage)

    req := httptest.NewRequest("GET", "/login", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
}

func TestLoginSuccess(t *testing.T) {
    r, testDB := setupTestRouter(t)
    handler := NewAuthHandler(testDB)

    // Create test user
    repo := models.NewUserRepository(testDB)
    repo.Create("test@example.com", "password123", "Test", "User", false)

    r.POST("/login", handler.Login)

    form := url.Values{}
    form.Add("email", "test@example.com")
    form.Add("password", "password123")

    req := httptest.NewRequest("POST", "/login",
        strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    if w.Code != http.StatusFound {
        t.Errorf("Expected redirect (302), got %d", w.Code)
    }

    location := w.Header().Get("Location")
    if location != "/dashboard" {
        t.Errorf("Expected redirect to /dashboard, got %s", location)
    }
}

func TestLoginFailure(t *testing.T) {
    r, testDB := setupTestRouter(t)
    handler := NewAuthHandler(testDB)

    r.POST("/login", handler.Login)

    form := url.Values{}
    form.Add("email", "wrong@example.com")
    form.Add("password", "wrongpassword")

    req := httptest.NewRequest("POST", "/login",
        strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200 (re-render login), got %d", w.Code)
    }

    body := w.Body.String()
    if !strings.Contains(body, "Invalid") {
        t.Error("Expected error message in response")
    }
}
```

---

## Integration Tests

```go
// tests/integration/auth_test.go
package integration

import (
    "net/http"
    "net/http/cookiejar"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"

    "blueprint-go/internal/app"
)

func setupIntegrationTest(t *testing.T) *httptest.Server {
    // Create test app with in-memory database
    testApp := app.New(app.Config{
        DatabaseURL: ":memory:",
        SecretKey:   "test-secret-key",
        Debug:       true,
    })

    return httptest.NewServer(testApp.Router())
}

func TestFullLoginFlow(t *testing.T) {
    server := setupIntegrationTest(t)
    defer server.Close()

    // Create HTTP client with cookie jar
    jar, _ := cookiejar.New(nil)
    client := &http.Client{Jar: jar}

    // 1. Get login page
    resp, err := client.Get(server.URL + "/login")
    if err != nil {
        t.Fatalf("Failed to get login page: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }

    // 2. Login with valid credentials
    form := url.Values{
        "email":    {"admin@example.com"},
        "password": {"admin"},
    }
    resp, err = client.PostForm(server.URL+"/login", form)
    if err != nil {
        t.Fatalf("Failed to login: %v", err)
    }

    // Should redirect to dashboard
    if resp.Request.URL.Path != "/dashboard" {
        t.Errorf("Expected redirect to /dashboard, got %s", resp.Request.URL.Path)
    }

    // 3. Access protected route
    resp, err = client.Get(server.URL + "/dashboard")
    if err != nil {
        t.Fatalf("Failed to get dashboard: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200 for dashboard, got %d", resp.StatusCode)
    }

    // 4. Logout
    resp, err = client.Post(server.URL+"/logout", "", nil)
    if err != nil {
        t.Fatalf("Failed to logout: %v", err)
    }

    // 5. Verify cannot access protected route
    resp, err = client.Get(server.URL + "/dashboard")
    if err != nil {
        t.Fatalf("Failed to get dashboard after logout: %v", err)
    }
    // Should redirect to login
    if resp.Request.URL.Path != "/login" {
        t.Errorf("Expected redirect to /login, got %s", resp.Request.URL.Path)
    }
}
```

---

## Table-Driven Tests

```go
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name      string
        email     string
        password  string
        firstName string
        lastName  string
        wantErr   bool
    }{
        {
            name:      "valid user",
            email:     "valid@example.com",
            password:  "password123",
            firstName: "John",
            lastName:  "Doe",
            wantErr:   false,
        },
        {
            name:      "empty email",
            email:     "",
            password:  "password123",
            firstName: "John",
            lastName:  "Doe",
            wantErr:   true,
        },
        {
            name:      "short password",
            email:     "valid@example.com",
            password:  "short",
            firstName: "John",
            lastName:  "Doe",
            wantErr:   true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testDB := setupTestDB(t)
            repo := NewUserRepository(testDB)

            _, err := repo.Create(tt.email, tt.password, tt.firstName, tt.lastName, false)

            if (err != nil) != tt.wantErr {
                t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## Mocking

### Interface-Based Mocking

```go
// Define interface
type UserRepositoryInterface interface {
    GetByID(id uint) (*User, error)
    GetByEmail(email string) (*User, error)
    Create(email, password, firstName, lastName string, isAdmin bool) (*User, error)
}

// Mock implementation
type MockUserRepository struct {
    Users map[string]*User
}

func (m *MockUserRepository) GetByEmail(email string) (*User, error) {
    if user, ok := m.Users[email]; ok {
        return user, nil
    }
    return nil, nil
}

func (m *MockUserRepository) Create(email, password, firstName, lastName string, isAdmin bool) (*User, error) {
    user := &User{
        Email:     email,
        FirstName: firstName,
        LastName:  lastName,
        IsAdmin:   isAdmin,
    }
    user.SetPassword(password)
    m.Users[email] = user
    return user, nil
}

// Use in tests
func TestAuthHandlerWithMock(t *testing.T) {
    mockRepo := &MockUserRepository{
        Users: make(map[string]*User),
    }

    handler := &AuthHandler{userRepo: mockRepo}
    // ... test handler
}
```

---

## Test Helpers

```go
// tests/helpers.go
package tests

import (
    "testing"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

// SetupTestDB creates an in-memory SQLite database for testing
func SetupTestDB(t *testing.T, models ...interface{}) *gorm.DB {
    t.Helper()

    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }

    if len(models) > 0 {
        db.AutoMigrate(models...)
    }

    return db
}

// CreateTestUser creates a user for testing
func CreateTestUser(t *testing.T, db *gorm.DB, email string, isAdmin bool) *models.User {
    t.Helper()

    repo := models.NewUserRepository(db)
    user, err := repo.Create(email, "testpassword", "Test", "User", isAdmin)
    if err != nil {
        t.Fatalf("Failed to create test user: %v", err)
    }
    return user
}
```

---

## Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# View in browser
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Coverage Targets

| Package | Target |
|---------|--------|
| Models | 80%+ |
| Handlers | 70%+ |
| System | 60%+ |

---

## Best Practices

1. **Test file naming** - Use `*_test.go` suffix
2. **Test function naming** - `TestFunctionName_Scenario`
3. **Table-driven tests** - For multiple input variations
4. **In-memory database** - Fast, isolated tests
5. **Setup/teardown** - Use helper functions
6. **Parallel tests** - Use `t.Parallel()` when safe
7. **Assertions** - Be specific about expected values
8. **Coverage** - Aim for 70%+ on business logic

---

## Makefile Targets

```makefile
# Run all tests
test:
	go test -v ./...

# Run with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run specific package
test-models:
	go test -v ./internal/modules/core/models/...

# Run with race detector
test-race:
	go test -race ./...
```
