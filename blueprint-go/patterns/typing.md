# Type Safety in Go

> **Complete guide to type-safe Go code patterns for Blueprint modules.**
> Go's static type system catches errors at compile time.

---

## Overview

Go is a statically typed language - the compiler enforces types at build time. This guide covers:

- Interface patterns for modules
- Struct types and composition
- Generics (Go 1.18+)
- JSON and form binding
- Error types and handling
- Common type patterns

---

## Table of Contents

- [Static vs Dynamic Typing](#static-vs-dynamic-typing)
- [Interface Patterns](#interface-patterns)
- [Struct Patterns](#struct-patterns)
- [Generics](#generics)
- [JSON Struct Tags](#json-struct-tags)
- [Error Types](#error-types)
- [Common Patterns](#common-patterns)
- [Type Assertions](#type-assertions)

---

## Static vs Dynamic Typing

### Go Advantages

Unlike Python (which uses mypy for optional type checking), Go enforces types at compile time:

```go
// Compiler catches this immediately
var count int = "hello"  // Error: cannot use "hello" (type string) as type int

// Python equivalent would only fail at runtime (or with mypy)
```

### No Type Annotations Needed

Go infers types from assignments:

```go
// Explicit type
var name string = "John"

// Inferred type (preferred)
name := "John"        // string
count := 42           // int
active := true        // bool
items := []string{}   // []string
```

---

## Interface Patterns

### Module Interface

```go
// internal/system/module/interfaces.go
package module

import "github.com/gin-gonic/gin"

// Module defines the interface all modules must implement
type Module interface {
    // GetRoutes returns routes to register
    GetRoutes() []Route
}

// Route defines a route registration
type Route struct {
    Blueprint *gin.RouterGroup
    Prefix    string
}

// DatabaseInitializer is optional interface for DB setup
type DatabaseInitializer interface {
    InitDatabase() error
}

// Manifest defines module metadata
type Manifest struct {
    Name            string   `json:"name"`
    Version         string   `json:"version"`
    MainRoute       string   `json:"main_route"`
    Type            string   `json:"type"` // "App" or "System"
    Depends         []string `json:"depends"`
    IconClass       string   `json:"icon_class"`
    Color           string   `json:"color"`
    Description     string   `json:"description"`
    LongDescription string   `json:"long_description"`
}
```

### Implementing Module Interface

```go
// internal/modules/tasks/module.go
package tasks

import (
    "yourapp/internal/system/db"
    "yourapp/internal/system/module"
    "yourapp/internal/modules/tasks/handlers"
    "yourapp/internal/modules/tasks/models"
)

// Ensure TasksModule implements required interfaces
var _ module.Module = (*TasksModule)(nil)
var _ module.DatabaseInitializer = (*TasksModule)(nil)

type TasksModule struct {
    handler *handlers.TasksHandler
}

func New(database *db.Database) *TasksModule {
    repo := models.NewTaskRepository(database.DB())
    return &TasksModule{
        handler: handlers.NewTasksHandler(repo),
    }
}

func (m *TasksModule) GetRoutes() []module.Route {
    return []module.Route{
        {Blueprint: m.handler.Router(), Prefix: "/tasks"},
    }
}

func (m *TasksModule) InitDatabase() error {
    return models.Migrate(db.Get())
}

func (m *TasksModule) Manifest() module.Manifest {
    return module.Manifest{
        Name:        "Tasks",
        Version:     "1.0",
        MainRoute:   "/tasks",
        Type:        "App",
        Depends:     []string{"core"},
        IconClass:   "fa-solid fa-check",
        Color:       "#28a745",
        Description: "Task management",
    }
}
```

### Interface Composition

```go
// Compose interfaces for flexible requirements
type Reader interface {
    Read() ([]byte, error)
}

type Writer interface {
    Write(data []byte) error
}

// Combined interface
type ReadWriter interface {
    Reader
    Writer
}

// Accept any type implementing ReadWriter
func Process(rw ReadWriter) error {
    data, err := rw.Read()
    if err != nil {
        return err
    }
    return rw.Write(data)
}
```

---

## Struct Patterns

### Model Struct with GORM

```go
// internal/modules/tasks/models/task.go
package models

import (
    "time"

    "gorm.io/gorm"
)

type Task struct {
    ID          uint           `gorm:"primaryKey" json:"id"`
    Title       string         `gorm:"size:255;not null" json:"title"`
    Description string         `gorm:"type:text" json:"description"`
    Priority    int            `gorm:"default:1" json:"priority"`
    IsComplete  bool           `gorm:"default:false" json:"is_complete"`
    DueDate     *time.Time     `json:"due_date,omitempty"`
    UserID      uint           `gorm:"index" json:"user_id"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

    // Relationships
    User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Task) TableName() string {
    return "tasks"
}
```

### Embedded Structs

```go
// Base fields to embed in all models
type BaseModel struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Use in models
type Task struct {
    BaseModel
    Title       string `gorm:"size:255;not null" json:"title"`
    Description string `gorm:"type:text" json:"description"`
}
```

### Struct Methods

```go
// Methods on struct types
func (t *Task) IsOverdue() bool {
    if t.DueDate == nil {
        return false
    }
    return time.Now().After(*t.DueDate) && !t.IsComplete
}

func (t *Task) MarkComplete() {
    t.IsComplete = true
}

func (t *Task) Validate() error {
    if strings.TrimSpace(t.Title) == "" {
        return errors.New("title is required")
    }
    if len(t.Title) > 255 {
        return errors.New("title too long")
    }
    return nil
}
```

---

## Generics

### Generic Repository

```go
// internal/system/db/repository.go
package db

import "gorm.io/gorm"

// Generic repository for common CRUD operations
type Repository[T any] struct {
    db *gorm.DB
}

func NewRepository[T any](db *gorm.DB) *Repository[T] {
    return &Repository[T]{db: db}
}

func (r *Repository[T]) Create(entity *T) error {
    return r.db.Create(entity).Error
}

func (r *Repository[T]) GetByID(id uint) (*T, error) {
    var entity T
    err := r.db.First(&entity, id).Error
    if err != nil {
        return nil, err
    }
    return &entity, nil
}

func (r *Repository[T]) GetAll() ([]T, error) {
    var entities []T
    err := r.db.Find(&entities).Error
    return entities, err
}

func (r *Repository[T]) Update(entity *T) error {
    return r.db.Save(entity).Error
}

func (r *Repository[T]) Delete(id uint) error {
    var entity T
    return r.db.Delete(&entity, id).Error
}
```

### Usage

```go
// Create typed repository
taskRepo := db.NewRepository[Task](database)

// Type-safe operations
task, err := taskRepo.GetByID(1)      // Returns *Task, error
tasks, err := taskRepo.GetAll()        // Returns []Task, error
err = taskRepo.Create(&newTask)        // Accepts *Task
```

### Generic Utility Functions

```go
// Pointer helper for optional values
func Ptr[T any](v T) *T {
    return &v
}

// Usage
task := Task{
    Title:   "Example",
    DueDate: Ptr(time.Now().AddDate(0, 0, 7)),  // *time.Time
}

// Map function
func Map[T, U any](items []T, fn func(T) U) []U {
    result := make([]U, len(items))
    for i, item := range items {
        result[i] = fn(item)
    }
    return result
}

// Usage
names := Map(users, func(u User) string { return u.Name })

// Filter function
func Filter[T any](items []T, predicate func(T) bool) []T {
    result := make([]T, 0)
    for _, item := range items {
        if predicate(item) {
            result = append(result, item)
        }
    }
    return result
}

// Usage
activeTasks := Filter(tasks, func(t Task) bool { return !t.IsComplete })
```

### Generic Constraints

```go
import "golang.org/x/exp/constraints"

// Numeric constraint
func Sum[T constraints.Integer | constraints.Float](values []T) T {
    var sum T
    for _, v := range values {
        sum += v
    }
    return sum
}

// Custom constraint
type Validator interface {
    Validate() error
}

func ValidateAll[T Validator](items []T) error {
    for _, item := range items {
        if err := item.Validate(); err != nil {
            return err
        }
    }
    return nil
}
```

---

## JSON Struct Tags

### Complete Tag Reference

```go
type User struct {
    // Basic JSON tag
    ID uint `json:"id"`

    // Omit if zero value
    Name string `json:"name,omitempty"`

    // Skip in JSON entirely
    PasswordHash string `json:"-"`

    // Custom name
    EmailAddress string `json:"email"`

    // Pointer for nullable fields
    AvatarURL *string `json:"avatar_url,omitempty"`

    // Nested struct
    Profile Profile `json:"profile"`

    // Slice
    Roles []string `json:"roles"`

    // String representation of number
    Balance float64 `json:"balance,string"`
}
```

### Form Binding Tags

```go
type CreateTaskForm struct {
    Title       string     `form:"title" binding:"required,max=255"`
    Description string     `form:"description" binding:"max=1000"`
    Priority    int        `form:"priority" binding:"min=1,max=5"`
    DueDate     *time.Time `form:"due_date" time_format:"2006-01-02"`
}

// Usage in handler
func (h *Handler) Create(c *gin.Context) {
    var form CreateTaskForm
    if err := c.ShouldBind(&form); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // form is now validated and populated
}
```

### Custom JSON Marshaling

```go
type Status int

const (
    StatusPending Status = iota
    StatusActive
    StatusComplete
)

func (s Status) MarshalJSON() ([]byte, error) {
    names := map[Status]string{
        StatusPending:  "pending",
        StatusActive:   "active",
        StatusComplete: "complete",
    }
    return json.Marshal(names[s])
}

func (s *Status) UnmarshalJSON(data []byte) error {
    var name string
    if err := json.Unmarshal(data, &name); err != nil {
        return err
    }
    values := map[string]Status{
        "pending":  StatusPending,
        "active":   StatusActive,
        "complete": StatusComplete,
    }
    *s = values[name]
    return nil
}
```

---

## Error Types

### Custom Error Types

```go
// internal/system/errors/errors.go
package errors

import "fmt"

// ValidationError for input validation failures
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
    return &ValidationError{Field: field, Message: message}
}

// NotFoundError for missing resources
type NotFoundError struct {
    Resource string
    ID       interface{}
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s with id %v not found", e.Resource, e.ID)
}

func NewNotFoundError(resource string, id interface{}) *NotFoundError {
    return &NotFoundError{Resource: resource, ID: id}
}

// AuthorizationError for permission failures
type AuthorizationError struct {
    Action   string
    Resource string
}

func (e *AuthorizationError) Error() string {
    return fmt.Sprintf("not authorized to %s %s", e.Action, e.Resource)
}
```

### Error Handling Pattern

```go
func (h *Handler) GetTask(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }

    task, err := h.taskRepo.GetByID(uint(id))
    if err != nil {
        // Handle specific error types
        var notFound *errors.NotFoundError
        if errors.As(err, &notFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
            return
        }
        // Unknown error
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }

    c.JSON(http.StatusOK, task)
}
```

### Sentinel Errors

```go
import "errors"

var (
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrInvalidInput  = errors.New("invalid input")
    ErrAlreadyExists = errors.New("already exists")
)

// Usage
func (r *Repository) GetByEmail(email string) (*User, error) {
    var user User
    err := r.db.Where("email = ?", email).First(&user).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, ErrNotFound
    }
    return &user, err
}

// Checking
user, err := repo.GetByEmail(email)
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

---

## Common Patterns

### Optional Fields with Pointers

```go
type UpdateTaskRequest struct {
    Title       *string    `json:"title"`
    Description *string    `json:"description"`
    Priority    *int       `json:"priority"`
    IsComplete  *bool      `json:"is_complete"`
    DueDate     *time.Time `json:"due_date"`
}

func (r *Repository) Update(id uint, req UpdateTaskRequest) error {
    updates := make(map[string]interface{})

    if req.Title != nil {
        updates["title"] = *req.Title
    }
    if req.Description != nil {
        updates["description"] = *req.Description
    }
    if req.Priority != nil {
        updates["priority"] = *req.Priority
    }
    if req.IsComplete != nil {
        updates["is_complete"] = *req.IsComplete
    }
    if req.DueDate != nil {
        updates["due_date"] = *req.DueDate
    }

    if len(updates) == 0 {
        return nil
    }

    return r.db.Model(&Task{}).Where("id = ?", id).Updates(updates).Error
}
```

### Response Types

```go
// API response wrapper
type Response[T any] struct {
    Success bool   `json:"success"`
    Data    T      `json:"data,omitempty"`
    Error   string `json:"error,omitempty"`
}

func SuccessResponse[T any](data T) Response[T] {
    return Response[T]{Success: true, Data: data}
}

func ErrorResponse[T any](message string) Response[T] {
    return Response[T]{Success: false, Error: message}
}

// Usage
c.JSON(http.StatusOK, SuccessResponse(task))
c.JSON(http.StatusBadRequest, ErrorResponse[Task]("invalid input"))
```

### Pagination

```go
type PaginatedResult[T any] struct {
    Items      []T   `json:"items"`
    Total      int64 `json:"total"`
    Page       int   `json:"page"`
    PerPage    int   `json:"per_page"`
    TotalPages int   `json:"total_pages"`
}

type PaginationParams struct {
    Page    int `form:"page" binding:"min=1"`
    PerPage int `form:"per_page" binding:"min=1,max=100"`
}

func (p *PaginationParams) Defaults() {
    if p.Page == 0 {
        p.Page = 1
    }
    if p.PerPage == 0 {
        p.PerPage = 20
    }
}

func (r *Repository[T]) Paginate(params PaginationParams) (*PaginatedResult[T], error) {
    params.Defaults()

    var total int64
    r.db.Model(new(T)).Count(&total)

    var items []T
    offset := (params.Page - 1) * params.PerPage
    err := r.db.Offset(offset).Limit(params.PerPage).Find(&items).Error

    totalPages := int(total) / params.PerPage
    if int(total)%params.PerPage > 0 {
        totalPages++
    }

    return &PaginatedResult[T]{
        Items:      items,
        Total:      total,
        Page:       params.Page,
        PerPage:    params.PerPage,
        TotalPages: totalPages,
    }, err
}
```

---

## Type Assertions

### Basic Type Assertion

```go
// Single assertion (panics if wrong type)
str := value.(string)

// Safe assertion with ok check
str, ok := value.(string)
if !ok {
    // Handle wrong type
}
```

### Type Switch

```go
func HandleValue(value interface{}) string {
    switch v := value.(type) {
    case string:
        return v
    case int:
        return strconv.Itoa(v)
    case bool:
        if v {
            return "true"
        }
        return "false"
    case nil:
        return ""
    default:
        return fmt.Sprintf("%v", v)
    }
}
```

### Interface Satisfaction Check

```go
// Compile-time check that type implements interface
var _ Module = (*TasksModule)(nil)
var _ DatabaseInitializer = (*TasksModule)(nil)

// Runtime check
if initializer, ok := module.(DatabaseInitializer); ok {
    initializer.InitDatabase()
}
```

---

## Checklist for New Modules

Before committing, verify:

- [ ] All exported functions have documentation comments
- [ ] Struct fields have appropriate JSON/GORM tags
- [ ] Interfaces are satisfied (compile-time checks)
- [ ] Error types are used consistently
- [ ] Pointer types for optional fields
- [ ] Generic types where appropriate
- [ ] Code compiles without errors (`go build ./...`)
- [ ] Linting passes (`golangci-lint run`)

---

## Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Blog: Type Parameters](https://go.dev/blog/intro-generics)
- [GORM Documentation](https://gorm.io/docs/)
- [Gin Binding Documentation](https://gin-gonic.com/docs/examples/binding-and-validation/)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

---

**Next:** [Testing](testing.md) | [Deployment](deployment.md)
