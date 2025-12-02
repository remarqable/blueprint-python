package module

// ModuleType indicates whether a module is user-facing or system-level
type ModuleType string

const (
	// ModuleTypeApp is for user-facing modules (visible in app switcher)
	ModuleTypeApp ModuleType = "App"
	// ModuleTypeSystem is for infrastructure modules (hidden from UI)
	ModuleTypeSystem ModuleType = "System"
)

// Manifest contains module metadata
// Equivalent to Python's __manifest__.py
type Manifest struct {
	// Required fields
	Name      string     `json:"name"`       // Display name (e.g., "Dashboard")
	Version   string     `json:"version"`    // Semantic version (e.g., "1.0")
	MainRoute string     `json:"main_route"` // URL prefix (e.g., "/dashboard")
	Type      ModuleType `json:"type"`       // "App" or "System"
	Depends   []string   `json:"depends"`    // Module dependencies (e.g., ["core"])

	// Display fields
	IconClass       string `json:"icon_class"`       // FontAwesome class (e.g., "fa-solid fa-home")
	Color           string `json:"color"`            // Theme color hex (e.g., "#007bff")
	Description     string `json:"description"`      // One-line description
	LongDescription string `json:"long_description"` // Detailed description

	// Runtime fields (set by loader)
	Enabled bool `json:"enabled"` // Whether the module is enabled
}
