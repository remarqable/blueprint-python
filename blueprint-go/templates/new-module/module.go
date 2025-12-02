package modulename

import (
	"yourapp/internal/modules/modulename/handlers"
	"yourapp/internal/modules/modulename/models"
	"yourapp/internal/system/db"
	"yourapp/internal/system/module"

	"gorm.io/gorm"
)

// Ensure Module implements required interfaces
var _ module.Module = (*Module)(nil)
var _ module.DatabaseInitializer = (*Module)(nil)

// Module implements the module interface for modulename
type Module struct {
	db      *gorm.DB
	handler *handlers.Handler
}

// New creates a new module instance
func New(database *gorm.DB) *Module {
	repo := models.NewItemRepository(database)
	return &Module{
		db:      database,
		handler: handlers.NewHandler(repo),
	}
}

// GetRoutes returns the routes for this module
func (m *Module) GetRoutes() []module.Route {
	return []module.Route{
		{
			Handler: m.handler.RegisterRoutes,
			Prefix:  "/modulename",
		},
	}
}

// InitDatabase initializes database tables and sample data
func (m *Module) InitDatabase() error {
	// Auto-migrate models
	if err := m.db.AutoMigrate(&models.Item{}); err != nil {
		return err
	}

	// Create sample data (idempotent)
	repo := models.NewItemRepository(m.db)
	return repo.CreateSampleData()
}

// GetManifest returns the module manifest
func (m *Module) GetManifest() ModuleManifest {
	return Manifest
}
