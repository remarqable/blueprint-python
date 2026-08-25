# Security Patterns

> CSRF protection, rate limiting, input validation, and security checklist

---

## Table of Contents

- [Security Checklist](#security-checklist)
- [CSRF Protection](#csrf-protection)
- [Rate Limiting](#rate-limiting)
- [Input Validation](#input-validation)
- [SQL Injection Prevention](#sql-injection-prevention)
- [XSS Prevention](#xss-prevention)
- [Session Security](#session-security)
- [Security Headers](#security-headers)
- [Secrets Management](#secrets-management)

---

## Security Checklist

### Must-Have (Before Production)

- [ ] **CSRF protection** on all state-changing requests
- [ ] **Rate limiting** on auth endpoints
- [ ] **Input validation** (length, format, type)
- [ ] **Parameterized queries** (SQLAlchemy handles this)
- [ ] **Auto-escaping templates** (Jinja2 default)
- [ ] **Secure sessions** (HttpOnly, Secure, SameSite)
- [ ] **HTTPS only** in production
- [ ] **Security headers** configured

### Nice-to-Have

- [ ] Content Security Policy (CSP)
- [ ] Two-factor authentication
- [ ] Audit logging
- [ ] Penetration testing

---

## CSRF Protection

### Simple Token Implementation

```python
# app/middleware/csrf.py
"""CSRF protection middleware."""

import secrets
from flask import session, request, abort


def generate_csrf_token() -> str:
    """Generate CSRF token and store in session."""
    if '_csrf_token' not in session:
        session['_csrf_token'] = secrets.token_urlsafe(32)
    return session['_csrf_token']


def validate_csrf_token() -> bool:
    """Validate CSRF token from form or header."""
    token = session.get('_csrf_token')
    if not token:
        return False

    submitted = request.form.get('csrf_token') or \
                request.headers.get('X-CSRF-Token')

    if not submitted:
        return False

    return secrets.compare_digest(token, submitted)


def init_csrf(app):
    """Initialize CSRF protection."""

    @app.context_processor
    def csrf_context():
        return {'csrf_token': generate_csrf_token()}

    @app.before_request
    def check_csrf():
        if request.method in ('GET', 'HEAD', 'OPTIONS'):
            return

        if request.path.startswith('/api/'):
            return  # API uses token auth

        if request.path == '/health':
            return

        if not validate_csrf_token():
            abort(403, 'CSRF token invalid')
```

### Usage in Templates

```html
<form method="POST">
  <input type="hidden" name="csrf_token" value="{{ csrf_token }}">
  <!-- form fields -->
</form>
```

### HTMX Requests

```html
<!-- Add token to all HTMX requests -->
<body hx-headers='{"X-CSRF-Token": "{{ csrf_token }}"}'>
```

---

## Rate Limiting

### Simple In-Memory Rate Limiter

```python
# app/middleware/ratelimit.py
"""Rate limiting middleware."""

import time
from functools import wraps
from flask import request, current_app
from app.platform.errors import RateLimitError

# Storage: {key: (count, window_start)}
_rate_limits = {}


def rate_limit(limit: int = 100, window: int = 60):
    """Rate limit decorator.

    Args:
        limit: Max requests per window
        window: Window size in seconds
    """
    def decorator(f):
        @wraps(f)
        def decorated(*args, **kwargs):
            if not current_app.config.get('RATELIMIT_ENABLED', True):
                return f(*args, **kwargs)

            key = f'{f.__name__}:{_get_client_ip()}'

            if not _check_limit(key, limit, window):
                raise RateLimitError()

            return f(*args, **kwargs)
        return decorated
    return decorator


def _get_client_ip() -> str:
    """Get client IP, handling proxies."""
    forwarded = request.headers.get('X-Forwarded-For')
    if forwarded:
        return forwarded.split(',')[0].strip()
    return request.remote_addr or '127.0.0.1'


def _check_limit(key: str, limit: int, window: int) -> bool:
    """Check if request is within limit."""
    now = time.time()
    data = _rate_limits.get(key)

    if data is None:
        _rate_limits[key] = (1, now)
        return True

    count, window_start = data

    if now - window_start > window:
        _rate_limits[key] = (1, now)
        return True

    if count >= limit:
        return False

    _rate_limits[key] = (count + 1, window_start)
    return True
```

### Usage

```python
from app.middleware.ratelimit import rate_limit

@bp.route('/login', methods=['POST'])
@rate_limit(limit=10, window=60)  # 10 attempts per minute
def login():
    ...

@bp.route('/api/data')
@rate_limit(limit=100, window=60)  # 100 requests per minute
def api_data():
    ...
```

### Production: Use Redis

```python
# pip install flask-limiter redis

from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

limiter = Limiter(
    key_func=get_remote_address,
    storage_uri="redis://localhost:6379",
    default_limits=["100 per minute"]
)

# In app factory
limiter.init_app(app)

# Usage
@bp.route('/login', methods=['POST'])
@limiter.limit("10 per minute")
def login():
    ...
```

---

## Input Validation

### Model-Level Validation

```python
# app/models/user.py
import re
from app.platform.errors import ValidationError


class User(BaseModel):
    def validate(self):
        """Validate before saving."""
        self.email = self.email.strip().lower() if self.email else ''
        self.name = self.name.strip() if self.name else ''

        # Required fields
        if not self.email:
            raise ValidationError('Email is required')
        if not self.name:
            raise ValidationError('Name is required')

        # Format validation
        if not re.match(r'^[^@]+@[^@]+\.[^@]+$', self.email):
            raise ValidationError('Invalid email format')

        # Length limits
        if len(self.email) > 255:
            raise ValidationError('Email too long')
        if len(self.name) > 100:
            raise ValidationError('Name too long')
```

### Controller-Level Validation

```python
@bp.route('/profile/edit', methods=['POST'])
@login_required
def edit_profile():
    name = request.form.get('name', '')
    avatar_url = request.form.get('avatar_url', '')

    # Basic sanitization
    name = name.strip()[:100]  # Limit length
    avatar_url = avatar_url.strip()[:500]

    # URL validation
    if avatar_url and not avatar_url.startswith(('http://', 'https://')):
        flash('Invalid avatar URL', 'error')
        return redirect(url_for('users.edit_profile'))

    try:
        current_user.update(name=name, avatar_url=avatar_url or None)
        flash('Profile updated', 'success')
    except ValidationError as e:
        flash(e.message, 'error')

    return redirect(url_for('users.profile'))
```

---

## SQL Injection Prevention

### SQLAlchemy Handles This

```python
# ✅ SAFE: SQLAlchemy parameterizes automatically
user = User.query.filter_by(email=email).first()
users = User.query.filter(User.name.like(f'%{search}%')).all()

# ✅ SAFE: Using text() with bound parameters
from sqlalchemy import text
result = db.session.execute(
    text("SELECT * FROM user WHERE email = :email"),
    {"email": email}
)

# ❌ NEVER DO THIS: String interpolation
result = db.session.execute(f"SELECT * FROM user WHERE email = '{email}'")
```

---

## XSS Prevention

### Jinja2 Auto-Escaping

```html
<!-- ✅ Auto-escaped (safe) -->
<p>{{ user.name }}</p>
<p>{{ user_input }}</p>

<!-- ❌ Dangerous: Only use with trusted content -->
<p>{{ trusted_html|safe }}</p>
```

### Content Security Policy

```python
@app.after_request
def add_security_headers(response):
    # Prevent inline scripts (mitigates XSS)
    response.headers['Content-Security-Policy'] = \
        "default-src 'self'; script-src 'self' 'unsafe-eval'; " \
        "style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; " \
        "object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
    return response
```

### CSP and Alpine.js

`'unsafe-eval'` above is not decoration and not an oversight — **Alpine requires
it.** Alpine 3 compiles inline expressions (`x-on:click="open = !open"`,
`x-show="open"`) with `new Function()`, so under a strict `script-src 'self'`
every Alpine directive silently stops working. Nothing throws a visible error;
your dropdowns and modals simply do nothing.

You have two honest options:

**1. Allow `'unsafe-eval'`** (shown above). Simple, and the practical choice for
most apps. Understand what it costs: it removes CSP's protection against an
attacker who has already achieved script injection turning a string into
executable code. It does **not** weaken your defence against the injection
itself — that is `script-src 'self'` plus Jinja's autoescaping, both still in
force.

**2. Use the Alpine CSP build**, which needs no `eval` because expressions are
pre-registered rather than parsed at runtime:

```html
<script defer src="/static/js/alpine-csp.min.js"></script>
```

```javascript
// app/static/js/components.js — loaded before Alpine
document.addEventListener('alpine:init', () => {
  Alpine.data('dropdown', () => ({
    open: false,
    toggle() { this.open = !this.open },
    close()  { this.open = false },
  }));
});
```

```html
<!-- Only method and property NAMES in attributes. No expressions. -->
<div x-data="dropdown">
  <button x-on:click="toggle" x-bind:aria-expanded="open">Menu</button>
  <div x-show="open" x-on:click.outside="close" x-cloak>…</div>
</div>
```

The cost is real: every component must be registered in JavaScript, and the
inline-expression style used throughout
[frontend.md](frontend.md#component-patterns) has to be rewritten. Pick this if
you handle payment data, health records, or anything else where you will be
asked to justify `'unsafe-eval'` in an audit. Pick it **before** you write the
components, not after.

Whichever you choose, keep `object-src 'none'`, `base-uri 'self'`, and
`frame-ancestors 'none'` — they are free and they close real attacks.

> `style-src 'unsafe-inline'` is separately required by the per-tenant brand
> block in [theming.md](../theming.md#branding-without-templates). If you drop
> per-tenant branding you can drop it too, but Alpine's `x-show` also sets inline
> styles.

---

## Session Security

### Configuration

```python
# app/config.py
class Config:
    SECRET_KEY = os.environ.get('SECRET_KEY')  # Must be set in production

    # Session cookies
    SESSION_COOKIE_SECURE = True       # HTTPS only
    SESSION_COOKIE_HTTPONLY = True     # No JavaScript access
    SESSION_COOKIE_SAMESITE = 'Lax'    # CSRF protection
    PERMANENT_SESSION_LIFETIME = 86400  # 24 hours
```

### Session Regeneration

```python
# After login, regenerate session to prevent fixation
from flask import session
from flask_login import login_user

def login_user_safely(user):
    session.clear()  # Clear old session
    login_user(user, remember=True)
```

---

## Security Headers

```python
# app/__init__.py or middleware

@app.after_request
def add_security_headers(response):
    # Prevent clickjacking
    response.headers['X-Frame-Options'] = 'SAMEORIGIN'

    # Prevent MIME sniffing
    response.headers['X-Content-Type-Options'] = 'nosniff'

    # XSS filter (legacy browsers)
    response.headers['X-XSS-Protection'] = '1; mode=block'

    # Referrer policy
    response.headers['Referrer-Policy'] = 'strict-origin-when-cross-origin'

    # HSTS (only in production with HTTPS)
    if not app.debug:
        response.headers['Strict-Transport-Security'] = \
            'max-age=31536000; includeSubDomains'

    return response
```

---

## Secrets Management

### Development

```bash
# config/local.env
SECRET_KEY=dev-secret-key
DATABASE_URL=sqlite:///app.db
```

### Production

**Never commit secrets to git.**

```bash
# Environment variables (set in deployment platform)
export SECRET_KEY="$(python -c 'import secrets; print(secrets.token_hex(32))')"
export DATABASE_URL="postgresql://..."
```

### Generating Secrets

```python
import secrets

# Generate secure secret key
secret_key = secrets.token_hex(32)

# Generate API key
api_key = secrets.token_urlsafe(32)
```

### .gitignore

```
# Secrets
.env
*.env
config/local.env
!config/local.env.example

# Keys
*.pem
*.key
credentials.json
```

---

**Next:** [Testing](testing.md) | [Deployment](deployment.md)
