package models

import (
	"time"

	"gorm.io/gorm"

	"blueprint-go/internal/system/db"
)

func init() {
	db.Register("core", "Group", "groups")
}

// Group represents a permission group for access control
type Group struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"uniqueIndex;size:64;not null" json:"name"`
	Description string         `gorm:"size:255" json:"description"`
	IsSystem    bool           `gorm:"default:false" json:"is_system"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Users []User `gorm:"many2many:user_groups;" json:"users,omitempty"`
}

// TableName specifies the table name for Group
func (Group) TableName() string {
	return "groups"
}

// --- Repository Pattern ---

// GroupRepository provides database operations for Group
type GroupRepository struct {
	db *gorm.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Create creates a new group
func (r *GroupRepository) Create(name, description string, isSystem bool) (*Group, error) {
	group := &Group{
		Name:        name,
		Description: description,
		IsSystem:    isSystem,
	}

	if err := r.db.Create(group).Error; err != nil {
		return nil, err
	}

	return group, nil
}

// GetOrCreate gets an existing group or creates a new one
func (r *GroupRepository) GetOrCreate(name, description string, isSystem bool) (*Group, error) {
	var group Group
	err := r.db.Where("name = ?", name).First(&group).Error

	if err == gorm.ErrRecordNotFound {
		return r.Create(name, description, isSystem)
	}

	return &group, err
}

// GetByID retrieves a group by ID
func (r *GroupRepository) GetByID(id uint) (*Group, error) {
	var group Group
	err := r.db.First(&group, id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetByName retrieves a group by name
func (r *GroupRepository) GetByName(name string) (*Group, error) {
	var group Group
	err := r.db.Where("name = ?", name).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetAll retrieves all groups
func (r *GroupRepository) GetAll() ([]Group, error) {
	var groups []Group
	err := r.db.Find(&groups).Error
	return groups, err
}

// Delete deletes a group (only non-system groups)
func (r *GroupRepository) Delete(id uint) error {
	// Prevent deletion of system groups
	var group Group
	if err := r.db.First(&group, id).Error; err != nil {
		return err
	}

	if group.IsSystem {
		return gorm.ErrInvalidData
	}

	return r.db.Delete(&Group{}, id).Error
}

// CreateSystemGroups creates the default system groups
func (r *GroupRepository) CreateSystemGroups() error {
	_, err := r.GetOrCreate("ALL", "All authenticated users", true)
	if err != nil {
		return err
	}

	_, err = r.GetOrCreate("ADMIN", "Administrators", true)
	return err
}
