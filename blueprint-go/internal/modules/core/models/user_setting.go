package models

import (
	"time"

	"gorm.io/gorm"

	"blueprint-go/internal/system/db"
)

func init() {
	db.Register("core", "UserSetting", "user_settings")
}

// UserSetting represents a key-value setting for a user
type UserSetting struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Key       string         `gorm:"size:255;not null" json:"key"`
	Value     string         `gorm:"size:255" json:"value"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for UserSetting
func (UserSetting) TableName() string {
	return "user_settings"
}

// UniqueIndex for user_id + key combination
func (UserSetting) TableArgs() string {
	return "UNIQUE INDEX idx_user_key (user_id, key)"
}

// --- Repository Pattern ---

// UserSettingRepository provides database operations for UserSetting
type UserSettingRepository struct {
	db *gorm.DB
}

// NewUserSettingRepository creates a new UserSettingRepository
func NewUserSettingRepository(db *gorm.DB) *UserSettingRepository {
	return &UserSettingRepository{db: db}
}

// Get retrieves a setting value for a user
func (r *UserSettingRepository) Get(userID uint, key string, defaultValue string) string {
	var setting UserSetting
	err := r.db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error
	if err != nil {
		return defaultValue
	}
	return setting.Value
}

// Set sets a setting value for a user
func (r *UserSettingRepository) Set(userID uint, key, value string) error {
	var setting UserSetting
	err := r.db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error

	if err == gorm.ErrRecordNotFound {
		setting = UserSetting{
			UserID: userID,
			Key:    key,
			Value:  value,
		}
		return r.db.Create(&setting).Error
	}

	if err != nil {
		return err
	}

	setting.Value = value
	return r.db.Save(&setting).Error
}

// Delete removes a setting for a user
func (r *UserSettingRepository) Delete(userID uint, key string) error {
	return r.db.Where("user_id = ? AND key = ?", userID, key).Delete(&UserSetting{}).Error
}

// GetAllForUser retrieves all settings for a user
func (r *UserSettingRepository) GetAllForUser(userID uint) (map[string]string, error) {
	var settings []UserSetting
	err := r.db.Where("user_id = ?", userID).Find(&settings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}
