# Audit Trail Pattern

Track who created and last updated records using embedded audit fields.

---

## Overview

Audit fields provide automatic tracking of:
- **CreatedByID** - User who created the record (set once, never changes)
- **UpdatedByID** - User who last modified the record (updated on each save)

These are system-managed, readonly fields from the user's perspective.

---

## Implementation

### 1. Create AuditFields Struct

```go
// internal/system/db/audit.go
package db

import (
    "time"
)

// AuditFields provides created_by and updated_by tracking
type AuditFields struct {
    CreatedByID *uint     `gorm:"index" json:"created_by_id,omitempty"`
    UpdatedByID *uint     `gorm:"index" json:"updated_by_id,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// SetCreatedBy sets the creator (call only on create)
func (a *AuditFields) SetCreatedBy(userID uint) {
    a.CreatedByID = &userID
}

// SetUpdatedBy sets the last updater
func (a *AuditFields) SetUpdatedBy(userID uint) {
    a.UpdatedByID = &userID
}
```

### 2. Create Audit Context Helper

```go
// internal/system/auth/context.go
package auth

import (
    "github.com/gin-gonic/gin"
)

const userContextKey = "current_user"

// GetUser returns the authenticated user from context
func GetUser(c *gin.Context) *User {
    if user, exists := c.Get(userContextKey); exists {
        return user.(*User)
    }
    return nil
}

// GetUserID returns the authenticated user's ID from context
func GetUserID(c *gin.Context) *uint {
    user := GetUser(c)
    if user != nil {
        return &user.ID
    }
    return nil
}
```

### 3. Apply to Models

```go
// internal/modules/contacts/models/contact.go
package models

import (
    "yourapp/internal/system/db"
    "gorm.io/gorm"
)

func init() {
    db.Register("contacts", "Contact", "contacts")
}

type Contact struct {
    ID        uint   `gorm:"primaryKey" json:"id"`
    Name      string `gorm:"size:255;not null" json:"name"`
    Email     string `gorm:"size:255" json:"email"`
    Phone     string `gorm:"size:50" json:"phone"`

    // Embed audit fields
    db.AuditFields
}

func (Contact) TableName() string {
    return "contacts"
}
```

### 4. Repository with Audit Tracking

```go
// internal/modules/contacts/models/contact_repository.go
package models

import (
    "gorm.io/gorm"
)

type ContactRepository struct {
    db *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
    return &ContactRepository{db: db}
}

// Create a new contact with audit trail
func (r *ContactRepository) Create(contact *Contact, createdByID *uint) error {
    if createdByID != nil {
        contact.SetCreatedBy(*createdByID)
    }
    return r.db.Create(contact).Error
}

// Update a contact with audit trail
func (r *ContactRepository) Update(contact *Contact, updatedByID *uint) error {
    if updatedByID != nil {
        contact.SetUpdatedBy(*updatedByID)
    }
    return r.db.Save(contact).Error
}

// UpdateFields updates specific fields with audit trail
func (r *ContactRepository) UpdateFields(id uint, updates map[string]interface{}, updatedByID *uint) error {
    if updatedByID != nil {
        updates["updated_by_id"] = *updatedByID
    }
    return r.db.Model(&Contact{}).Where("id = ?", id).Updates(updates).Error
}
```

### 5. Handler Usage

```go
// internal/modules/contacts/handlers/routes.go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "yourapp/internal/system/auth"
    "yourapp/internal/modules/contacts/models"
)

type ContactsHandler struct {
    contactRepo *models.ContactRepository
}

func (h *ContactsHandler) Create(c *gin.Context) {
    var contact models.Contact
    if err := c.ShouldBind(&contact); err != nil {
        c.HTML(http.StatusBadRequest, "contacts/form.html", gin.H{
            "error": err.Error(),
        })
        return
    }

    // Get current user ID for audit
    userID := auth.GetUserID(c)

    if err := h.contactRepo.Create(&contact, userID); err != nil {
        c.HTML(http.StatusInternalServerError, "contacts/form.html", gin.H{
            "error": "Failed to create contact",
        })
        return
    }

    c.Redirect(http.StatusSeeOther, "/contacts")
}

func (h *ContactsHandler) Update(c *gin.Context) {
    id := parseUint(c.Param("id"))

    contact, err := h.contactRepo.GetByID(id)
    if err != nil {
        c.HTML(http.StatusNotFound, "errors/404.html", nil)
        return
    }

    if err := c.ShouldBind(&contact); err != nil {
        c.HTML(http.StatusBadRequest, "contacts/edit.html", gin.H{
            "error":   err.Error(),
            "contact": contact,
        })
        return
    }

    // Get current user ID for audit
    userID := auth.GetUserID(c)

    if err := h.contactRepo.Update(contact, userID); err != nil {
        c.HTML(http.StatusInternalServerError, "contacts/edit.html", gin.H{
            "error":   "Failed to update contact",
            "contact": contact,
        })
        return
    }

    c.Redirect(http.StatusSeeOther, "/contacts/"+c.Param("id"))
}
```

---

## Preloading Audit Users

To display creator/updater names, preload the User relationship:

### Model with User Relationships

```go
// internal/modules/contacts/models/contact.go
package models

