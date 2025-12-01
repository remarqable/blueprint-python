# Database Patterns

> SQLAlchemy with ModelRegistry, module-scoped models, and query patterns.

---

## Table of Contents

- [Database Setup](#database-setup)
- [ModelRegistry Pattern](#modelregistry-pattern)
- [Model Conventions](#model-conventions)
- [Query Patterns](#query-patterns)
- [Sample Data Pattern](#sample-data-pattern)
- [Relationships](#relationships)
- [Transactions](#transactions)
- [Multi-Tenancy](#multi-tenancy)
- [Performance](#performance)

---

## Database Setup

### SQLite (Default)

SQLite is the default - no setup required. Database file created at `app.db`.

```python
# In app.py
app.config["SQLALCHEMY_DATABASE_URI"] = "sqlite:///" + os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "app.db"
)
```

**SQLite works great for:**
- Development and prototyping
- Small to medium production apps
- Single-server deployments

### Database Instance

The SQLAlchemy instance is centralized in the system layer:

```python
# system/db/database.py
from flask_sqlalchemy import SQLAlchemy

db = SQLAlchemy()
```

All modules import from this location:

```python
from system.db.database import db
```

---

## ModelRegistry Pattern

Blueprint uses `@ModelRegistry.register` to track all models across modules.

### Basic Usage

```python
# modules/yourmodule/models/item.py
from system.db.database import db
from system.db.decorators import ModelRegistry


@ModelRegistry.register
class Item(db.Model):
    __tablename__ = "item"

    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(255), nullable=False)
```

### Association Tables

For many-to-many relationships:

```python
# modules/core/models/user_group.py
from system.db.database import db
from system.db.decorators import ModelRegistry

user_group = db.Table(
    "user_group",
    db.Column("user_id", db.Integer, db.ForeignKey("user.id")),
    db.Column("group_id", db.Integer, db.ForeignKey("group.id")),
    db.UniqueConstraint("user_id", "group_id"),
)

# Register with explicit module name
ModelRegistry.register_table(user_group, "core")
```

### Registry Output

On startup, the registry prints a summary:

```
Database Model Registry:
--------   ----------   --------------------
Module     Model        Table
--------   ----------   --------------------
core       User         user
core       Group        group
core       user_group   user_group
tasks      Task         task
```

### Registry Internals

```python
# system/db/decorators.py
class ModelRegistry:
    models = []
    registration_order = 1
    MODULE_ORDER = ["core"]  # Only core is required

    @classmethod
    def register(cls, model_class):
        """Decorator to register a model."""
        # Extract module name from path
        module_path = model_class.__module__.split(".")
        if "modules" in module_path:
            module_name = module_path[module_path.index("modules") + 1]
        else:
            module_name = "core"

        cls.models.append({
            "module": module_name,
            "model": model_class.__name__,
            "table": model_class.__tablename__,
            "order": cls.registration_order,
        })
        cls.registration_order += 1
        return model_class
```

---

## Model Conventions

### Naming

- **Tables**: lowercase, singular (`user`, `task`)
- **Columns**: snake_case (`created_at`, `user_id`, `title`)
- **Foreign keys**: `<entity>_id` (`user_id`, `group_id`)
- **Models**: PascalCase, singular (`User`, `Task`)
- **Tablename**: Always explicit with `__tablename__`

### Standard Model Structure

```python
# modules/yourmodule/models/item.py
from system.db.database import db
from system.db.decorators import ModelRegistry


@ModelRegistry.register
class Item(db.Model):
    __tablename__ = "item"

    # --- Columns ---
    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(255), nullable=False)
    description = db.Column(db.Text)
    is_active = db.Column(db.Boolean, default=True)
    created_at = db.Column(db.DateTime, default=db.func.current_timestamp())
    updated_at = db.Column(db.DateTime, onupdate=db.func.current_timestamp())

    # --- Relationships ---
    # user = db.relationship("User", backref="items")

    # --- Class Methods (CRUD) ---
    @classmethod
    def create(cls, name, description=None):
        """Create a new item."""
        item = cls(name=name, description=description)
        db.session.add(item)
        db.session.commit()
        return item

    @classmethod
    def get_all(cls):
        """Get all items."""
        return cls.query.all()

    @classmethod
    def get_by_id(cls, item_id):
        """Get item by ID."""
        return cls.query.get(item_id)

    @classmethod
    def delete(cls, item_id):
        """Delete an item."""
        item = cls.query.get(item_id)
        if item:
            db.session.delete(item)
            db.session.commit()
            return True
        return False

    @classmethod
    def update(cls, item_id, **kwargs):
        """Update an item."""
        item = cls.query.get(item_id)
        if item:
            for key, value in kwargs.items():
                if hasattr(item, key):
                    setattr(item, key, value)
            db.session.commit()
            return item
        return None

    # --- Static Methods ---
    @staticmethod
    def get_active():
        """Get active items."""
        return Item.query.filter_by(is_active=True).all()

    # --- Sample Data ---
    @classmethod
    def create_sample_data(cls):
        """Create sample data if table is empty."""
        if not cls.query.first():
            cls.create("Sample Item 1", "Description 1")
            cls.create("Sample Item 2", "Description 2")
```

---

## Query Patterns

### Basic Queries

```python
# Get by ID
item = Item.query.get(1)
item = Item.query.get_or_404(1)  # Raises 404 if not found

# Filter
item = Item.query.filter_by(name="Test").first()
items = Item.query.filter(Item.is_active == True).all()

# Order and limit
items = Item.query.order_by(Item.created_at.desc()).limit(10).all()

# Count
count = Item.query.count()

# First or None
item = Item.query.filter_by(name="Test").first()
```

### Pagination

```python
@classmethod
def list_paginated(cls, page=1, per_page=20):
    """List items with pagination."""
    return cls.query.order_by(cls.created_at.desc()).paginate(
        page=page, per_page=per_page, error_out=False
    )

# Usage
pagination = Item.list_paginated(page=1)
items = pagination.items
total_pages = pagination.pages
has_next = pagination.has_next
```

### Eager Loading (Avoid N+1)

```python
# Load related objects in single query
users = User.query.options(db.joinedload(User.groups)).all()

# Multiple relationships
users = User.query.options(
    db.joinedload(User.groups),
    db.joinedload(User.settings)
).all()
```

---

## Sample Data Pattern

Every model should have a `create_sample_data()` method:

```python
@classmethod
def create_sample_data(cls):
    """Create sample data for development/demo.

    Called from module's init_database() hook.
    Must be idempotent (safe to call multiple times).
    """
    if not cls.query.first():  # Only if table is empty
        cls.create("Sample 1")
        cls.create("Sample 2")
```

**In module.py:**

```python
@hookimpl
def init_database(self):
    """Initialize database."""
    db.create_all()
    try:
        from .models.item import Item
        Item.create_sample_data()
    except Exception as e:
        import logging
        logging.getLogger(__name__).error(f"Sample data error: {e}")
```

---

## Relationships

### One-to-Many

```python
# Parent (User)
@ModelRegistry.register
class User(db.Model):
    __tablename__ = "user"
    id = db.Column(db.Integer, primary_key=True)
    tasks = db.relationship("Task", backref="user", lazy="dynamic")


# Child (Task)
@ModelRegistry.register
class Task(db.Model):
    __tablename__ = "task"
    id = db.Column(db.Integer, primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.id"))
```

### Many-to-Many

```python
# Association table
user_group = db.Table(
    "user_group",
    db.Column("user_id", db.Integer, db.ForeignKey("user.id")),
    db.Column("group_id", db.Integer, db.ForeignKey("group.id")),
)
ModelRegistry.register_table(user_group, "core")


# User model
@ModelRegistry.register
class User(db.Model):
    __tablename__ = "user"
    groups = db.relationship("Group", secondary=user_group, backref="users")


# Group model
@ModelRegistry.register
class Group(db.Model):
    __tablename__ = "group"
    id = db.Column(db.Integer, primary_key=True)
    name = db.Column(db.String(64), unique=True)
```

### One-to-One

```python
# User
@ModelRegistry.register
class User(db.Model):
    __tablename__ = "user"
    tasks = db.relationship("Task", backref="owner")


# Task
@ModelRegistry.register
class Task(db.Model):
    __tablename__ = "task"
    user_id = db.Column(db.Integer, db.ForeignKey("user.id"))
```

---

## Transactions

### Automatic Commits

Each model method commits automatically:

```python
@classmethod
def create(cls, name):
    item = cls(name=name)
    db.session.add(item)
    db.session.commit()  # Commits here
    return item
```

### Manual Transaction

```python
def transfer_credits(from_id, to_id, amount):
    """Atomic credit transfer."""
    try:
        from_user = User.query.get(from_id)
        to_user = User.query.get(to_id)

        from_user.credits -= amount
        to_user.credits += amount

        db.session.commit()
    except Exception:
        db.session.rollback()
        raise
```

### Context Manager Pattern

```python
from contextlib import contextmanager

@contextmanager
def transaction():
    """Transaction context manager."""
    try:
        yield db.session
        db.session.commit()
    except Exception:
        db.session.rollback()
        raise


# Usage
with transaction():
    user1.credits -= 100
    user2.credits += 100
```

---

## Multi-Tenancy

### Group-Based Access (Blueprint Default - Single Tenant)

Blueprint uses group-based access control for **single-tenant applications**:

```python
# User belongs to groups within the same organization
user.groups  # [ALL, ADMIN, ...]

# Check membership
if current_user.is_admin:
    # Admin-only action

# All users share the same database/organization
# No tenant_id filtering needed
```

**Note:** This pattern assumes all users belong to one organization. Queries don't need tenant isolation.

### User-Owned Data Pattern

```python
@ModelRegistry.register
class Setting(db.Model):
    __tablename__ = "user_setting"

    id = db.Column(db.Integer, primary_key=True)
    user_id = db.Column(db.Integer, db.ForeignKey("user.id"), nullable=False)
    key = db.Column(db.String(255), nullable=False)
    value = db.Column(db.String(255))

    __table_args__ = (db.UniqueConstraint("user_id", "key"),)

    @staticmethod
    def get(user_id, key, default=None):
        """Get setting for user."""
        setting = Setting.query.filter_by(user_id=user_id, key=key).first()
        return setting.value if setting else default

    @staticmethod
    def set(user_id, key, value):
        """Set setting for user."""
        setting = Setting.query.filter_by(user_id=user_id, key=key).first()
        if setting:
            setting.value = value
        else:
            setting = Setting(user_id=user_id, key=key, value=value)
            db.session.add(setting)
        db.session.commit()
```

---

## Performance

### Indexes

```python
# Single column index
email = db.Column(db.String(255), index=True)

# Unique index
email = db.Column(db.String(255), unique=True)

# Composite index
__table_args__ = (
    db.Index("ix_setting_user_key", "user_id", "key"),
)
```

### Query Optimization

```python
# Limit columns fetched
users = User.query.options(db.load_only(User.id, User.email)).all()

# Batch inserts
db.session.bulk_insert_mappings(User, [
    {"email": "user1@example.com", "name": "User 1"},
    {"email": "user2@example.com", "name": "User 2"},
])
db.session.commit()

# Batch updates
User.query.filter(User.is_active == False).update({"is_active": True})
db.session.commit()
```

### Avoid N+1 Queries

```python
# BAD - N+1 queries
users = User.query.all()
for user in users:
    print(user.tasks)  # Separate query per user

# GOOD - Single query with joinedload
users = User.query.options(db.joinedload(User.tasks)).all()
for user in users:
    print(user.tasks)  # Already loaded
```

---

## Cross-Module References

When referencing models from other modules:

```python
# In modules/tasks/models/task.py
from modules.core.models.user import User

@ModelRegistry.register
class Task(db.Model):
    user_id = db.Column(db.Integer, db.ForeignKey("user.id"))
    owner = db.relationship("User", backref="tasks")
```

**Note:** Direct imports create coupling. Only depend on core module unless necessary.

---

**Next:** [Module System](module-system.md) | [MVC Pattern](mvc.md) | [Testing](testing.md)
