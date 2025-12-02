package core

import "blueprint-go/internal/system/module"

// Manifest defines the core module metadata
var Manifest = module.Manifest{
	Name:            "Core",
	Version:         "1.0",
	MainRoute:       "/",
	Type:            module.ModuleTypeSystem,
	Depends:         []string{},
	IconClass:       "fa-solid fa-home",
	Color:           "#6c757d",
	Description:     "Core system module",
	LongDescription: "Provides authentication, user management, and base templates.",
}
