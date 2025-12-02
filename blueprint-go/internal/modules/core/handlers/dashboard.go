package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blueprint-go/internal/system/auth"
)

// Dashboard renders the main dashboard page
func Dashboard(c *gin.Context) {
	user := auth.GetUser(c)

	c.HTML(http.StatusOK, "core/dashboard.html", gin.H{
		"title":   "Dashboard",
		"user":    user,
		"flashes": auth.GetFlashes(c),
	})
}

// Settings renders the user settings page
func Settings(c *gin.Context) {
	user := auth.GetUser(c)

	c.HTML(http.StatusOK, "core/settings.html", gin.H{
		"title":   "Settings",
		"user":    user,
		"flashes": auth.GetFlashes(c),
	})
}

// Index redirects to dashboard or login
func Index(c *gin.Context) {
	if auth.IsAuthenticated(c) {
		c.Redirect(http.StatusSeeOther, "/dashboard")
		return
	}
	c.Redirect(http.StatusSeeOther, "/login")
}
