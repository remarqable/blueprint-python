package modulename

// Manifest contains module metadata for the loader
var Manifest = ModuleManifest{
	Name:            "ModuleName",
	Version:         "1.0.0",
	MainRoute:       "/modulename",
	Type:            "App", // or "System"
	Depends:         []string{"core"},
	IconClass:       "fa-solid fa-cube",
	Color:           "#007bff",
	Description:     "Short description of your module",
	LongDescription: "Detailed description of what this module does and its features.",
}

// ModuleManifest defines the structure for module metadata
type ModuleManifest struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	MainRoute       string   `json:"main_route"`
	Type            string   `json:"type"` // "App" or "System"
	Depends         []string `json:"depends"`
	IconClass       string   `json:"icon_class"`
	Color           string   `json:"color"`
	Description     string   `json:"description"`
	LongDescription string   `json:"long_description"`
}