import (
    "yourapp/internal/system/db"
    coremodels "yourapp/internal/modules/core/models"
)

type Contact struct {
    ID    uint   `gorm:"primaryKey" json:"id"`
    Name  string `gorm:"size:255;not null" json:"name"`
    Email string `gorm:"size:255" json:"email"`

    // Audit fields
    db.AuditFields

    // Relationships for display (not stored in table)
    CreatedBy *coremodels.User `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
    UpdatedBy *coremodels.User `gorm:"foreignKey:UpdatedByID" json:"updated_by,omitempty"`
}

// CreatedByName returns the creator's full name
func (c *Contact) CreatedByName() string {
    if c.CreatedBy != nil {
        return c.CreatedBy.FirstName + " " + c.CreatedBy.LastName
    }
    return ""
}

// UpdatedByName returns the last updater's full name
func (c *Contact) UpdatedByName() string {
    if c.UpdatedBy != nil {
        return c.UpdatedBy.FirstName + " " + c.UpdatedBy.LastName
    }
    return ""
}
```

### Repository with Preload

```go
// Get contact with audit user data
func (r *ContactRepository) GetByID(id uint) (*Contact, error) {
    var contact Contact
    err := r.db.
        Preload("CreatedBy").
        Preload("UpdatedBy").
        First(&contact, id).Error
    if err != nil {
        return nil, err
    }
    return &contact, nil
}

// Get all contacts with audit user data
func (r *ContactRepository) GetAll() ([]Contact, error) {
    var contacts []Contact
    err := r.db.
        Preload("CreatedBy").
        Preload("UpdatedBy").
        Order("created_at DESC").
        Find(&contacts).Error
    return contacts, err
}
```

---

## Display in Templates

```html
<!-- In detail view -->
<div class="text-muted small mt-4">
    <p>
        Created: {{.contact.CreatedAt.Format "2006-01-02 15:04"}}
        {{if .contact.CreatedByName}}
        by {{.contact.CreatedByName}}
        {{end}}
    </p>
    {{if .contact.UpdatedByName}}
    <p>
        Last updated: {{.contact.UpdatedAt.Format "2006-01-02 15:04"}}
        by {{.contact.UpdatedByName}}
    </p>
    {{end}}
</div>
```

### In List View (Table)

```html
<table class="table">
    <thead>
        <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Created</th>
            <th>Modified By</th>
        </tr>
    </thead>
    <tbody>
        {{range .contacts}}
        <tr>
            <td>{{.Name}}</td>
            <td>{{.Email}}</td>
            <td>{{.CreatedAt.Format "2006-01-02"}}</td>
            <td>{{if .UpdatedByName}}{{.UpdatedByName}}{{else}}-{{end}}</td>
        </tr>
        {{end}}
    </tbody>
</table>
```

---

## GORM Hooks (Alternative)

For automatic audit tracking without explicit repository calls:

```go
// internal/system/db/audit_hooks.go
package db

import (
    "gorm.io/gorm"
)

// currentUserIDGetter is set by auth middleware
var currentUserIDGetter func() *uint

// SetUserIDGetter configures the function to get current user ID
func SetUserIDGetter(fn func() *uint) {
    currentUserIDGetter = fn
}

// BeforeCreate GORM hook
func (a *AuditFields) BeforeCreate(tx *gorm.DB) error {
    if currentUserIDGetter != nil {
        if userID := currentUserIDGetter(); userID != nil {
            a.CreatedByID = userID
        }
    }
    return nil
}

// BeforeUpdate GORM hook
func (a *AuditFields) BeforeUpdate(tx *gorm.DB) error {
    if currentUserIDGetter != nil {
        if userID := currentUserIDGetter(); userID != nil {
            a.UpdatedByID = userID
        }
    }
    return nil
}
```

**Note:** GORM hooks with request context require careful goroutine management. The explicit repository approach is recommended for clarity.

---

## Key Points

- Fields are **nullable** (`*uint`) to handle existing records and public submissions
- Use `Preload()` to avoid N+1 queries when displaying user names
- Check for `nil` before accessing user relationships
- The `*Name()` methods provide convenient display formatting
- Embed `AuditFields` struct for consistent field naming
- Pass `userID` explicitly to repository methods for clarity and testability

---

**Next:** [Security](security.md) | [Testing](testing.md)
