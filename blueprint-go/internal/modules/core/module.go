package core

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"blueprint-go/internal/modules/core/handlers"
	"blueprint-go/internal/modules/core/models"
	"blueprint-go/internal/system/auth"
	"blueprint-go/internal/system/module"
)

// CoreModule implements the Module interface for the core system
type CoreModule struct{}

// Manifest returns the module's metadata
func (m *CoreModule) Manifest() module.Manifest {
	return Manifest
}

// RegisterRoutes registers the core module's routes
func (m *CoreModule) RegisterRoutes(r *gin.RouterGroup) {
	// Public routes (no auth required)
	r.GET("/", handlers.Index)
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.Login)
	r.GET("/logout", handlers.Logout)
	r.GET("/register", handlers.RegisterPage)
	r.POST("/register", handlers.Register)

	// Protected routes (auth required)
	protected := r.Group("/")
	protected.Use(auth.RequireLogin())
	{
		protected.GET("/dashboard", handlers.Dashboard)
		protected.GET("/settings", handlers.Settings)
	}

	// Admin routes
	admin := r.Group("/admin")
	admin.Use(auth.RequireLogin(), auth.RequireAdmin())
	{
		// Admin routes will be added here
	}
}

// InitDatabase initializes the core module's database tables and sample data
func (m *CoreModule) InitDatabase(db *gorm.DB) error {
	// Auto-migrate core models
	err := db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.UserSetting{},
	)
	if err != nil {
		return err
	}

	// Create system groups
	groupRepo := models.NewGroupRepository(db)
	if err := groupRepo.CreateSystemGroups(); err != nil {
		return err
	}

	// Create sample data
	userRepo := models.NewUserRepository(db)
	return userRepo.CreateSampleData()
}

// Module is the singleton instance of CoreModule
var Module = &CoreModule{}
