package module

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Module defines the interface that all modules must implement
// This is the Go equivalent of Python's module_instance pattern
type Module interface {
	// Manifest returns the module's metadata
	// Equivalent to Python's __manifest__.py
	Manifest() Manifest

	// RegisterRoutes registers the module's routes on the router group
	// The router group is already scoped to the module's URL prefix
	// Equivalent to Python's get_routes() returning Flask blueprints
	RegisterRoutes(r *gin.RouterGroup)
}

// DatabaseInitializer is an optional interface for modules that need
// to initialize database tables or seed data
// Equivalent to Python's @hookimpl init_database
type DatabaseInitializer interface {
	// InitDatabase is called after all modules are loaded
	// and the database connection is established
	// Use this for:
	// - Running auto-migrations
	// - Seeding sample data
	// - Creating default records
	InitDatabase(db *gorm.DB) error
}

// TemplateProvider is an optional interface for modules that provide
// custom templates
type TemplateProvider interface {
	// TemplatePath returns the path to the module's templates directory
	TemplatePath() string
}

// TranslationProvider is an optional interface for modules that provide
// translations
type TranslationProvider interface {
	// TranslationPath returns the path to the module's lang directory
	TranslationPath() string
}
