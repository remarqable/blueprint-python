package auth

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

const (
	// SessionName is the name of the session cookie
	SessionName = "blueprint_session"

	// SessionUserID is the key for storing user ID in session
	SessionUserID = "user_id"

	// SessionFlash is the key for flash messages
	SessionFlash = "flash"
)

// SetupSessions configures session middleware for Gin
func SetupSessions(router *gin.Engine, secretKey string) {
	store := cookie.NewStore([]byte(secretKey))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   false, // Set true in production with HTTPS
		SameSite: 2,     // SameSiteLaxMode
	})

	router.Use(sessions.Sessions(SessionName, store))
}

// SetUserID stores the user ID in the session
func SetUserID(c *gin.Context, userID uint) error {
	session := sessions.Default(c)
	session.Set(SessionUserID, userID)
	return session.Save()
}

// GetUserID retrieves the user ID from the session
func GetUserID(c *gin.Context) (uint, bool) {
	session := sessions.Default(c)
	val := session.Get(SessionUserID)
	if val == nil {
		return 0, false
	}

	// Handle different integer types
	switch v := val.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case uint64:
		return uint(v), true
	default:
		return 0, false
	}
}

// ClearSession clears all session data
func ClearSession(c *gin.Context) error {
	session := sessions.Default(c)
	session.Clear()
	return session.Save()
}

// Flash message types
const (
	FlashSuccess = "success"
	FlashError   = "error"
	FlashWarning = "warning"
	FlashInfo    = "info"
)

// FlashMessage represents a flash message
type FlashMessage struct {
	Category string
	Message  string
}

// AddFlash adds a flash message to the session
func AddFlash(c *gin.Context, category, message string) {
	session := sessions.Default(c)
	flashes := session.Flashes(SessionFlash)

	messages := make([]FlashMessage, 0)
	for _, f := range flashes {
		if fm, ok := f.(FlashMessage); ok {
			messages = append(messages, fm)
		}
	}

	messages = append(messages, FlashMessage{
		Category: category,
		Message:  message,
	})

	for _, m := range messages {
		session.AddFlash(m, SessionFlash)
	}
	session.Save()
}

// GetFlashes retrieves and clears all flash messages
func GetFlashes(c *gin.Context) []FlashMessage {
	session := sessions.Default(c)
	flashes := session.Flashes(SessionFlash)
	session.Save()

	messages := make([]FlashMessage, 0)
	for _, f := range flashes {
		if fm, ok := f.(FlashMessage); ok {
			messages = append(messages, fm)
		}
	}

	return messages
}
