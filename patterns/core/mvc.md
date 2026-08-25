# MVC Pattern Reference

> Complete guide to Models, Views, and Controllers in Flask applications

---

## Table of Contents

- [Philosophy](#philosophy)
- [Models (Fat Models)](#models-fat-models)
- [Views (Templates)](#views-templates)
- [Controllers (Thin Controllers)](#controllers-thin-controllers)
- [Request Flow](#request-flow)
- [Best Practices](#best-practices)

---

## Philosophy

### Fat Models, Thin Controllers

**Models** contain:
- Business logic (validation, calculations)
- Database access (CRUD, queries)
- Domain rules

**Controllers** contain:
- Parse input (query params, form data, JSON)
- Call model methods
- Render view or return JSON
- Handle HTTP-specific concerns (status codes, headers)

**Views** contain:
- Jinja2 templates
- Minimal logic (loops, conditions)
- HTMX attributes for interactivity

---

## Models (Fat Models)

### Base Model

```python
# app/models/base.py
"""Base model with common fields and methods."""

from app.extensions import db
from app.models.base import utcnow
from app.models.types import BigIntPK


class BaseModel(db.Model):
    """Abstract base model with timestamps."""

    __abstract__ = True

    # BIGINT primary keys do not autoincrement on SQLite -- BigIntPK applies the
    # required variant. See core/portability.md.
    id = db.Column(BigIntPK, primary_key=True, autoincrement=True)
    created_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    updated_at = db.Column(db.DateTime(timezone=True), nullable=False,
                           default=utcnow, onupdate=utcnow)

    def save(self):
        """Validate and persist. Commits immediately -- wrap multi-step
        operations in transaction() so they stay atomic."""
        if hasattr(self, 'validate'):
            self.validate()
        db.session.add(self)
        db.session.commit()
        return self

    def delete(self):
        """Delete model from database."""
        db.session.delete(self)
        db.session.commit()

    @classmethod
    def get_by_id(cls, id: int):
        """Get record by ID."""
        return db.session.get(cls, id)

    @classmethod
    def get_or_404(cls, id: int):
        """Get record by ID or raise 404."""
        from app.platform.errors import NotFoundError
        record = db.session.get(cls, id)
        if record is None:
            raise NotFoundError(f'{cls.__name__} not found')
        return record
```

### User Model (Primary Example)

```python
# app/models/user.py
"""User model - example of a fat model."""

import re
from typing import Optional, List
from flask_login import UserMixin
from app.extensions import db
from app.platform.errors import ValidationError, NotFoundError
from .base import BaseModel


class User(BaseModel, UserMixin):
    """User model with authentication support."""

    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False, index=True)
    name = db.Column(db.String(100), nullable=False)
    avatar_url = db.Column(db.String(500), nullable=True)
    is_active = db.Column(db.Boolean, default=True, nullable=False)

    # Relationships
    settings = db.relationship('Setting', backref='user', lazy='dynamic',
                               cascade='all, delete-orphan')

    def validate(self):
        """Validate user data before saving."""
        self.email = self.email.strip().lower() if self.email else ''
        self.name = self.name.strip() if self.name else ''

        if not self.email:
            raise ValidationError('Email is required')
        if not self._is_valid_email(self.email):
            raise ValidationError('Invalid email format')
        if not self.name:
            raise ValidationError('Name is required')
        if len(self.name) > 100:
            raise ValidationError('Name too long (max 100 chars)')

        # Friendly duplicate check. This races under concurrency -- the
        # unique constraint on email is the real guarantee, so controllers
        # must also catch IntegrityError.
        existing = User.query.filter_by(email=self.email).first()
        if existing and existing.id != self.id:
            raise ValidationError('Email already registered')

    @staticmethod
    def _is_valid_email(email: str) -> bool:
        pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
        return bool(re.match(pattern, email))

    def update(self, name: Optional[str] = None, avatar_url: Optional[str] = None):
        """Update user profile."""
        if name is not None:
            self.name = name
        if avatar_url is not None:
            self.avatar_url = avatar_url
        return self.save()

    def get_initials(self) -> str:
        """Get user's initials for avatar placeholder."""
        return self.name[0].upper() if self.name else self.email[0].upper()

    # -------------------------------------------------------------------------
    # Query methods (class-level)
    # -------------------------------------------------------------------------

    @classmethod
    def get_by_email(cls, email: str) -> Optional['User']:
        """Get user by email address."""
        email = email.strip().lower() if email else ''
        return cls.query.filter_by(email=email).first()

    @classmethod
    def list_all(cls, limit: int = 50, offset: int = 0) -> List['User']:
        """List all users with pagination."""
        return cls.query.order_by(cls.created_at.desc()) \
            .limit(limit).offset(offset).all()

    @classmethod
    def create(cls, email: str, name: str, avatar_url: Optional[str] = None) -> 'User':
        """Create a new user."""
        user = cls(email=email, name=name, avatar_url=avatar_url)
        return user.save()
```

### Setting Model (Key-Value Pattern)

```python
# app/models/setting.py
"""Setting model - key-value storage per user."""

from typing import Optional, Dict
from app.extensions import db
from app.platform.errors import ValidationError
from .base import BaseModel


class Setting(BaseModel):
    """User setting model (key-value pairs)."""

    __tablename__ = 'setting'

    user_id = db.Column(db.BigInteger, db.ForeignKey('user.id', ondelete='CASCADE'),
                        nullable=False, index=True)
    key = db.Column(db.String(100), nullable=False)
    value = db.Column(db.Text, nullable=False, default='')

    __table_args__ = (
        db.UniqueConstraint('user_id', 'key', name='uq_setting_user_key'),
    )

    def validate(self):
        """Validate setting data."""
        self.key = self.key.strip() if self.key else ''
        if not self.key:
            raise ValidationError('Setting key is required')
        if not self.user_id:
            raise ValidationError('User ID is required')

    @classmethod
    def get_for_user(cls, user_id: int, key: str) -> Optional['Setting']:
        """Get a specific setting for a user."""
        return cls.query.filter_by(user_id=user_id, key=key).first()

    @classmethod
    def get_value(cls, user_id: int, key: str, default: str = '') -> str:
        """Get setting value with default fallback."""
        setting = cls.get_for_user(user_id, key)
        return setting.value if setting else default

    @classmethod
    def get_map(cls, user_id: int) -> Dict[str, str]:
        """Get all settings for a user as a dictionary."""
        settings = cls.query.filter_by(user_id=user_id).all()
        return {s.key: s.value for s in settings}

    @classmethod
    def set(cls, user_id: int, key: str, value: str) -> 'Setting':
        """Create or update a setting (upsert pattern)."""
        setting = cls.get_for_user(user_id, key)
        if setting:
            setting.value = value
        else:
            setting = cls(user_id=user_id, key=key, value=value)
        return setting.save()
```

---

## Views (Templates)

### Base Layout

Full layout, including the CSRF header and skip link:
[frontend.md § Component Patterns](frontend.md#component-patterns).

```html
<!-- app/views/layouts/base.html -->
<!DOCTYPE html>
<html lang="{{ lang }}" dir="{{ 'rtl' if is_rtl else 'ltr' }}"
      class="{{ 'dark' if theme_mode == 'dark' }}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{% block title %}Your App{% endblock %}</title>

  <link href="{{ url_for('static', filename='css/app.css') }}" rel="stylesheet">
  <script src="{{ url_for('static', filename='js/htmx.min.js') }}"></script>
  <script defer src="{{ url_for('static', filename='js/alpine.min.js') }}"></script>
  {% block head %}{% endblock %}
</head>
<body class="min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-900 dark:text-slate-100"
      hx-headers='{"X-CSRF-Token": "{{ csrf_token }}"}'>

  {% include 'partials/_navbar.html' %}

  <main id="main" class="mx-auto max-w-5xl px-4 py-8">
    {% include 'partials/_flash.html' %}
    {% block content %}{% endblock %}
  </main>

  {% include 'partials/_toasts.html' %}
  {% block scripts %}{% endblock %}
</body>
</html>
```

### User Profile Page

```html
<!-- app/views/users/profile.html -->
{% extends 'layouts/base.html' %}

{% block title %}{{ user.name }} — {{ _('user.profile') }}{% endblock %}

{% block content %}
<div class="mx-auto max-w-2xl">
  <div class="card">
    <div class="flex items-center gap-4">
      {% if user.avatar_url %}
        <img src="{{ user.avatar_url }}" alt="" class="size-20 rounded-full">
      {% else %}
        <div class="flex size-20 items-center justify-center rounded-full
                    bg-brand-600 text-3xl text-white" aria-hidden="true">
          {{ user.get_initials() }}
        </div>
      {% endif %}

      <div class="flex-1">
        <h1 class="text-xl font-semibold">{{ user.name }}</h1>
        <p class="text-sm text-slate-600 dark:text-slate-400">{{ user.email }}</p>
      </div>

      <a href="{{ url_for('users.edit_profile') }}" class="btn-secondary">
        {{ _('user.edit_profile') }}
      </a>
    </div>

    <hr class="my-6 border-slate-200 dark:border-slate-700">

    <dl class="grid grid-cols-3 gap-2 text-sm">
      <dt class="text-slate-500">{{ _('user.member_since') }}</dt>
      <dd class="col-span-2">{{ user.created_at | localdate }}</dd>
    </dl>
  </div>
</div>
{% endblock %}
```

`text-slate-600` rather than `text-slate-500` for the email: the lighter shade
falls below WCAG AA on white. See
[frontend.md § Accessibility](frontend.md#accessibility).

### Settings Form with HTMX

```html
<!-- app/views/settings/index.html -->
{% extends 'layouts/base.html' %}

{% block content %}
<div class="mx-auto max-w-2xl">
  <h1 class="mb-6 text-xl font-semibold">{{ _('settings.title') }}</h1>

  <div class="card space-y-4">
    <div>
      <label for="theme" class="label">{{ _('settings.theme') }}</label>
      <select id="theme" name="value" class="select mt-1"
              hx-post="{{ url_for('settings.update') }}"
              hx-trigger="change"
              hx-vals='{"key": "theme"}'
              hx-target="#theme-status"
              hx-swap="innerHTML">
        <option value="light" {{ 'selected' if settings.theme == 'light' }}>
          {{ _('settings.theme.light') }}</option>
        <option value="dark" {{ 'selected' if settings.theme == 'dark' }}>
          {{ _('settings.theme.dark') }}</option>
      </select>
      <p id="theme-status" class="mt-1 text-sm text-green-600" role="status"
         aria-live="polite"></p>
    </div>
  </div>
</div>
{% endblock %}
```

`role="status"` with `aria-live="polite"` makes the HTMX confirmation audible to
a screen reader instead of purely visual.

---

## Controllers (Thin Controllers)

### Main Controller

```python
# app/controllers/main.py
"""Main controller - public pages."""

from flask import Blueprint, render_template

bp = Blueprint('main', __name__)


@bp.route('/')
def index():
    """Home page."""
    return render_template('main/index.html')


@bp.route('/health')
def health():
    """Health check endpoint."""
    return {'status': 'ok'}
```

### Users Controller

```python
# app/controllers/users.py
"""Users controller - profile management."""

from flask import Blueprint, render_template, request, redirect, url_for, flash
from flask_login import login_required, current_user
from app.models import User
from app.platform.errors import ValidationError
from app.platform.logger import get_logger

bp = Blueprint('users', __name__)
log = get_logger()


@bp.route('/profile')
@login_required
def profile():
    """Show current user's profile."""
    return render_template('users/profile.html', user=current_user)


@bp.route('/profile/edit', methods=['GET', 'POST'])
@login_required
def edit_profile():
    """Edit current user's profile."""
    errors = {}

    if request.method == 'POST':
        name = request.form.get('name', '').strip()
        avatar_url = request.form.get('avatar_url', '').strip()

        try:
            current_user.update(name=name, avatar_url=avatar_url or None)
            log.info('profile_updated', user_id=current_user.id)
            flash('Profile updated successfully', 'success')
            return redirect(url_for('users.profile'))

        except ValidationError as e:
            errors['form'] = e.message

    return render_template('users/edit.html', user=current_user, errors=errors)
```

### Settings Controller (HTMX)

```python
# app/controllers/settings.py
"""Settings controller - user preferences."""

from flask import Blueprint, render_template, request, redirect, url_for
from flask_login import login_required, current_user
from app.models import Setting

bp = Blueprint('settings', __name__)


@bp.route('/settings')
@login_required
def index():
    """Show settings page."""
    settings = Setting.get_map(current_user.id)

    # Provide defaults
    defaults = {'theme': 'light', 'language': 'en'}
    for key, default in defaults.items():
        if key not in settings:
            settings[key] = default

    return render_template('settings/index.html', settings=settings)


@bp.route('/settings', methods=['POST'])
@login_required
def update():
    """Update a single setting (HTMX endpoint)."""
    key = request.form.get('key', '').strip()
    value = request.form.get('value', '').strip()

    if not key:
        return 'Key required', 400

    Setting.set(current_user.id, key, value)

    # HTMX response
    if request.headers.get('HX-Request') == 'true':
        return 'Saved'

    return redirect(url_for('settings.index'))
```

---

## Request Flow

### Typical Request Lifecycle

1. **Request arrives** → Flask router
2. **Before request hooks** run (auth, request ID)
3. **Route matched** → Controller function called
4. **Decorators** run (@login_required, etc.)
5. **Controller**:
   - Extract user from `current_user`
   - Parse input (request.form, request.args)
   - Call model method
   - Handle errors
   - Render template or return JSON
6. **Response sent** to client

---

## Best Practices

### Models

✅ **Do:**
- Keep all business logic in models
- Use class methods for queries
- Validate in `validate()` method before save
- Return `self` from `save()` for chaining

❌ **Don't:**
- Don't access Flask request/session in models
- Don't log in models (raise errors instead)
- Don't hardcode values (use constants or config)

### Controllers

✅ **Do:**
- Keep controllers thin (just HTTP orchestration)
- Use `current_user` for authenticated user
- Check `HX-Request` header for HTMX requests
- Log actions with context

❌ **Don't:**
- Don't put business logic in controllers
- Don't write raw SQL in controllers
- Don't return internal error details to users

### Views

✅ **Do:**
- Use partials for reusable components (prefix with `_`)
- Use HTMX attributes for interactivity
- Use Tailwind utilities inline; reach for a component class only when it repeats
- Use i18n for all user-visible text

❌ **Don't:**
- Don't put complex logic in templates
- Don't use inline styles (the one exception is per-tenant brand variables)
- Don't hardcode text strings
- Don't use `|safe` unless content is trusted

---

**Next:** [Database Patterns](database.md) | [HTMX Patterns](htmx.md) | [Testing](testing.md)
