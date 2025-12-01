# CLAUDE.md - Modular Flask Blueprint

> **Master guide for building modular Flask applications with dynamic plugin architecture.**
> Single-file blueprint combining module system, MVC patterns, database setup, i18n, and deployment.
> Optimized for **extensibility**, **clean code**, and **AI agent execution**.

---

## 🚀 QUICK START (New Module Bootstrap)

**👉 AI Agents: Execute this section to create a new module (10 minutes)**

### Prerequisites
- [ ] Existing Blueprint application running
- [ ] Understanding of the module you want to create

### Bootstrap Sequence (Follow in Order)

**Step 1: Create Module Directory** (1 min)
```bash
MODULE_NAME="yourmodule"  # lowercase, no spaces
mkdir -p modules/$MODULE_NAME/{controllers,models,views/{templates/$MODULE_NAME,assets/css},lang,utils}
touch modules/$MODULE_NAME/__init__.py
touch modules/$MODULE_NAME/__manifest__.py
touch modules/$MODULE_NAME/module.py
touch modules/$MODULE_NAME/models/__init__.py
touch modules/$MODULE_NAME/controllers/__init__.py
touch modules/$MODULE_NAME/controllers/routes.py
```
✅ **Verify:** `ls modules/$MODULE_NAME` shows folder structure

**Step 2: Create Manifest** (1 min)
```python
# modules/yourmodule/__manifest__.py
manifest = {
    "name": "YourModule",
    "version": "1.0",
    "main_route": "/yourmodule",
    "icon_class": "fa-solid fa-cube",
    "type": "App",  # or "System"
    "color": "#007bff",
    "depends": ["core"],
    "description": "Short description of your module",
    "long_description": "Detailed description of what this module does.",
}
```

**Step 3: Create Module Class** (2 min)
```python
# modules/yourmodule/module.py
from system.db.database import db
from system.module.hooks import hookimpl


class YourModuleModule:
    def get_routes(self):
        """Return list of (blueprint, url_prefix) tuples"""
        from .controllers.routes import blueprint
        return [(blueprint, "/yourmodule")]

    @hookimpl
    def init_database(self):
        """Initialize database tables and sample data"""
        db.create_all()
        # Optional: Create sample data
        # from .models.yourmodel import YourModel
        # YourModel.create_sample_data()
```

**Step 4: Create Module Instance** (1 min)
```python
# modules/yourmodule/__init__.py
from .module import YourModuleModule

module_instance = YourModuleModule()

__all__ = ["module_instance"]
```

**Step 5: Create Routes** (3 min)
```python
# modules/yourmodule/controllers/routes.py
from flask import Blueprint, render_template
from flask_login import login_required

blueprint = Blueprint(
    "yourmodule_bp",
    __name__,
    template_folder="../views/templates",
    static_folder="../views/assets",
)

@blueprint.route("/")
@login_required
def index():
    return render_template("yourmodule/index.html")
```

**Step 6: Create Template** (2 min)
```html
<!-- modules/yourmodule/views/templates/yourmodule/index.html -->
{% extends "base.html" %}

{% block content %}
<div class="container mt-4">
    <h1>{{ _("Your Module") }}</h1>
    <p>{{ _("Welcome to your new module!") }}</p>
</div>
{% endblock %}
```

**Step 7: Run & Verify**
```bash
python app.py
```
✅ **Verify:** Navigate to http://localhost:8000/yourmodule

**Expected Time: 10 minutes to working module**

---

## 📋 TABLE OF CONTENTS

