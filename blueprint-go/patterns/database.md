# Database Patterns

> Complete guide to database operations in Blueprint Go using GORM.

---

## Overview

Blueprint uses **GORM** for all database operations:

- Auto-migrations
- CRUD operations
- Relationships (one-to-many, many-to-many)
- Transactions
- Raw SQL when needed

---

## Initialization

```go
// internal/system/db/database.go

import "gorm.io/gorm"

// Initialize sets up the GORM connection
func Initialize(databaseURL string, debug bool) error

// Get returns the GORM instance
func Get() *gorm.DB

// Close closes the connection
func Close() error

// Transaction runs a function within a transaction
func Transaction(fn func(tx *gorm.DB) error) error
```

### Usage

```go
import "blueprint-go/internal/system/db"

gormDB := db.Get()
gormDB.Create(&item)
```

---

## Model Registry

Track all models for introspection:

```go
// In each model file
func init() {
    db.Register("modulename", "ModelName", "table_name")
}
```

### Print Summary

```go
db.PrintSummary()

// Output:
// Database Model Registry:
// --------   ----------   --------------------
// Module     Model        Table
// --------   ----------   --------------------
// core       User         users
// core       Group        groups
// tasks      Task         tasks
```

---

## Model Definition

### Basic Model

```go
package models

import (
    "time"
    "gorm.io/gorm"
    "blueprint-go/internal/system/db"
)

func init() {
    db.Register("yourmodule", "Item", "yourmodule_items")
}

type Item struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    Name      string         `gorm:"size:255;not null" json:"name"`
    IsActive  bool           `gorm:"default:true" json:"is_active"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Item) TableName() string {
    return "yourmodule_items"
}
```

### With Relationships

```go
type User struct {
    ID     uint    `gorm:"primaryKey"`
    Email  string  `gorm:"uniqueIndex;size:120"`
    Groups []Group `gorm:"many2many:user_groups;"`
}

type Group struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"uniqueIndex;size:64"`
    Users []User `gorm:"many2many:user_groups;"`
}
```

---

## Repository Pattern (Fat Models)

All database operations are encapsulated in repositories:

```go
// internal/modules/yourmodule/models/item.go

type ItemRepository struct {
    db *gorm.DB
}

func NewItemRepository(db *gorm.DB) *ItemRepository {
    return &ItemRepository{db: db}
}

// Create
func (r *ItemRepository) Create(name string) (*Item, error) {
    item := &Item{Name: name}
    if err := r.db.Create(item).Error; err != nil {
        return nil, err
    }
    return item, nil
}

// Read
func (r *ItemRepository) GetByID(id uint) (*Item, error) {
    var item Item
    err := r.db.First(&item, id).Error
    if err != nil {
        return nil, err
    }
    return &item, nil
}

func (r *ItemRepository) GetAll() ([]Item, error) {
    var items []Item
    err := r.db.Find(&items).Error
    return items, err
}

// Update
func (r *ItemRepository) Update(id uint, name string) (*Item, error) {
    item, err := r.GetByID(id)
    if err != nil {
        return nil, err
    }
    item.Name = name
    err = r.db.Save(item).Error
    return item, err
}

// Delete
func (r *ItemRepository) Delete(id uint) error {
    return r.db.Delete(&Item{}, id).Error
}

// Sample Data (idempotent)
func (r *ItemRepository) CreateSampleData() error {
    var count int64
    r.db.Model(&Item{}).Count(&count)
    if count > 0 {
        return nil
    }

    r.Create("Sample Item 1")
    r.Create("Sample Item 2")
    return nil
}
```

---

## Query Patterns

### Basic Queries

```go
// Find by ID
var item Item
db.First(&item, 1)

// Find by condition
db.Where("name = ?", "test").First(&item)

// Find all
var items []Item
db.Find(&items)

// Find with limit/offset
db.Limit(10).Offset(20).Find(&items)

// Order
db.Order("created_at DESC").Find(&items)
```

### Filtering

```go
// Single condition
db.Where("is_active = ?", true).Find(&items)

// Multiple conditions
db.Where("is_active = ? AND category_id = ?", true, 5).Find(&items)

// In clause
db.Where("id IN ?", []uint{1, 2, 3}).Find(&items)

// Like
db.Where("name LIKE ?", "%test%").Find(&items)
```

### Select Specific Columns

```go
db.Select("id", "name").Find(&items)
```

---

## Transactions

```go
func (r *ItemRepository) TransferOwnership(itemID, newOwnerID uint) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        // Update item owner
        if err := tx.Model(&Item{}).
            Where("id = ?", itemID).
            Update("owner_id", newOwnerID).Error; err != nil {
            return err
        }

        // Log the transfer
        log := &TransferLog{
            ItemID:     itemID,
            NewOwnerID: newOwnerID,
        }
        if err := tx.Create(log).Error; err != nil {
            return err
        }

        return nil // Commit
    })
}
```

---

## Raw SQL

When you need raw SQL:

```go
// Select
var results []ItemStats
db.Raw("SELECT category, COUNT(*) as count FROM items GROUP BY category").Scan(&results)

// Exec
db.Exec("UPDATE items SET is_active = ? WHERE created_at < ?", false, oldDate)
```

---

## Eager Loading (Preload)

```go
// Load user with groups
func (r *UserRepository) GetByID(id uint) (*User, error) {
    var user User
    err := r.db.Preload("Groups").First(&user, id).Error
    return &user, err
}

// Nested preload
func (r *OrderRepository) GetWithItems(id uint) (*Order, error) {
    var order Order
    err := r.db.
        Preload("Items").
        Preload("Items.Product").
        First(&order, id).Error
    return &order, err
}
```

---

## Pagination

```go
type PaginatedResult struct {
    Items      []Item `json:"items"`
    Total      int64  `json:"total"`
    Page       int    `json:"page"`
    PerPage    int    `json:"per_page"`
    TotalPages int    `json:"total_pages"`
}

func (r *ItemRepository) Paginate(page, perPage int) (*PaginatedResult, error) {
    var items []Item
    var total int64

    r.db.Model(&Item{}).Count(&total)

    offset := (page - 1) * perPage
    err := r.db.
        Offset(offset).
        Limit(perPage).
        Order("created_at DESC").
        Find(&items).Error

    if err != nil {
        return nil, err
    }

    totalPages := int(total) / perPage
    if int(total)%perPage > 0 {
        totalPages++
    }

    return &PaginatedResult{
        Items:      items,
        Total:      total,
        Page:       page,
        PerPage:    perPage,
        TotalPages: totalPages,
    }, nil
}
```

---

## Migrations with Goose

### Create Migration

```bash
make migrate-create
# Enter: add_categories_table
```

### Migration File

```sql
-- migrations/00002_add_categories_table.sql

-- +goose Up
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE categories;
```

### Run Migrations

```bash
make migrate-up     # Apply all pending
make migrate-down   # Rollback last
make migrate-status # Show status
```

---

## Best Practices

1. **Use repositories** - Encapsulate all DB operations
2. **Always use transactions** - For multi-step operations
3. **Preload relationships** - Avoid N+1 queries
4. **Index foreign keys** - Performance optimization
5. **Soft delete by default** - Use `DeletedAt` field
6. **Idempotent sample data** - Check before inserting
7. **Use migrations for production** - Don't rely on AutoMigrate alone
