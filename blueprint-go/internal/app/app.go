package app

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"blueprint-go/internal/system/auth"
	"blueprint-go/internal/system/db"
	"blueprint-go/internal/system/i18n"
	"blueprint-go/internal/system/module"
	"blueprint-go/internal/system/template"
)

// App represents the application instance
type App struct {
	Config       *Config
	Router       *gin.Engine
	ModuleLoader *module.Loader
	Templates    *template.Engine
}

// New creates a new application instance
func New(config *Config) (*App, error) {
	// Set Gin mode
	if config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.Default()

	// Initialize database
	if err := db.Initialize(config.DatabaseURL, config.Debug); err != nil {
		return nil, fmt.Errorf("database initialization: %w", err)
	}

	// Setup sessions
	auth.SetupSessions(router, config.SecretKey)

	// Create template engine
	templates := template.New(config.Debug)

	// Create module loader
	loader := module.NewLoader(router, db.Get())

	app := &App{
		Config:       config,
		Router:       router,
		ModuleLoader: loader,
		Templates:    templates,
	}

	return app, nil
}

// RegisterModule registers a module with the application
func (a *App) RegisterModule(m module.Module) {
	a.ModuleLoader.RegisterModule(m)
}

// Initialize initializes all modules and loads resources
func (a *App) Initialize(modulesPath string) error {
	// Load all modules
	if err := a.ModuleLoader.LoadAll(); err != nil {
		return fmt.Errorf("loading modules: %w", err)
	}

	// Print module status
	a.ModuleLoader.PrintStatus()

	// Initialize databases for all modules
	if err := a.ModuleLoader.InitDatabases(); err != nil {
		return fmt.Errorf("initializing databases: %w", err)
	}

	// Print model registry
	db.PrintSummary()

	// Load templates
	if err := a.Templates.Load(modulesPath); err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	// Load translations
	if err := i18n.Load(modulesPath); err != nil {
		log.Printf("Warning: Could not load translations: %v", err)
	}

	// Setup HTML rendering with Gin
	a.setupHTMLRendering()

	return nil
}

// setupHTMLRendering configures Gin to use our template engine
func (a *App) setupHTMLRendering() {
	// For each template, register with Gin's HTML renderer
	// This is a simplified approach - in production you might want a custom renderer
	a.Router.SetFuncMap(template.DefaultFuncs())

	// Load templates for Gin (it will re-parse, but this ensures compatibility)
	a.Router.LoadHTMLGlob("internal/modules/*/views/templates/*/*.html")
}

// Run starts the application server
func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.Config.Port)
	log.Printf("Starting server on %s", addr)
	return a.Router.Run(addr)
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown() error {
	return db.Close()
}
