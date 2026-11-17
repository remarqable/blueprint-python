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
- [Strict CSP (opt-in)](#strict-csp-opt-in)
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
# uv add flask-limiter redis

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

### Why `unsafe-eval` is in there

Alpine runs the little expressions you write in HTML attributes
(`@click="open = !open"`), which means turning text into code at runtime. That is
what `'unsafe-eval'` permits. Leave it in — without it every dropdown, modal, and
toggle stops working, silently and with no console error.

It is a worse name than it deserves. It only matters if an attacker has *already*
got a script onto your page; it does not make that any more likely. The two
things that actually prevent injection — Jinja escaping every rendered variable,
and `script-src 'self'` blocking outside scripts — are both still in force.

`style-src 'unsafe-inline'` is needed for the same practical reason: Alpine's
`x-show` sets inline styles, and per-tenant branding injects a `<style>` block
([theming.md](../theming.md#branding-without-templates)).

Keep `object-src 'none'`, `base-uri 'self'`, and `frame-ancestors 'none'` — they
cost nothing and close real attacks.

`script-src 'self'` also means **no inline `<script>` blocks** in app templates.
A page that needs a few lines of JavaScript (the admin's page-load timer in
[deployment.md](deployment.md#request-profiling-system-info)) gets a small file
in `static/js/`. The exception is a page the app doesn't serve: Caddy's "be
right back" page is outside the app's CSP, so its inline script is fine.

> Handling raw card numbers or health records, or answering a security
> questionnaire that names `unsafe-eval`? There is a stricter Alpine build that
> avoids it, at the cost of rewriting every component in JavaScript. Switch
> before writing components, not after. See
> [Strict CSP (opt-in)](#strict-csp-opt-in).

---

## Strict CSP (opt-in)

> **Applies only when the project's configuration says `csp: strict`**
> ([Project Configuration](../../CLAUDE.md#-project-configuration)). With
> `csp: relaxed`, the default, nothing in this section applies: the policy in
> [XSS Prevention](#xss-prevention) stays, and every template in this blueprint
> works as written. Skip to [Session Security](#session-security).

The relaxed policy is sound, for the reasons given
[above](#why-unsafe-eval-is-in-there). The strict one removes `'unsafe-eval'`
and `'unsafe-inline'` as well, and the price is that every Alpine component is
written in a JavaScript file instead of in the markup. Choose it for a new
project that can pay that from the first template, or for one facing a security
review that names either keyword. It is one or the other: a template written
for relaxed does nothing under strict.

### The strict policy

```
default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'
```

`img-src` is the same as in relaxed: `https:` stays because an avatar URL can
point at any host ([mvc.md](mvc.md#views-templates)). Drop it once every image
is served by the app.

### Selecting the header

One module holds both policies, and the project's answer picks one. It replaces
the `Content-Security-Policy` line in `add_security_headers` above, so the
header is set in one place.

```python
# app/config.py (addition)
class Config:
    CSP = 'strict'        # relaxed | strict: the project's `csp:` answer
```

```python
# app/platform/csp.py -- init_csp(app), called from create_app
RELAXED = (
    "default-src 'self'; script-src 'self' 'unsafe-eval'; "
    "style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; "
    "object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
)
STRICT = (
    "default-src 'self'; script-src 'self'; style-src 'self'; "
    "img-src 'self' data: https:; font-src 'self'; connect-src 'self'; "
    "object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
)
POLICIES = {'relaxed': RELAXED, 'strict': STRICT}


def init_csp(app):
    policy = POLICIES[app.config['CSP']]      # a typo fails at boot, not in a browser

    @app.after_request
    def set_csp(response):
        response.headers['Content-Security-Policy'] = policy
        return response
```

It is a constant in code, not an environment variable: the policy and the
templates it governs ship in the same commit.

### What it forbids

| Forbidden | Example | In the browser |
|-----------|---------|----------------|
| Inline script | `<script>…</script>` in a template | Not run; console error |
| Inline handlers | `onclick="…"`, any `on*=` | Not run; console error |
| Inline style | `style="…"`, a `<style>` block | Not applied; console error |
| `hx-on` and HTMX's other evaluated strings | `hx-on::response-error="…"`, `js:` in `hx-vals`, `hx-trigger="keyup[key=='Enter']"` | Not run |
| Alpine inline expressions | `x-data="{ open: false }"`, `@click="open = !open"` | The element does nothing; console warning |

Still allowed, because none of it is inline code: `<script src>` and
`<link rel="stylesheet">` from `/static`, `hx-headers` and `hx-vals` holding
JSON, `data-*` attributes, and styles set from JavaScript through the DOM
(which is how Alpine's `x-show` and `x-transition` work).

### What changes in this blueprint

Each of these is written for relaxed. With `csp: strict`, write it this way
instead:

- **Every Alpine component**: the navbar, flash, modal and toasts in
  [frontend.md](frontend.md#component-patterns), the language switcher in
  [i18n.md](i18n.md#switching-languages), the bottom sheet in
  [mobile-navigation.md](mobile-navigation.md#mobile-bottom-navigation) and the
  Info box in [deployment.md](deployment.md#the-info-link). Use the CSP build
  and named components:
  [Alpine with a Strict CSP](frontend.md#alpine-with-a-strict-csp-opt-in).
- **Toasts** ([htmx.md](htmx.md#toast-notifications)): the `@toast.window`
  handler becomes the `toasts` component. The server side is unchanged.
- **HTMX's indicator styles.** HTMX injects a `<style>` element at load, which
  `style-src 'self'` blocks, and the `<style>` block in
  [htmx.md](htmx.md#loading-states) is blocked too. Add this to the layout's
  `<head>` and move the two `.htmx-indicator` rules into `input.css`:

  ```html
  <!-- app/views/layouts/base.html -->
  <meta name="htmx-config" content='{"includeIndicatorStyles":false,"allowEval":false}'>
  ```

  `allowEval: false` makes HTMX refuse `hx-on`, `js:` values and trigger
  filters itself, so a stray one is reported rather than half-working.
- **`hx-on::response-error="alert(…)"`** ([htmx.md](htmx.md#error-handling)):
  delete it. The global `htmx:responseError` listener shown beside it goes in
  `app/static/js/app.js` and raises a toast.
- **`onclick="history.back()"`**
  ([mobile-navigation.md](mobile-navigation.md#mobile-header)): the button
  carries `data-back` and one listener in `app.js` handles every such button.
- **The Info box** ([deployment.md](deployment.md#the-info-link)):
  `x-data="{ open: false }"` and `@click="open = true"` become
  `x-data="disclosure"` and `@click="show"`. `sysinfo.js` is already a static
  file and sets text and a class through the DOM, so it is unchanged.
- **The "be right back" page**
  ([deployment.md](deployment.md#be-right-back-page)): unchanged. Caddy serves
  it, so its inline script and CSS are outside the app's policy.
- **Per-tenant branding** (`theming: true` only): the `<style>` block in
  [theming.md](../theming.md#branding-without-templates) is blocked. Serve the
  same variables from a route and link it after `app.css`:

  ```python
  # app/controllers/main.py
  @bp.route('/brand.css')
  def brand_css():
      color = g.org.brand_primary if g.org else None      # validated #RRGGBB on write
      css = (f':root{{--color-brand-500:{color};--color-brand-600:{color};'
             f'--color-brand-700:{color}}}') if color else ''
      return Response(css, mimetype='text/css', headers={'Cache-Control': 'no-cache'})
  ```
- **Data for a script**: put it in a `data-*` attribute and read it in
  `app.js`. Never write a `<script>` block to pass a value.
- **OAuth** ([auth.md](auth.md#oauth-optional)): start the flow with a link,
  not a POST form. `form-action 'self'` blocks a form whose response redirects
  to another origin.

### Moving an existing project

Send the strict policy as `Content-Security-Policy-Report-Only` beside the
relaxed header. Nothing is blocked, and the browser console lists everything
strict would block. Convert the templates, then change `CSP` and remove the
report-only header in the same commit that swaps the Alpine build.

The checks that keep it converted are in
[frontend.md](frontend.md#testing-the-strict-rules).

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

Diagnostic headers are not security headers, and they leak: `Server-Timing`
(server and database time, query counts) shows anyone how the app is built and
where it's slow. Send it **only to platform admins**, as the request profiling in
[deployment.md](deployment.md#request-profiling-system-info) does; an
`X-Request-Id` is harmless and helps support, so every response can carry it.

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
