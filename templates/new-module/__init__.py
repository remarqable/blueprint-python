"""Module initialization - exports the module_instance for the loader."""

from .module import ModuleNameModule

# Required: The module loader looks for this exact attribute
module_instance: ModuleNameModule = ModuleNameModule()

__all__: list[str] = ["module_instance"]
