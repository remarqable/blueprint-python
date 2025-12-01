# Module System Pattern Guide

> Complete guide to the Blueprint pluggy-based module system for building extensible applications.

---

## Overview

Blueprint uses [pluggy](https://pluggy.readthedocs.io/) (the plugin system behind pytest) to enable dynamic module loading. Modules are self-contained units that can be added, removed, or disabled without modifying core application code.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     ModuleLoader                            │
│                  (system/module/loader.py)                  │
├─────────────────────────────────────────────────────────────┤
│  1. discover_modules()  - Scan modules/ directory           │
│  2. load_module()       - Import manifest & module_instance │
│  3. register_routes()   - Register Flask blueprints         │
└─────────────────────────────────────────────────────────────┘
                             │
                             │ uses
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    pluggy PluginManager                     │
│                       ("app" project)                       │
├─────────────────────────────────────────────────────────────┤
│  - Registers hook specifications (ModuleSpecs)              │
│  - Registers module instances as plugins                    │
│  - Calls hooks on all registered modules                    │
└─────────────────────────────────────────────────────────────┘
```

---

## Module Lifecycle

### 1. Discovery Phase

The `ModuleLoader.discover_modules()` method:

```python
# Scans modules/ directory
modules_dir = "modules"
module_names = [
    d for d in os.listdir(modules_dir)
    if os.path.isdir(os.path.join(modules_dir, d))
    and not d.startswith("_")
]
```

**Loading Order:**
1. `core` module (required - provides base templates, auth)
2. All other modules (alphabetically) - such as `tasks` (example)

### 2. Loading Phase

For each module, `load_module()`:

1. **Import manifest** from `modules/{name}/__manifest__.py`
2. **Check if disabled** - looks for `__DISABLED__` file
3. **Import module** from `modules/{name}/__init__.py`
4. **Get instance** - expects `module_instance` attribute
5. **Register with pluggy** - makes hooks available

```python
def load_module(self, module_name):
    # Load manifest
    manifest = importlib.import_module(
        f"modules.{module_name}.__manifest__"
    ).manifest

    # Check disabled status
    disabled_file = os.path.join("modules", module_name, "__DISABLED__")
    is_enabled = not os.path.exists(disabled_file)
    manifest["enabled"] = is_enabled

    if is_enabled:
        # Load module and register
        module = importlib.import_module(f"modules.{module_name}")
        if hasattr(module, "module_instance"):
            self.pm.register(module.module_instance)
            self.modules.append(module.module_instance)
```

### 3. Route Registration Phase

After all modules load, `register_routes()` is called:

```python
def register_routes(self, app):
    for module in self.modules:
        if hasattr(module, "get_routes"):
            routes = module.get_routes()
            for blueprint, url_prefix in routes:
                app.register_blueprint(blueprint, url_prefix=url_prefix)
```

### 4. Database Initialization Phase

After routes are registered and `db.create_all()` runs:

```python
# In app.py
module_loader.pm.hook.init_database()
```

This calls `init_database()` on every module that implements it.

---

## Creating a Module

### Required Files

```
modules/yourmodule/
├── __init__.py          # REQUIRED: Exports module_instance
├── __manifest__.py      # REQUIRED: Module metadata
├── module.py            # REQUIRED: Module class
├── controllers/
│   └── routes.py        # REQUIRED: Flask blueprint
├── models/              # Optional: SQLAlchemy models
├── views/
│   └── templates/       # Optional: Jinja2 templates
└── lang/                # Optional: i18n JSON files
```

### __manifest__.py

```python
manifest = {
    # Required fields
    "name": "YourModule",         # Display name (used as dict key)
    "version": "1.0",             # Semantic version
    "main_route": "/yourmodule",  # URL prefix for route matching
    "type": "App",                # "App" or "System"
    "depends": ["core"],          # Module dependencies

    # Display fields
    "icon_class": "fa-solid fa-cube",   # FontAwesome icon
    "color": "#007bff",                  # Theme color (hex)
    "description": "Short description",  # One line
    "long_description": "...",           # Detailed (optional)
}
```

**Type Values:**
| Type | Description | App Switcher |
|------|-------------|--------------|
| `"App"` | User-facing feature | Visible |
| `"System"` | Infrastructure/admin | Hidden |

### __init__.py

```python
from .module import YourModuleModule

# This is what ModuleLoader looks for
module_instance = YourModuleModule()

__all__ = ["module_instance"]
```

**Important:** The `module_instance` variable name is required. The loader checks for this exact attribute.

### module.py

```python
from system.db.database import db
from system.module.hooks import hookimpl


class YourModuleModule:
    """Module class implementing lifecycle hooks."""

    def get_routes(self):
        """Return list of (blueprint, url_prefix) tuples.

        Called by ModuleLoader.register_routes() during app startup.
        """
        from .controllers.routes import blueprint
        return [(blueprint, "/yourmodule")]

    @hookimpl
    def init_database(self):
        """Initialize database tables and sample data.

        Called after all modules are loaded and db.create_all() runs.
        """
        db.create_all()
        # Optional: Create sample data
        from .models.yourmodel import YourModel
        YourModel.create_sample_data()
```

---

## Hook System

### Hook Specifications

Defined in `system/module/hooks.py`:

```python
import pluggy

hookspec = pluggy.HookspecMarker("app")
hookimpl = pluggy.HookimplMarker("app")


class ModuleSpecs:
    @hookspec
    def init_database(self):
        """Initialize database tables and sample data."""
        pass
```

### Implementing Hooks

Use the `@hookimpl` decorator:

```python
from system.module.hooks import hookimpl

class MyModule:
    @hookimpl
    def init_database(self):
        """This will be called during app startup."""
        db.create_all()
```

### Available Hooks

| Hook | When Called | Purpose |
|------|-------------|---------|
| `init_database()` | After `db.create_all()` | Create tables, seed data |

### Adding Custom Hooks

Modules can define their own hook specifications. Example:

```python
# modules/tasks/hooks.py
class TaskHookSpecs:
    @hookspec
    def task_created(self, task):
        """Called when a new task is created."""
        pass

# In TaskModule
def register_specs(self, plugin_manager):
    plugin_manager.add_hookspecs(TaskHookSpecs)
```

---

## Disabling Modules

To disable a module without deleting it:

```bash
# Disable
touch modules/yourmodule/__DISABLED__

# Re-enable
rm modules/yourmodule/__DISABLED__
```

Disabled modules:
- Are still tracked in manifests (for UI display)
- Have `enabled: false` in their manifest
- Don't have their code imported
- Don't register routes or hooks

---

## Module Context

During requests, module information is available via Flask's `g` object:

```python
from flask import g

# In before_request hook (set by app.py):
g.installed_modules  # List of all module manifests
g.current_module     # Current module's manifest (based on URL)
```

### Determining Current Module

The current module is determined by matching the URL path:

```python
path = request.path.split("/")[1] or "core"
current_module = next(
    (m for m in g.installed_modules
     if m.get("main_route", "").strip("/").lower() == path.lower()),
    # Fallback to core
    next((m for m in g.installed_modules if m.get("name") == "Core"), None)
)
```

---

## Best Practices

### 1. Keep Modules Independent

- Only depend on `core` unless necessary
- Don't import from other modules directly
- Use hooks for inter-module communication

### 2. Consistent Structure

Follow the standard directory layout:

```
yourmodule/
├── __init__.py          # Only: module_instance = YourModule()
├── __manifest__.py      # Only: manifest = {...}
├── module.py            # Module class with hooks
├── controllers/
│   ├── __init__.py
│   └── routes.py        # Blueprint definition
├── models/
│   ├── __init__.py
│   └── *.py             # @ModelRegistry.register models
├── views/
│   ├── templates/yourmodule/
│   │   ├── index.html
│   │   └── partials/
│   │       └── _*.html
│   └── assets/
│       └── css/
└── lang/
    ├── en.json
    └── es.json
```

### 3. Route Prefix Convention

Define the URL prefix **only** in `get_routes()`:

```python
# CORRECT - single source of truth
def get_routes(self):
    return [(blueprint, "/yourmodule")]

# WRONG - don't register blueprint in __init__.py
# module_instance.register_blueprint(bp, "/yourmodule")  # Don't do this
```

### 4. Sample Data Pattern

```python
@classmethod
def create_sample_data(cls):
    """Idempotent sample data creation."""
    if not cls.query.first():  # Only if table is empty
        cls.create("Sample 1")
        cls.create("Sample 2")
```

### 5. Error Handling

Wrap sample data creation in try/except:

```python
@hookimpl
def init_database(self):
    db.create_all()
    try:
        MyModel.create_sample_data()
    except Exception as e:
        import logging
        logging.getLogger(__name__).error(f"Sample data error: {e}")
```

---

## Troubleshooting

### Module Not Loading

1. Check `module_instance` is exported from `__init__.py`
2. Check for Python import errors (run `python -c "import modules.yourmodule"`)
3. Check manifest is valid Python dict
4. Look for `__DISABLED__` file

### Routes Not Working

1. Verify `get_routes()` returns correct format: `[(blueprint, "/prefix")]`
2. Check blueprint is properly defined
3. Verify URL prefix matches `main_route` in manifest

### Hooks Not Called

1. Verify `@hookimpl` decorator is imported from correct location
2. Ensure method signature matches hook specification
3. Check module is registered with pluggy (no load errors)

### Console Output

On startup, the loader prints a module registry:

```
Database Model Registry:
--------   ----------   --------------------
Module     Model        Table
--------   ----------   --------------------
core       User         user
core       Group        group
tasks      Task         task
```

Check this output to verify modules loaded correctly.

---

## Example: Complete Module

Here's a complete minimal module:

```python
# modules/notes/__manifest__.py
manifest = {
    "name": "Notes",
    "version": "1.0",
    "main_route": "/notes",
    "icon_class": "fa-solid fa-sticky-note",
    "type": "App",
    "color": "#ffc107",
    "depends": ["core"],
    "description": "Simple note taking",
}
```

```python
# modules/notes/__init__.py
from .module import NotesModule
module_instance = NotesModule()
__all__ = ["module_instance"]
```

```python
# modules/notes/module.py
from system.db.database import db
from system.module.hooks import hookimpl

class NotesModule:
    def get_routes(self):
        from .controllers.routes import blueprint
        return [(blueprint, "/notes")]

    @hookimpl
    def init_database(self):
        db.create_all()
        from .models.note import Note
        Note.create_sample_data()
```

```python
# modules/notes/models/note.py
from system.db.database import db
from system.db.decorators import ModelRegistry

@ModelRegistry.register
class Note(db.Model):
    __tablename__ = "note"
    id = db.Column(db.Integer, primary_key=True)
    title = db.Column(db.String(255), nullable=False)
    content = db.Column(db.Text)

    @classmethod
    def create_sample_data(cls):
        if not cls.query.first():
            db.session.add(cls(title="Welcome", content="Your first note"))
            db.session.commit()
```

```python
# modules/notes/controllers/routes.py
from flask import Blueprint, render_template
from flask_login import login_required
from ..models.note import Note

blueprint = Blueprint(
    "notes_bp", __name__,
    template_folder="../views/templates",
    static_folder="../views/assets",
)

@blueprint.route("/")
@login_required
def index():
    return render_template("notes/index.html", notes=Note.query.all())
```

```html
<!-- modules/notes/views/templates/notes/index.html -->
{% extends "base.html" %}
{% block content %}
<div class="container mt-4">
    <h1>{{ _("Notes") }}</h1>
    {% for note in notes %}
    <div class="card mb-2">
        <div class="card-body">
            <h5>{{ note.title }}</h5>
            <p>{{ note.content }}</p>
        </div>
    </div>
    {% endfor %}
</div>
{% endblock %}
```