### Foundation (Read Once)
- [Philosophy](#philosophy) - Core principles
- [Architecture Overview](#architecture-overview) - System design
- [Folder Structure](#folder-structure) - Project layout
- [Tech Stack](#tech-stack) - Dependencies & rationale

### Module System
- [Module System](#module-system) → [patterns/module-system.md](patterns/module-system.md)
- [Manifest Reference](#manifest-reference) - Module metadata
- [Hook System](#hook-system) - Lifecycle hooks

### MVC Pattern (Module-Scoped)
- [Models](#models) - Fat models with ModelRegistry
- [Views](#views) - Templates with inheritance
- [Controllers](#controllers) - Thin Flask blueprints

### Platform Components
- [Database](#database) → [patterns/database.md](patterns/database.md)
- [Authentication](#authentication) → [patterns/auth.md](patterns/auth.md)
- [Internationalization](#internationalization) → [patterns/i18n.md](patterns/i18n.md)
- [Frontend](#frontend) → [patterns/frontend.md](patterns/frontend.md)

### Operations
- [Testing](#testing) → [patterns/testing.md](patterns/testing.md)
- [Deployment](#deployment) → [patterns/deployment.md](patterns/deployment.md)

---

## Philosophy

### Core Principles

- **Modular by design.** Features are self-contained modules that can be enabled/disabled.
- **Plugin architecture.** Modules extend functionality via hooks without modifying core.
- **Fat Models, Thin Controllers.** Business logic lives in models.
- **Server-rendered HTML + Alpine.js + HTMX.** No SPA complexity.
- **i18n from day 1.** Module-scoped translations with fallback to core.
- **Convention over configuration.** Standard patterns reduce decisions.

### Module Philosophy

Each module is a **self-contained unit** with:
- Its own models, controllers, views, and translations
- A manifest declaring metadata and dependencies
- Hooks to integrate with the system lifecycle
- Independence from other modules (except declared dependencies)

### Single-Tenant Architecture

This blueprint is designed for **single-tenant applications** where:

- One database serves one organization/deployment
- All users belong to the same implicit organization
- Access control via groups (ADMIN, USER, etc.) within that organization
- No data isolation between tenants required

**When to Use:**
- Internal tools (company-specific applications)
- Department-specific apps
- Single organization SaaS (one deployment per customer)
- Self-hosted applications

**Multi-Tenant Extension:**

To extend for multi-tenancy, you would need to:
1. Add `tenant_id` column to all models
2. Implement tenant isolation middleware
3. Filter all queries by current tenant
4. Separate tenant data at DB or schema level
5. Add tenant switching UI

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Flask Application                        │
│                      (app.py)                               │
└────────────────────────────┬────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   ▼                   ▼
┌─────────────────┐  ┌───────────────┐  ┌────────────────┐
│  Module Loader  │  │   Database    │  │   i18n System  │
│    (pluggy)     │  │  (SQLAlchemy) │  │   (JSON)       │
└────────┬────────┘  └───────────────┘  └────────────────┘
         │
         │ discovers & loads
         ▼
┌─────────────────────────────────────────────────────────────┐
│                        modules/                             │
├─────────────┬─────────────┬──────────────────────────────────┤
│    core     │   tasks     │        [yourmodule]             │
│  (System)   │   (App)     │           (App)                 │
│  required   │  optional   │     your modules here           │
└─────────────┴─────────────┴──────────────────────────────────┘
```

### Loading Order
1. **Core module** (required) - Minimal: base templates, auth infrastructure
2. **All other modules** (alphabetically) - Your application modules

---

## Folder Structure

```
myapp/
├── app.py                       # Application factory & entry point
├── app.db                       # SQLite database
├── requirements.txt             # Python dependencies
│
├── system/                      # Core framework (don't modify often)
│   ├── module/
│   │   ├── loader.py            # Module discovery & loading
│   │   ├── hooks.py             # Hook specifications (pluggy)
│   │   └── utils.py             # Module utilities
│   ├── db/
│   │   ├── database.py          # SQLAlchemy instance
│   │   └── decorators.py        # @ModelRegistry.register
│   ├── i18n/
│   │   └── translation.py       # Translation functions
│   ├── auth/
│   │   └── decorators.py        # @admin_required, etc.
│   └── utils/
│       └── responses.py         # HTMX helpers, API responses
│
├── modules/                     # Dynamic modules (main development here)
│   ├── core/                    # System module (required)
│   │   ├── __init__.py
│   │   ├── __manifest__.py
│   │   ├── module.py
│   │   ├── models/
│   │   │   ├── user.py
│   │   │   ├── group.py
│   │   │   └── user_setting.py
│   │   ├── controllers/
│   │   │   └── routes.py
│   │   ├── views/
│   │   │   ├── templates/
│   │   │   │   ├── base.html    # Main layout (all modules inherit)
│   │   │   │   ├── header.html
│   │   │   │   └── ...
│   │   │   └── assets/
│   │   │       └── css/
│   │   └── lang/
│   │       ├── en.json
│   │       └── es.json
│   │
│   ├── tasks/                   # Example app module (optional)
│   │   └── ... (same structure)
│   │
│   └── [yourmodule]/            # Your modules here
│       ├── __init__.py          # module_instance = YourModule()
│       ├── __manifest__.py      # Module metadata
│       ├── module.py            # Module class with hooks
│       ├── __DISABLED__         # (optional) Disables module
│       ├── models/
│       ├── controllers/
│       ├── views/
│       │   ├── templates/[yourmodule]/
│       │   └── assets/css/
│       └── lang/
│
├── docs/                        # Documentation
└── tests/                       # Tests
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Backend** | Python 3.11+ | Type hints, performance |
| **Web Framework** | Flask 3.x | Simple, mature, Jinja2 built-in |
| **Plugin System** | pluggy 1.5 | pytest's battle-tested plugin manager |
| **Database** | SQLite (default) | No setup, easy development |
| **ORM** | SQLAlchemy 2.0 | Declarative models, excellent support |
| **Templates** | Jinja2 | Auto-escaping, fast, Flask-native |
| **Frontend JS** | Alpine.js | Reactive components, minimal footprint |
| **Frontend AJAX** | HTMX | Server-driven UI updates |
| **CSS** | Bootstrap 5 | No build step, comprehensive |
| **Icons** | FontAwesome 6 | Wide icon selection |
| **i18n** | JSON catalogs | Simple, module-scoped |
| **Auth** | Flask-Login | Session-based, extensible |
| **Real-time** | Flask-SocketIO | WebSocket support |
| **Testing** | pytest | Transaction rollback pattern |

---

## Module System

→ **See complete guide:** [patterns/module-system.md](patterns/module-system.md)

### Manifest Reference

Every module requires a `__manifest__.py`:

```python
manifest = {
    # Required
    "name": "Tasks",              # Display name
    "version": "1.0",             # Semantic version
    "main_route": "/tasks",       # URL prefix
    "type": "App",                # "App" (visible) or "System" (hidden)
    "depends": ["core"],          # Module dependencies

    # Display
    "icon_class": "fa-solid fa-check",  # FontAwesome class
    "color": "#007bff",                  # Theme color (CSS)
    "description": "Short desc",         # One line
    "long_description": "...",           # Detailed description
}
```

**Type Values:**
- `"App"` - Shows in app switcher, user-facing
- `"System"` - Hidden from app switcher, infrastructure

### Hook System

Modules implement hooks via the `@hookimpl` decorator:

```python
from system.module.hooks import hookimpl

class MyModule:
    @hookimpl
    def init_database(self):
        """Called after all modules load, DB connection established"""
        db.create_all()
        MyModel.create_sample_data()
```

**Available Hooks:**
- `init_database()` - Initialize tables and sample data

---

## Models

Models use the `@ModelRegistry.register` decorator for tracking:

```python
# modules/yourmodule/models/item.py
from system.db.database import db
from system.db.decorators import ModelRegistry


@ModelRegistry.register
class Item(db.Model):
    __tablename__ = "item"

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(255), nullable=False)
    created_at = db.Column(db.DateTime, default=db.func.current_timestamp())

    # --- Class Methods (CRUD) ---
    @classmethod
    def create(cls, name):
        item = cls(name=name)
        db.session.add(item)
        db.session.commit()
        return item

    @classmethod
    def get_all(cls):
        return cls.query.all()

    @classmethod
    def get_by_id(cls, item_id):
        return cls.query.get(item_id)

    @classmethod
    def create_sample_data(cls):
        """Called from init_database hook"""
        if not cls.query.first():
            cls.create("Sample Item 1")
            cls.create("Sample Item 2")
```

→ **See complete guide:** [patterns/database.md](patterns/database.md)

---

## Controllers

Controllers are thin Flask blueprints that parse input and call models:

```python
# modules/yourmodule/controllers/routes.py
from flask import Blueprint, render_template, request, redirect, url_for, flash
from flask_login import login_required

from ..models.item import Item

blueprint = Blueprint(
    "yourmodule_bp",
    __name__,
    template_folder="../views/templates",
    static_folder="../views/assets",
)


@blueprint.route("/")
@login_required
def index():
    """List all items"""
    items = Item.get_all()
    return render_template("yourmodule/index.html", items=items)


@blueprint.route("/add", methods=["POST"])
@login_required
def add():
    """Create new item"""
    name = request.form.get("name")
    if name:
        Item.create(name)
        flash("Item created", "success")
    return redirect(url_for("yourmodule_bp.index"))
```

---

## Views

Templates extend `base.html` from core module:

```html
<!-- modules/yourmodule/views/templates/yourmodule/index.html -->
{% extends "base.html" %}

{% block content %}
<div class="container mt-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
        <h1>{{ _("Items") }}</h1>
        <button class="btn btn-primary"
                hx-get="{{ url_for('yourmodule_bp.add_modal') }}"
                hx-target="#modals">
            {{ _("Add Item") }}
        </button>
    </div>

    <div class="list-group">
        {% for item in items %}
        <div class="list-group-item">{{ item.name }}</div>
        {% endfor %}
    </div>
</div>

<div id="modals"></div>
{% endblock %}
```

### Template Organization

```
views/templates/yourmodule/
├── index.html              # Main page
├── detail.html             # Detail view
└── partials/               # HTMX fragments
    ├── _list.html          # Item list partial
    ├── _form.html          # Form partial
    └── _modal.html         # Modal partial
```

**Conventions:**
- Partials prefixed with `_`
- Partials in `partials/` subfolder
- Use `{{ _("text") }}` for all user-visible strings

---

## Frontend

**Alpine.js** for reactive components:
```html
<div x-data="{ count: 0 }">
    <button @click="count++">Clicked <span x-text="count"></span></button>
</div>
```

**HTMX** for server-driven updates:
```html
<button hx-get="/items/list"
        hx-target="#list-container"
        hx-swap="innerHTML">
    Refresh
</button>
```

→ **See complete guide:** [patterns/frontend.md](patterns/frontend.md)

---

## Internationalization

Translations are module-scoped JSON files:

```json
// modules/yourmodule/lang/en.json
{
    "Items": "Items",
    "Add Item": "Add Item",
    "Item created": "Item created"
}
```

```json
// modules/yourmodule/lang/es.json
{
    "Items": "Artículos",
    "Add Item": "Agregar Artículo",
    "Item created": "Artículo creado"
}
```

**Usage in templates:**
```html
{{ _("Items") }}
```

**Fallback Chain:**
1. Module-specific translation
2. Core module translation
3. Original text

→ **See complete guide:** [patterns/i18n.md](patterns/i18n.md)

---

## Authentication

Uses Flask-Login with group-based access control:

```python
from flask_login import login_required, current_user
from system.auth.decorators import admin_required

@blueprint.route("/admin")
@login_required
@admin_required
def admin_panel():
    return render_template("admin.html")
```

**Built-in Groups:**
- `ALL` - All authenticated users
- `ADMIN` - Administrators

→ **See complete guide:** [patterns/auth.md](patterns/auth.md)

---

## Disabling Modules

To disable a module without deleting it:
```bash
touch modules/yourmodule/__DISABLED__
```

To re-enable:
```bash
rm modules/yourmodule/__DISABLED__
```

---

## Testing

```python
# tests/conftest.py
import pytest
from app import create_app
from system.db.database import db

@pytest.fixture
def app():
    app = create_app()
    app.config['SQLALCHEMY_DATABASE_URI'] = 'sqlite:///:memory:'
    app.config['TESTING'] = True

    with app.app_context():
        db.create_all()
        yield app
        db.drop_all()

@pytest.fixture
def client(app):
    return app.test_client()
```

→ **See complete guide:** [patterns/testing.md](patterns/testing.md)

---

## Type Safety

Blueprint uses mypy for static type checking to catch errors early and improve code maintainability.

### Running Type Checks

```bash
# Basic type checking (recommended - should always pass!)
mypy app.py modules/ system/ --exclude blueprint

# Strict mode (aspirational, currently has ~200 errors)
mypy --strict app.py modules/ system/ --exclude blueprint
```

### Current Status

- **Basic mypy**: ✅ **0 errors - fully passing!**
- **Strict mode**: ~200 errors (Flask/SQLAlchemy framework limitations)

### Development Workflow

**Before committing code:**
1. Run `mypy app.py modules/ system/ --exclude blueprint`
2. Ensure it passes with 0 errors
3. Fix any new type errors introduced by your changes

**The codebase is now fully type-checked and should remain that way!**

### Type Hint Patterns

#### Flask Routes with Decorators

```python
from flask import Blueprint, render_template
from flask_login import login_required

@blueprint.route("/")  # type: ignore[misc]
@login_required  # type: ignore[misc]
def index() -> str:
    return render_template(  # type: ignore[no-any-return]
        "tasks/index.html",
        active_page="list",
        title="Tasks"
    )
```

**Why the ignores?**
- `# type: ignore[misc]`: Flask decorators aren't typed, mypy can't verify decorator compatibility
- `# type: ignore[no-any-return]`: `render_template()` returns `Any` in Flask's type stubs

#### Routes with Status Codes

```python
from typing import Any
from flask import Response

# Simple tuple return
@blueprint.route("/api/data")  # type: ignore[misc]
def api_data() -> tuple[Any, int]:
    return jsonify({"status": "ok"}), 200

# Mixed return types (Response or tuple)
@blueprint.route("/api/toggle")  # type: ignore[misc]
def toggle() -> Response | tuple[Any, int]:
    if some_condition:
        return jsonify({"success": True})  # type: ignore[no-any-return]
    return jsonify({"error": "Failed"}), 500
```

#### SQLAlchemy Models (Current Style)

```python
from system.db.database import db
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy import String, Integer

class User(db.Model):
    # Current: old style (works but not ideal for strict typing)
    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(100))

    # Future: SQLAlchemy 2.0 typed style
    # id: Mapped[int] = mapped_column(primary_key=True)
    # name: Mapped[str] = mapped_column(String(100))
```

#### User Loader Functions

```python
from modules.core.models.user import User

@login_manager.user_loader
def load_user(user_id: str) -> User | None:
    return db.session.get(User, int(user_id))
```

#### Request Handlers

```python
from flask import Response, redirect, url_for

def index() -> Response | str:
    """Redirect root to dashboard"""
    return redirect(url_for("dashboard_bp.index"))  # type: ignore[no-any-return]

@app.before_request
def before_request() -> None:
    """Global request setup"""
    g.lang = request.args.get("lang", "en")
```

### Common Type Ignore Patterns

| Pattern | When to Use | Example |
|---------|-------------|---------|
| `# type: ignore[misc]` | Untyped decorators | `@blueprint.route()`, `@login_required` |
| `# type: ignore[no-any-return]` | Functions returning `Any` | `render_template()`, `redirect()`, `url_for()` |
| `# type: ignore[attr-defined]` | Dynamic attributes | `app.socketio`, `app.module_loader` |
| `# type: ignore[arg-type]` | Type mismatch in args | Complex SQLAlchemy queries |

### SQLAlchemy 2.0 Migration Plan

**Goal**: Convert models to use `Mapped[T]` and `mapped_column()` for better type safety.

**Current blockers**:
- 34+ model files using old `db.Column` style
- Need comprehensive testing after migration
- Breaking change requires careful coordination

**Migration example**:
```python
# Before (old style)
class User(db.Model):
    id = db.Column(db.Integer, primary_key=True)
    email = db.Column(db.String(120), unique=True, nullable=False)
    is_active = db.Column(db.Boolean, default=True)

# After (SQLAlchemy 2.0 style)
from typing import Optional
from sqlalchemy.orm import Mapped, mapped_column

class User(db.Model):
    id: Mapped[int] = mapped_column(primary_key=True)
    email: Mapped[str] = mapped_column(String(120), unique=True)
    is_active: Mapped[bool] = mapped_column(default=True)
```

### Philosophy

**Pragmatic typing over perfection:**
- Use `# type: ignore` strategically for framework limitations
- Flask/SQLAlchemy weren't designed for strict typing
- Basic mypy provides 80% of the value without framework friction
- Focus on business logic type safety, not framework boilerplate

**When to add type hints:**
- All new functions should have return type annotations
- Add parameter types for non-obvious arguments
- Use `| None` for optional returns
- Use `Any` sparingly, prefer specific types when possible

**When type ignores are acceptable:**
- Flask decorator typing issues
- SQLAlchemy dynamic attributes
- Third-party library limitations
- Framework-level code that's well-tested

### Type Safety and Runtime Safety

Type hints and mypy checking are the **first line of defense** against errors:
- Catch type mismatches at development time
- Prevent `None` reference errors
- Ensure consistent function signatures

But types alone aren't enough:
- Types don't validate **values** (e.g., age > 0)
- Types don't prevent **logic errors**
- Types don't handle **runtime exceptions**

That's why Blueprint combines:
1. **Type hints** → Catch errors before running
2. **Input validation** → Verify values are correct
3. **Guard clauses** → Fail fast on invalid state
4. **Exception handling** → Handle errors gracefully
5. **Logging** → Track errors in production

This layered approach provides defense in depth for maximum reliability.

---

## Error Handling & Exceptions

Blueprint uses a structured exception hierarchy and global error handlers to provide consistent, secure error responses.

### Domain Exception Hierarchy

All custom exceptions inherit from `AppError` in `system/exceptions.py`:

```python
from system.exceptions import (
    AppError,           # Base exception
    ValidationError,    # Invalid input (400)
    NotFoundError,      # Resource not found (404)
    ConflictError,      # State conflict (409)
    AuthenticationError,  # Auth failed (401)
    AuthorizationError,   # Permission denied (403)
    ConfigurationError,   # Config error
    ExternalServiceError  # External API failure
)
```

### Raising Exceptions

Always use specific exception types with clear messages:

```python
from system.exceptions import ValidationError, NotFoundError

# Validation error
if not email:
    raise ValidationError("Email is required", field="email")

# Not found error
user = User.query.filter_by(id=user_id).first()
if not user:
    raise NotFoundError("User not found", resource="user", id=user_id)

# Conflict error
existing = User.query.filter_by(email=email).first()
if existing:
    raise ConflictError("Email already registered", email=email)
```

### Global Error Handlers

Error handlers in `app.py` automatically:
- Log errors with unique tracking IDs
- Return JSON for API requests
- Return HTML for browser requests
- Map exception types to HTTP status codes
- Never expose stack traces to users

### Error Response Format

**JSON Response**:
```json
{
  "error": {
    "id": "uuid-here",
    "code": 400,
    "title": "Validation Error",
    "message": "Email is required",
    "field": "email"
  }
}
```

**HTML Response**:
- Uses `templates/errors/error.html`
- Shows error code, title, message
- Displays error ID for support
- Provides navigation buttons

### Best Practices

1. **Use specific exceptions**: Don't catch generic `Exception` unless at top level
2. **Include context**: Add field names, resource types, IDs to exceptions
3. **Log appropriately**:
   - `app.logger.error()` for 5xx errors
   - `app.logger.warning()` for 4xx errors
   - `app.logger.info()` for normal events
4. **Never expose internals**: User-facing messages should be safe and helpful
5. **Track with error IDs**: Every error gets a UUID for support tracking

### Example Route with Proper Error Handling

```python
from flask import Blueprint, request
from flask_login import login_required, current_user
from system.exceptions import ValidationError, AuthorizationError
from system.utils.validation import require_fields

@blueprint.route("/user/<int:user_id>", methods=["DELETE"])
@login_required
def delete_user(user_id: int) -> tuple[dict[str, str], int]:
    # Guard clause for authorization
    if not current_user.is_admin:
        raise AuthorizationError("Admin access required")

    # Guard clause for resource existence
    user = User.query.get(user_id)
    if not user:
        raise NotFoundError("User not found", resource="user", id=user_id)

    # Guard clause for system resources
    if user.is_system:
        raise ConflictError("Cannot delete system user")

    # Perform operation
    db.session.delete(user)
    db.session.commit()

    return {"message": "User deleted"}, 200
```

---

## Input Validation

Blueprint provides validation utilities in `system/utils/validation.py` for consistent input checking.

### Validation Helper Functions

#### require_fields

Validate required form fields:

```python
from system.utils.validation import require_fields

@blueprint.route("/user", methods=["POST"])
@login_required
def create_user() -> tuple[str, int]:
    # Raises ValidationError if fields missing
    require_fields("email", "name", "password")

    email = request.form["email"]
    name = request.form["name"]
    # ...
```

#### require_json_fields

Validate required JSON fields:

```python
from system.utils.validation import require_json_fields

@blueprint.route("/api/user", methods=["POST"])
@login_required
def api_create_user() -> tuple[dict[str, Any], int]:
    data = request.get_json() or {}
    require_json_fields(data, "email", "name", "password")

    # Safe to access fields now
    email = data["email"]
    # ...
```

#### validate_email

Check email format:

```python
from system.utils.validation import validate_email

email = request.form.get("email", "")
validate_email(email)  # Raises ValidationError if invalid
```

#### validate_range

Check numeric ranges:

```python
from system.utils.validation import validate_range

age = int(request.form.get("age", 0))
validate_range(age, min_value=18, max_value=120, field_name="age")
```

#### validate_length

Check string length:

```python
from system.utils.validation import validate_length

password = request.form.get("password", "")
validate_length(password, min_length=8, max_length=100, field_name="password")
```

#### validate_choice

Check if value is in allowed choices:

```python
from system.utils.validation import validate_choice

role = request.form.get("role")
validate_choice(role, choices=["admin", "user", "guest"], field_name="role")
```

### Validation Patterns

#### Guard Clauses

Use guard clauses to fail fast on invalid input:

```python
@blueprint.route("/order", methods=["POST"])
@login_required
def create_order() -> tuple[str, int]:
    # Guard: validate required fields
    require_fields("product_id", "quantity")

    quantity = int(request.form["quantity"])

    # Guard: validate range
    validate_range(quantity, min_value=1, max_value=1000, field_name="quantity")

    # Guard: check resource exists
    product = Product.query.get(product_id)
    if not product:
        raise NotFoundError("Product not found", resource="product")

    # Guard: check business rules
    if not product.in_stock:
        raise ConflictError("Product out of stock")

    # All validation passed - proceed with operation
    order = Order(user_id=current_user.id, product_id=product.id, quantity=quantity)
    db.session.add(order)
    db.session.commit()

    return render_template("order_success.html", order=order), 201
```

### Best Practices

1. **Validate at boundaries**: Check all external input (forms, JSON, query params)
2. **Fail fast**: Use guard clauses at the start of functions
3. **Be specific**: Provide field names in validation errors
4. **Don't trust types**: Parse and validate even if types are hinted
5. **Use helpers**: Don't repeat validation logic
6. **Check business rules**: Validate both format and semantics

### Integration with Type Safety

Validation complements type checking:

```python
def create_user(name: str, age: int, email: str) -> User:
    # Type hints ensure correct types are passed
    # But still validate values at runtime

    validate_length(name, min_length=1, max_length=100, field_name="name")
    validate_range(age, min_value=0, max_value=150, field_name="age")
    validate_email(email)

    # Now safe to use
    user = User(name=name, age=age, email=email)
    return user
```

---

## Production Security

Blueprint implements security best practices for production deployment.

### Security Configuration

All security settings are in `app.py`:

```python
# Environment-based debug mode
debug_mode = os.environ.get("FLASK_DEBUG", "False").lower() == "true"

# Secret key with warning for dev default
secret_key = os.environ.get("SECRET_KEY", "dev")
if secret_key == "dev":
    app.logger.warning(
        "WARNING: Using default SECRET_KEY='dev'. "
        "Set SECRET_KEY environment variable for production!"
    )
app.config["SECRET_KEY"] = secret_key

# Session security
app.config["SESSION_COOKIE_SECURE"] = not debug_mode  # HTTPS only in production
app.config["SESSION_COOKIE_HTTPONLY"] = True  # No JavaScript access
app.config["SESSION_COOKIE_SAMESITE"] = "Lax"  # CSRF protection

# Error handling
app.config["PROPAGATE_EXCEPTIONS"] = False  # Let handlers catch errors
```

### Environment Variables

Required environment variables for production:

- `SECRET_KEY`: Strong random string for session signing
- `FLASK_DEBUG`: Set to "False" for production
- `FLASK_ENV`: Set to "production"
- `DATABASE_URL`: Production database connection string

Example `.env` file:

```bash
SECRET_KEY=generate-a-strong-random-key-here
FLASK_DEBUG=False
FLASK_ENV=production
DATABASE_URL=postgresql://user:pass@localhost/dbname
```

### Production Checklist

Before deploying to production:

- [ ] `SECRET_KEY` is set to a strong random value
- [ ] `FLASK_DEBUG=False`
- [ ] `SESSION_COOKIE_SECURE=True` (requires HTTPS)
- [ ] Database is backed up regularly
- [ ] Error logging is configured (see Logging section)
- [ ] SSL/TLS certificates are valid
- [ ] Run behind a reverse proxy (nginx, Caddy)
- [ ] Use a proper WSGI server (gunicorn, uWSGI)
- [ ] Static files served by web server, not Flask
- [ ] CSRF protection enabled (if using forms)

### Running in Production

**Don't use**:
```bash
python app.py  # Development server only!
```

**Use**:
```bash
gunicorn "app:create_app()" -w 4 -b 127.0.0.1:8000
```

With environment variables:
```bash
export SECRET_KEY="your-secret-key"
export FLASK_DEBUG="False"
export FLASK_ENV="production"
gunicorn "app:create_app()" \
  -w 4 \
  -b 127.0.0.1:8000 \
  --access-logfile - \
  --error-logfile -
```

### Debug Mode Safety

The application checks `FLASK_DEBUG` environment variable:

```python
# In app.py
debug_mode = os.environ.get("FLASK_DEBUG", "False").lower() == "true"

# Used in
flask_app.socketio.run(flask_app, debug=debug_mode, ...)
```

Default is `False` for safety. Explicitly set to "True" for development only.

### Session Security

Sessions are protected with:

1. **SECURE flag**: Cookies only sent over HTTPS in production
2. **HTTPONLY flag**: JavaScript cannot access session cookie
3. **SAMESITE=Lax**: Protection against CSRF attacks
4. **Strong SECRET_KEY**: Session data is cryptographically signed

### HTTPS Enforcement

When behind a reverse proxy with HTTPS:

```python
# In nginx configuration
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

Flask will automatically detect HTTPS and set secure cookies.

### Authentication & Authorization

- Use `@login_required` decorator for protected routes
- Use `@admin_required` for admin-only routes
- Use `@group_required("GroupName")` for group-based access
- Never expose authentication logic in error messages

### Security Best Practices

1. **Never log sensitive data**: Passwords, tokens, keys
2. **Validate all input**: At system boundaries
3. **Use parameterized queries**: Prevent SQL injection (SQLAlchemy handles this)
4. **Escape output**: Jinja2 templates auto-escape by default
5. **Rate limit APIs**: Prevent abuse
6. **Monitor error logs**: Watch for attack patterns
7. **Keep dependencies updated**: Regular security patches

---

## Logging & Monitoring

Blueprint uses structured logging with appropriate levels and error tracking.

### Log Levels

Log levels are automatically set based on environment:

```python
# Development
FLASK_DEBUG=True  → logging.DEBUG

# Production
FLASK_DEBUG=False → logging.INFO
```

### Logging Patterns

#### By Severity

```python
# DEBUG: Detailed diagnostic info
app.logger.debug("User query returned %d results", len(results))

# INFO: General informational messages
app.logger.info("User %s logged in", user.email)

# WARNING: Something unexpected but handled
app.logger.warning("Invalid password attempt for user %s", username)

# ERROR: Error that was handled
app.logger.error("Failed to send email to %s: %s", email, str(e))

# EXCEPTION: Logs error with full stack trace
app.logger.exception("Unhandled exception in process_order")
```

#### With Error IDs

All errors get unique tracking IDs:

```python
import uuid

error_id = str(uuid.uuid4())
app.logger.error("Payment processing failed [%s]: %s", error_id, str(e))

# Return error_id to user for support reference
return {"error": {"id": error_id, "message": "Payment failed"}}, 500
```

#### Context Logging

Include relevant context in log messages:

```python
app.logger.info(
    "Order created: user_id=%s, order_id=%s, total=%s",
    current_user.id,
    order.id,
    order.total
)
```

### Error Tracking

Global error handlers automatically log:

```python
# In app.py error handlers
@app.errorhandler(Exception)
def handle_unexpected_exception(e: Exception):
    error_id = str(uuid.uuid4())
    app.logger.exception("Unhandled exception [%s]: %s", error_id, str(e))
    # ... return error response with error_id
```

Each error log includes:
- Unique error ID
- Exception type and message
- Full stack trace (server-side only)
- Request context

### Structured Logging Format

```
%(asctime)s - %(name)s - %(levelname)s - %(message)s

# Example output:
2025-01-15 10:30:45 - app - ERROR - Payment failed [abc-123-def]: Connection timeout
```

### Best Practices

1. **Use appropriate levels**:
   - DEBUG: Verbose diagnostic info
   - INFO: Important events (login, order created)
   - WARNING: Unexpected but handled (validation failures)
   - ERROR: Errors that affect functionality
   - EXCEPTION: Unhandled exceptions (use `exception()` not `error()`)

2. **Include context**: User IDs, resource IDs, relevant values

3. **Use error IDs**: For tracking and support

4. **Never log sensitive data**:
   - ❌ Passwords, tokens, API keys
   - ❌ Credit card numbers, SSNs
   - ❌ Personal health information
   - ✅ User IDs, order IDs, timestamps

5. **Log at boundaries**:
   - API requests/responses
   - External service calls
   - Database errors
   - Authentication events

### Monitoring Integration

To integrate with external monitoring (e.g., Sentry):

```python
# In app.py, after creating app
import sentry_sdk
from sentry_sdk.integrations.flask import FlaskIntegration

sentry_sdk.init(
    dsn=os.environ.get("SENTRY_DSN"),
    integrations=[FlaskIntegration()],
    environment=os.environ.get("FLASK_ENV", "development"),
)
```

### Request Tracing

For request tracing, error IDs tie logs to specific requests:

```
2025-01-15 10:30:45 - app - INFO - Request started: GET /api/users
2025-01-15 10:30:45 - app - ERROR - Database query failed [error-id-123]
2025-01-15 10:30:45 - app - INFO - Request completed: GET /api/users - 500
```

User receives `error-id-123` and support can search logs for that ID.

### Log Rotation

In production, use log rotation:

```bash
# In systemd service or supervisor config
--access-logfile /var/log/myapp/access.log \
--error-logfile /var/log/myapp/error.log

# With logrotate
/var/log/myapp/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
}
```

---

## Deployment

### Development
```bash
python app.py
```

### Production
```bash
gunicorn "app:create_app()" -w 4 -b 0.0.0.0:8000
```

→ **See complete guide:** [patterns/deployment.md](patterns/deployment.md)

---

## 🤖 AI Agent Instructions

### For Claude/AI Assistants

**When creating a new module:**
1. Read § Quick Start (top of file)
2. Execute steps 1-7 in order
3. Verify: Module appears in app switcher

**When modifying existing module:**
1. Read the module's `__manifest__.py` to understand it
2. Follow MVC pattern (fat models, thin controllers)
3. Use `{{ _("text") }}` for all strings
4. Add translations to `lang/*.json`

**When troubleshooting:**
1. Check `modules/*/` folder structure matches expected
2. Verify `module_instance` is exported from `__init__.py`
3. Check for `__DISABLED__` file
4. Review console output for module loading errors

### Files to Read (in order)
1. `CLAUDE.md` - master blueprint (this file)
2. `patterns/module-system.md` - module architecture details
3. Other `patterns/*.md` as needed

---

## License

Copyright (c) [Year] [Your Organization]

---

**That's it.** One file to guide any Blueprint modular application.
**Build modules, extend features, scale cleanly.**
