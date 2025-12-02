package template

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Engine manages template loading and rendering
type Engine struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
	mu        sync.RWMutex
	debug     bool
}

// New creates a new template engine
func New(debug bool) *Engine {
	return &Engine{
		templates: make(map[string]*template.Template),
		funcMap:   DefaultFuncs(),
		debug:     debug,
	}
}

// Load loads all templates from the modules directory
func (e *Engine) Load(modulesPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Find base templates from core module
	baseTemplatePath := filepath.Join(modulesPath, "core", "views", "templates")
	baseFiles, err := filepath.Glob(filepath.Join(baseTemplatePath, "*.html"))
	if err != nil {
		return fmt.Errorf("loading base templates: %w", err)
	}

	if len(baseFiles) == 0 {
		log.Printf("Warning: No base templates found in %s", baseTemplatePath)
	}

	// Scan all modules for templates
	entries, err := os.ReadDir(modulesPath)
	if err != nil {
		return fmt.Errorf("reading modules directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		templatePath := filepath.Join(modulesPath, moduleName, "views", "templates", moduleName)

		// Check if template directory exists
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			continue
		}

		// Load all HTML files in the module's template directory
		files, err := filepath.Glob(filepath.Join(templatePath, "*.html"))
		if err != nil {
			log.Printf("Warning: Error loading templates for %s: %v", moduleName, err)
			continue
		}

		// Also load partials
		partialFiles, _ := filepath.Glob(filepath.Join(templatePath, "partials", "*.html"))
		files = append(files, partialFiles...)

		for _, file := range files {
			// Template name: module/filename (e.g., "core/login.html")
			relPath, _ := filepath.Rel(filepath.Join(modulesPath, moduleName, "views", "templates"), file)
			name := strings.ReplaceAll(relPath, "\\", "/")

			// Combine base templates with module template
			allFiles := append([]string{}, baseFiles...)
			allFiles = append(allFiles, file)

			tmpl, err := template.New(filepath.Base(file)).
				Funcs(e.funcMap).
				ParseFiles(allFiles...)

			if err != nil {
				log.Printf("Warning: Error parsing template %s: %v", name, err)
				continue
			}

			e.templates[name] = tmpl
			log.Printf("Loaded template: %s", name)
		}
	}

	log.Printf("Loaded %d templates", len(e.templates))
	return nil
}

// Render renders a template with the given data
func (e *Engine) Render(w io.Writer, name string, data interface{}) error {
	e.mu.RLock()
	tmpl, ok := e.templates[name]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %s not found", name)
	}

	return tmpl.Execute(w, data)
}

// RenderWithFuncs renders a template with additional template functions
func (e *Engine) RenderWithFuncs(w io.Writer, name string, data interface{}, funcs template.FuncMap) error {
	e.mu.RLock()
	tmpl, ok := e.templates[name]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %s not found", name)
	}

	// Clone and add request-specific functions
	clone, err := tmpl.Clone()
	if err != nil {
		return fmt.Errorf("cloning template: %w", err)
	}

	return clone.Funcs(funcs).Execute(w, data)
}

// Has checks if a template exists
func (e *Engine) Has(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.templates[name]
	return ok
}

// List returns all template names
func (e *Engine) List() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.templates))
	for name := range e.templates {
		names = append(names, name)
	}
	return names
}

// AddFunc adds a template function
func (e *Engine) AddFunc(name string, fn interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.funcMap[name] = fn
}
