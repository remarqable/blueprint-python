package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"blueprint-go/internal/system/db"
)

func init() {
	db.Register("core", "User", "users")
}

// User represents an authenticated user in the system
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Email        string         `gorm:"uniqueIndex;size:120;not null" json:"email"`
	PasswordHash string         `gorm:"size:200" json:"-"`
	FirstName    string         `gorm:"size:50" json:"first_name"`
	LastName     string         `gorm:"size:50" json:"last_name"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Groups   []Group       `gorm:"many2many:user_groups;" json:"groups,omitempty"`
	Settings []UserSetting `json:"settings,omitempty"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// SetPassword hashes and sets the password
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies the password against the hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// IsAdmin checks if user belongs to ADMIN group
func (u *User) IsAdmin() bool {
	for _, g := range u.Groups {
		if g.Name == "ADMIN" {
			return true
		}
	}
	return false
}

// HasGroup checks if user belongs to a specific group
func (u *User) HasGroup(groupName string) bool {
	for _, g := range u.Groups {
		if g.Name == groupName {
			return true
		}
	}
	return false
}

// FullName returns the user's full name
func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return u.Email
	}
	return u.FirstName + " " + u.LastName
}

// --- Repository Pattern ---

// UserRepository provides database operations for User
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user with the given details
func (r *UserRepository) Create(email, password, firstName, lastName string, isAdmin bool) (*User, error) {
	user := &User{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		IsActive:  true,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := r.db.Create(user).Error; err != nil {
		return nil, err
	}

	// Add to ALL group
	var allGroup Group
	r.db.FirstOrCreate(&allGroup, Group{Name: "ALL"})
	r.db.Model(user).Association("Groups").Append(&allGroup)

	if isAdmin {
		var adminGroup Group
		r.db.FirstOrCreate(&adminGroup, Group{Name: "ADMIN"})
		r.db.Model(user).Association("Groups").Append(&adminGroup)
	}

	return user, nil
}

// GetByID retrieves a user by ID with groups preloaded
func (r *UserRepository) GetByID(id uint) (*User, error) {
	var user User
	err := r.db.Preload("Groups").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email with groups preloaded
func (r *UserRepository) GetByEmail(email string) (*User, error) {
	var user User
	err := r.db.Preload("Groups").Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAll retrieves all users
func (r *UserRepository) GetAll() ([]User, error) {
	var users []User
	err := r.db.Preload("Groups").Find(&users).Error
	return users, err
}

// Update updates a user's details
func (r *UserRepository) Update(user *User) error {
	return r.db.Save(user).Error
}

// Delete soft-deletes a user
func (r *UserRepository) Delete(id uint) error {
	return r.db.Delete(&User{}, id).Error
}

// CreateSampleData creates sample users if none exist
func (r *UserRepository) CreateSampleData() error {
	var count int64
	r.db.Model(&User{}).Count(&count)
	if count > 0 {
		return nil
	}

	// Create default admin user
	_, err := r.Create("admin@example.com", "admin", "Admin", "User", true)
	return err
}
