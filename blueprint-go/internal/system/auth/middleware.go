package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blueprint-go/internal/modules/core/models"
	"blueprint-go/internal/system/db"
)

// RequireLogin is middleware that requires authentication
// Equivalent to Python's @login_required decorator
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			// Store the requested URL for redirect after login
			next := c.Request.URL.Path
			c.Redirect(http.StatusSeeOther, "/login?next="+next)
			c.Abort()
			return
		}

		// Load user from database
		userRepo := models.NewUserRepository(db.Get())
		user, err := userRepo.GetByID(userID)
		if err != nil || !user.IsActive {
			ClearSession(c)
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Store user in context for handlers
		SetUser(c, user)
		c.Next()
	}
}

// RequireAdmin is middleware that requires admin group membership
// Equivalent to Python's @admin_required decorator
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user == nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		if !user.IsAdmin() {
			AddFlash(c, FlashError, "Admin access required")
			c.Redirect(http.StatusSeeOther, "/")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireGroup is middleware that requires membership in a specific group
// Equivalent to Python's @group_required("GroupName") decorator
func RequireGroup(groupName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user == nil {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		if !user.HasGroup(groupName) {
			AddFlash(c, FlashError, "Access denied: requires "+groupName+" group")
			c.Redirect(http.StatusSeeOther, "/")
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth is middleware that loads user if authenticated but doesn't require it
// Useful for pages that show different content for logged in users
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			c.Next()
			return
		}

		userRepo := models.NewUserRepository(db.Get())
		user, err := userRepo.GetByID(userID)
		if err == nil && user.IsActive {
			SetUser(c, user)
		}

		c.Next()
	}
}
