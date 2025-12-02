package auth

import (
	"github.com/gin-gonic/gin"

	"blueprint-go/internal/modules/core/models"
)

const (
	// ContextUserKey is the key for storing user in Gin context
	ContextUserKey = "user"

	// ContextLangKey is the key for storing language in Gin context
	ContextLangKey = "lang"

	// ContextModuleKey is the key for storing current module in Gin context
	ContextModuleKey = "current_module"
)

// SetUser stores the user in the Gin context
func SetUser(c *gin.Context, user *models.User) {
	c.Set(ContextUserKey, user)
}

// GetUser retrieves the user from the Gin context
func GetUser(c *gin.Context) *models.User {
	val, exists := c.Get(ContextUserKey)
	if !exists {
		return nil
	}

	user, ok := val.(*models.User)
	if !ok {
		return nil
	}

	return user
}

// IsAuthenticated checks if a user is logged in
func IsAuthenticated(c *gin.Context) bool {
	return GetUser(c) != nil
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	user := GetUser(c)
	if user == nil {
		return false
	}
	return user.IsAdmin()
}

// HasGroup checks if the current user belongs to a group
func HasGroup(c *gin.Context, groupName string) bool {
	user := GetUser(c)
	if user == nil {
		return false
	}
	return user.HasGroup(groupName)
}

// SetLang stores the language in the Gin context
func SetLang(c *gin.Context, lang string) {
	c.Set(ContextLangKey, lang)
}

// GetLang retrieves the language from the Gin context
func GetLang(c *gin.Context) string {
	val, exists := c.Get(ContextLangKey)
	if !exists {
		return "en"
	}

	lang, ok := val.(string)
	if !ok {
		return "en"
	}

	return lang
}
