package module

import (
	"fmt"
	"log"
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Loader handles module discovery and initialization
// Equivalent to Python's ModuleLoader class
type Loader struct {
	modules   []Module
	manifests []Manifest
	router    *gin.Engine
	db        *gorm.DB
	errors    []string
}

// NewLoader creates a new module loader
func NewLoader(router *gin.Engine, db *gorm.DB) *Loader {
	return &Loader{
		modules:   make([]Module, 0),
		manifests: make([]Manifest, 0),
		router:    router,
		db:        db,
		errors:    make([]string, 0),
	}
}

// RegisterModule adds a module to the loader
// In Go, modules are explicitly registered (unlike Python's dynamic discovery)
func (l *Loader) RegisterModule(m Module) {
	manifest := m.Manifest()
	manifest.Enabled = true

	l.modules = append(l.modules, m)
	l.manifests = append(l.manifests, manifest)
}

// LoadAll sorts and registers all modules
// Loading order: Core first, then alphabetically
func (l *Loader) LoadAll() error {
	// Sort: core first, then alphabetically
	sort.Slice(l.modules, func(i, j int) bool {
		mi, mj := l.modules[i].Manifest(), l.modules[j].Manifest()
		if mi.Name == "Core" {
			return true
		}
		if mj.Name == "Core" {
			return false
		}
		return mi.Name < mj.Name
	})

	// Also sort manifests to match
	sort.Slice(l.manifests, func(i, j int) bool {
		if l.manifests[i].Name == "Core" {
			return true
		}
		if l.manifests[j].Name == "Core" {
			return false
		}
		return l.manifests[i].Name < l.manifests[j].Name
	})

	// Register routes for each module
	for _, m := range l.modules {
		manifest := m.Manifest()
		if !manifest.Enabled {
			continue
		}

		// Create router group with module's URL prefix
		group := l.router.Group(manifest.MainRoute)
		m.RegisterRoutes(group)

		log.Printf("Module loaded: %s (%s)", manifest.Name, manifest.MainRoute)
	}

	return nil
}

// InitDatabases calls InitDatabase on modules that implement DatabaseInitializer
func (l *Loader) InitDatabases() error {
	for _, m := range l.modules {
		manifest := m.Manifest()
		if !manifest.Enabled {
			continue
		}

		if di, ok := m.(DatabaseInitializer); ok {
			if err := di.InitDatabase(l.db); err != nil {
				errMsg := fmt.Sprintf("module %s: %v", manifest.Name, err)
				l.errors = append(l.errors, errMsg)
				log.Printf("ERROR: %s", errMsg)
				// Continue with other modules instead of failing
			}
		}
	}

	if len(l.errors) > 0 {
		log.Printf("WARNING: %d module(s) had initialization errors", len(l.errors))
	}

	return nil
}

// GetManifests returns all module manifests (for templates/UI)
func (l *Loader) GetManifests() []Manifest {
	return l.manifests
}

// GetEnabledManifests returns only enabled module manifests
func (l *Loader) GetEnabledManifests() []Manifest {
	var enabled []Manifest
	for _, m := range l.manifests {
		if m.Enabled {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// GetAppModules returns manifests of type "App" (for UI display)
func (l *Loader) GetAppModules() []Manifest {
	var apps []Manifest
	for _, m := range l.manifests {
		if m.Enabled && m.Type == ModuleTypeApp {
			apps = append(apps, m)
		}
	}
	return apps
}

// GetErrors returns any loading errors
func (l *Loader) GetErrors() []string {
	return l.errors
}

// PrintStatus prints the module loading status table
func (l *Loader) PrintStatus() {
	fmt.Println("\nLoading modules:")
	fmt.Println("Module        Type     Status")
	fmt.Println("----------    ------   -------")

	for _, m := range l.manifests {
		status := "Enabled"
		if !m.Enabled {
			status = "Disabled"
		}
		fmt.Printf("%-13s %-8s %s\n", m.Name, m.Type, status)
	}

	if len(l.errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range l.errors {
			fmt.Printf("  ERROR: %s\n", err)
		}
	}
	fmt.Println()
}
