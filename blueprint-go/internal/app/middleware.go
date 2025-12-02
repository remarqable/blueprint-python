package app

import (
	"github.com/gin-gonic/gin"

	"blueprint-go/internal/system/auth"
)

// GlobalMiddleware sets up global middleware for the application
func GlobalMiddleware(router *gin.Engine, config *Config) {
	// Recovery middleware
	router.Use(gin.Recovery())

	// Logger middleware (only in debug mode)
	if config.Debug {
		router.Use(gin.Logger())
	}

	// Language detection middleware
	router.Use(LanguageMiddleware(config.DefaultLanguage))

	// Optional auth middleware (loads user if logged in)
	router.Use(auth.OptionalAuth())
}

// LanguageMiddleware detects and sets the current language
func LanguageMiddleware(defaultLang string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Priority: URL param > session > default
		lang := c.Query("lang")

		if lang == "" {
			// Try session
			if sessionLang, ok := c.Get("session_lang"); ok {
				lang = sessionLang.(string)
			}
		}

		if lang == "" {
			lang = defaultLang
		}

		// Store in context
		auth.SetLang(c, lang)
		c.Next()
	}
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}
