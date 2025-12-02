package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blueprint-go/internal/modules/core/models"
	"blueprint-go/internal/system/auth"
	"blueprint-go/internal/system/db"
)

// LoginPage renders the login page
func LoginPage(c *gin.Context) {
	// If already logged in, redirect to dashboard
	if auth.IsAuthenticated(c) {
		c.Redirect(http.StatusSeeOther, "/dashboard")
		return
	}

	c.HTML(http.StatusOK, "core/login.html", gin.H{
		"title":   "Login",
		"next":    c.Query("next"),
		"flashes": auth.GetFlashes(c),
	})
}

// Login handles the login form submission
func Login(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	next := c.PostForm("next")

	if next == "" {
		next = "/dashboard"
	}

	// Validate input
	if email == "" || password == "" {
		auth.AddFlash(c, auth.FlashError, "Email and password are required")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Find user
	userRepo := models.NewUserRepository(db.Get())
	user, err := userRepo.GetByEmail(email)
	if err != nil {
		auth.AddFlash(c, auth.FlashError, "Invalid email or password")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Check password
	if !user.CheckPassword(password) {
		auth.AddFlash(c, auth.FlashError, "Invalid email or password")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Check if active
	if !user.IsActive {
		auth.AddFlash(c, auth.FlashError, "Account is disabled")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Set session
	if err := auth.SetUserID(c, user.ID); err != nil {
		auth.AddFlash(c, auth.FlashError, "Login failed")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	auth.AddFlash(c, auth.FlashSuccess, "Welcome back, "+user.FirstName+"!")
	c.Redirect(http.StatusSeeOther, next)
}

// Logout handles user logout
func Logout(c *gin.Context) {
	auth.ClearSession(c)
	auth.AddFlash(c, auth.FlashSuccess, "You have been logged out")
	c.Redirect(http.StatusSeeOther, "/login")
}

// RegisterPage renders the registration page
func RegisterPage(c *gin.Context) {
	// If already logged in, redirect to dashboard
	if auth.IsAuthenticated(c) {
		c.Redirect(http.StatusSeeOther, "/dashboard")
		return
	}

	c.HTML(http.StatusOK, "core/register.html", gin.H{
		"title":   "Register",
		"flashes": auth.GetFlashes(c),
	})
}

// Register handles the registration form submission
func Register(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	firstName := c.PostForm("first_name")
	lastName := c.PostForm("last_name")

	// Validate input
	if email == "" || password == "" {
		auth.AddFlash(c, auth.FlashError, "Email and password are required")
		c.Redirect(http.StatusSeeOther, "/register")
		return
	}

	if password != confirmPassword {
		auth.AddFlash(c, auth.FlashError, "Passwords do not match")
		c.Redirect(http.StatusSeeOther, "/register")
		return
	}

	if len(password) < 6 {
		auth.AddFlash(c, auth.FlashError, "Password must be at least 6 characters")
		c.Redirect(http.StatusSeeOther, "/register")
		return
	}

	// Check if user exists
	userRepo := models.NewUserRepository(db.Get())
	if _, err := userRepo.GetByEmail(email); err == nil {
		auth.AddFlash(c, auth.FlashError, "Email already registered")
		c.Redirect(http.StatusSeeOther, "/register")
		return
	}

	// Create user
	user, err := userRepo.Create(email, password, firstName, lastName, false)
	if err != nil {
		auth.AddFlash(c, auth.FlashError, "Registration failed")
		c.Redirect(http.StatusSeeOther, "/register")
		return
	}

	// Log in the user
	if err := auth.SetUserID(c, user.ID); err != nil {
		auth.AddFlash(c, auth.FlashError, "Registration successful, please log in")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	auth.AddFlash(c, auth.FlashSuccess, "Welcome to Blueprint!")
	c.Redirect(http.StatusSeeOther, "/dashboard")
}
