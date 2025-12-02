package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Item represents a sample model for this module
type Item struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for this model
func (Item) TableName() string {
	return "modulename_items"
}

// Validate validates the item before saving
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

// ItemRepository handles database operations for Item
type ItemRepository struct {
	db *gorm.DB
}

// NewItemRepository creates a new item repository
func NewItemRepository(db *gorm.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// Create creates a new item
func (r *ItemRepository) Create(item *Item) error {
	if err := item.Validate(); err != nil {
		return err
	}
	return r.db.Create(item).Error
}

// GetByID finds an item by ID
func (r *ItemRepository) GetByID(id uint) (*Item, error) {
	var item Item
	err := r.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetAll returns all active items
func (r *ItemRepository) GetAll() ([]Item, error) {
	var items []Item
	err := r.db.Where("is_active = ?", true).Order("created_at DESC").Find(&items).Error
	return items, err
}

// Update updates an item
func (r *ItemRepository) Update(item *Item) error {
	if err := item.Validate(); err != nil {
		return err
	}
	return r.db.Save(item).Error
}

// Delete soft-deletes an item
func (r *ItemRepository) Delete(id uint) error {
	return r.db.Delete(&Item{}, id).Error
}

// CreateSampleData creates initial sample data (idempotent)
func (r *ItemRepository) CreateSampleData() error {
	var count int64
	r.db.Model(&Item{}).Count(&count)
	if count > 0 {
		return nil // Already has data
	}

	samples := []Item{
		{Name: "Sample Item 1", Description: "This is a sample item"},
		{Name: "Sample Item 2", Description: "Another sample item"},
	}

	for _, item := range samples {
		if err := r.Create(&item); err != nil {
			return err
		}
	}

	return nil
}
