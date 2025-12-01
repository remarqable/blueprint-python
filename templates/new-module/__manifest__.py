"""Module manifest - defines metadata for the module loader."""

from typing import Literal, TypedDict


class ModuleManifest(TypedDict):
    """Type definition for module manifest structure."""

    name: str
    version: str
    main_route: str
    type: Literal["App", "System"]
    depends: list[str]
    icon_class: str
    color: str
    description: str
    long_description: str


# Rename 'modulename' to your actual module name
manifest: ModuleManifest = {
    # Required fields
    "name": "ModuleName",  # Display name (PascalCase)
    "version": "1.0",  # Semantic version
    "main_route": "/modulename",  # URL prefix (lowercase)
    "type": "App",  # "App" (visible) or "System" (hidden)
    "depends": ["core"],  # Module dependencies
    # Display fields
    "icon_class": "fa-solid fa-cube",  # FontAwesome icon class
    "color": "#007bff",  # Theme color (hex)
    "description": "Short description of what this module does",
    "long_description": "Detailed description of the module's features and purpose.",
}
