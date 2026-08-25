# Authentication & Sessions

> Magic links, email + password, session management, and security

---

## Table of Contents

- [Overview](#overview)
- [Magic Links (Passwordless)](#magic-links-passwordless)
- [Email + Password](#email--password)
- [Session Management](#session-management)
- [Flask-Login Integration](#flask-login-integration)
- [Auth Controller](#auth-controller)
- [Protected Routes](#protected-routes)
- [OAuth (Optional)](#oauth-optional)
- [Security Checklist](#security-checklist)

---

## Overview

Two primary strategies. Record the choice as `auth:` in the project
configuration and implement **one** — do not blend them at the start (adding
the other later is straightforward).

**Magic links** (`auth: magic_links`) — the default:
- No password storage/hashing
- Simpler UX for users
- Easy to implement
- **Requires working outbound email.** Every login sends an email. If the
  application must install, authenticate, or operate without an email
  service configured, magic links cannot be the primary strategy.

**Email + password** (`auth: password`):
- Works with zero email infrastructure — no SMTP required to install,
  log in, or administer
- The right choice for self-hostable products, installations behind
  firewalls, and any app whose setup wizard must complete offline
- Costs you password hashing, a change-password flow, and an
  email-independent recovery path (a server-side CLI reset — see below)

Either way, OAuth providers (Google, GitHub) can be layered on later.

---

## Magic Links (Passwordless)

Use when `auth: magic_links`.

### How It Works

1. User enters email on login page
2. Server generates random token, stores with expiry
3. Email sent with magic link: `/auth/magic/<token>`
4. User clicks link → token validated → session created
5. Token marked as "used" (single-use)

### Token Storage

Development: In-memory dictionary
Production: Redis or database table

```python
# In-memory storage (development only)
_magic_links = {}
# Format: {token: {'email': str, 'expires_at': datetime, 'used': bool}}
```

### Token Generation

```python
import secrets
from datetime import timedelta
from app.models.base import utcnow


def create_magic_link(email: str, expiry_minutes: int = 15) -> str:
    """Create a magic link token.

    Args:
        email: User's email address
        expiry_minutes: Token validity period

    Returns:
        Token string (URL-safe)
    """
    token = secrets.token_urlsafe(48)

    _magic_links[token] = {
        'email': email.lower().strip(),
        'expires_at': utcnow() + timedelta(minutes=expiry_minutes),
        'used': False,
    }

    return token


def validate_magic_link(token: str) -> tuple[bool, str]:
    """Validate a magic link token.

    Returns:
        (is_valid, email_or_error_message)
    """
    link_data = _magic_links.get(token)

    if not link_data:
        return False, 'Invalid link'

    if link_data['used']:
        return False, 'Link already used'

    if utcnow() > link_data['expires_at']:
        return False, 'Link expired'

    # Mark as used
    link_data['used'] = True

    return True, link_data['email']
```

### Database Token Storage (Production)

```python
# app/models/magic_link.py
from datetime import timedelta
from app.models.base import utcnow
import secrets
from app.extensions import db
from app.models.base import BaseModel


class MagicLink(BaseModel):
    __tablename__ = 'magic_link'

    token = db.Column(db.String(100), unique=True, nullable=False, index=True)
    email = db.Column(db.String(255), nullable=False)
    expires_at = db.Column(db.DateTime(timezone=True), nullable=False)
    used_at = db.Column(db.DateTime(timezone=True), nullable=True)

    @classmethod
    def create(cls, email: str, expiry_minutes: int = 15) -> 'MagicLink':
        link = cls(
            token=secrets.token_urlsafe(48),
            email=email.lower().strip(),
            expires_at=utcnow() + timedelta(minutes=expiry_minutes),
        )
        return link.save()

    @classmethod
    def validate(cls, token: str) -> tuple[bool, str]:
        link = cls.query.filter_by(token=token).first()

        if not link:
            return False, 'Invalid link'
        if link.used_at:
            return False, 'Link already used'
        if utcnow() > link.expires_at:
            return False, 'Link expired'

        link.used_at = utcnow()
        link.save()

        return True, link.email

    @classmethod
    def cleanup_expired(cls):
        """Remove expired links (run periodically)."""
        cls.query.filter(cls.expires_at < utcnow()).delete()
        db.session.commit()
```

---

## Email + Password

Use when `auth: password`. Nothing here requires outbound email — including
account recovery.

### Hashing

Use Werkzeug's built-in helpers (scrypt by default in current Werkzeug) — no
extra dependency:

```python
# app/models/user.py
from werkzeug.security import generate_password_hash, check_password_hash


class User(BaseModel, UserMixin):
    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False)
    name = db.Column(db.String(100), nullable=False)
    password_hash = db.Column(db.String(255), nullable=False)
    is_active = db.Column(db.Boolean, default=True)

    def set_password(self, password: str) -> None:
        if len(password) < 8:
            raise ValidationError('Password must be at least 8 characters')
        self.password_hash = generate_password_hash(password)

    def check_password(self, password: str) -> bool:
        return check_password_hash(self.password_hash, password)
```

Never store, log, or flash the plaintext password. There is no
`password` column — only `password_hash`.

### Login Controller

```python
# app/controllers/auth.py
@bp.route('/login', methods=['GET', 'POST'])
def login():
    if current_user.is_authenticated:
        return redirect(url_for('users.profile'))

    if request.method == 'POST':
        email = request.form.get('email', '').strip().lower()
        password = request.form.get('password', '')

        user = User.get_by_email(email)

        # One generic failure message. Distinguishing "no such user" from
        # "wrong password" is a user-enumeration oracle.
        if user is None or not user.is_active or not user.check_password(password):
            log.info('login_failed', email=email)
            flash(t('auth.invalid_credentials'), 'error')
            return render_template('auth/login.html'), 401

        session.clear()                  # regenerate: prevents session fixation
        login_user(user, remember=True)
        log.info('user_logged_in', user_id=user.id)
        return redirect(url_for('users.profile'))

    return render_template('auth/login.html')
```

Rate limit this route (see [security.md](security.md)) — password endpoints
are the ones that get brute-forced.

### Password Change

Require the current password before accepting a new one — a hijacked session
must not be enough to lock out the real owner:

```python
@bp.route('/settings/password', methods=['POST'])
@login_required
def change_password():
    current = request.form.get('current_password', '')
    new = request.form.get('new_password', '')
    confirm = request.form.get('confirm_password', '')

    if not current_user.check_password(current):
        flash(t('auth.current_password_wrong'), 'error')
    elif new != confirm:
        flash(t('auth.passwords_do_not_match'), 'error')
    else:
        current_user.set_password(new)
        current_user.save()
        log.info('password_changed', user_id=current_user.id)
        flash(t('auth.password_changed'), 'success')

    return redirect(url_for('settings.index'))
```

### Recovery Without Email: CLI Reset

Email-based "forgot password" only works when email is configured. The
recovery path that always works is a server-side command, runnable by whoever
operates the installation:

```python
# app/controllers/cli.py
import click
import secrets
from flask import Blueprint
from app.models import User

bp = Blueprint('users_cli', __name__, cli_group='users')


@bp.cli.command('reset-password')
@click.argument('email')
def reset_password(email: str):
    """Set a new random password for EMAIL and print it once."""
    user = User.get_by_email(email.strip().lower())
    if user is None:
        raise click.ClickException(f'No user with email {email}')

    password = secrets.token_urlsafe(12)
    user.set_password(password)
    user.save()
    click.echo(f'New password for {user.email}: {password}')
    click.echo('It is shown only once. The user should change it after login.')
```

```bash
flask users reset-password admin@example.com
```

Register the blueprint in `create_app` like any other. If email *is*
configured, an email-based reset flow can be added on top; the CLI path stays
as the operator-level fallback either way.

---

## Session Management

### Flask Session Configuration

```python
# app/config.py
class Config:
    SECRET_KEY = os.environ.get('SECRET_KEY', 'change-in-production')

    # Session cookies
    SESSION_COOKIE_SECURE = os.environ.get('APP_ENV') != 'dev'  # HTTPS only in prod
    SESSION_COOKIE_HTTPONLY = True  # No JavaScript access
    SESSION_COOKIE_SAMESITE = 'Lax'  # CSRF protection
    PERMANENT_SESSION_LIFETIME = 86400  # 24 hours
```

### Session Regeneration

Always regenerate session after login to prevent session fixation:

```python
from flask import session

def login_user_safely(user):
    """Log in user with session regeneration."""
    # Clear old session
    session.clear()

    # Flask-Login handles the rest
    login_user(user, remember=True)
```

---

## Flask-Login Integration

### Setup

```python
# app/extensions.py
from flask_login import LoginManager

login_manager = LoginManager()
login_manager.login_view = 'auth.login'
login_manager.login_message = 'Please log in to access this page.'
login_manager.login_message_category = 'warning'


@login_manager.user_loader
def load_user(user_id):
    from app.models.user import User
    return User.query.get(int(user_id))
```

### User Model Requirements

```python
# app/models/user.py
from flask_login import UserMixin

class User(BaseModel, UserMixin):
    """User model with Flask-Login support.

    UserMixin provides:
    - is_authenticated (property)
    - is_active (property)
    - is_anonymous (property)
    - get_id() (method)
    """
    __tablename__ = 'user'

    email = db.Column(db.String(255), unique=True, nullable=False)
    name = db.Column(db.String(100), nullable=False)
    is_active = db.Column(db.Boolean, default=True)
```

---

## Auth Controller

Magic-link variant shown (`auth: magic_links`); the password variant of
`/login` is in [Email + Password](#email--password) above.

```python
# app/controllers/auth.py
"""Authentication controller."""

import secrets
from datetime import timedelta
from app.models.base import utcnow
from flask import Blueprint, render_template, request, redirect, url_for, flash, session, current_app
from flask_login import login_user, logout_user, current_user
from app.models import User
from app.platform.logger import get_logger
from app.platform.i18n import t

bp = Blueprint('auth', __name__)
log = get_logger()

# In-memory magic links (use database in production)
_magic_links = {}


@bp.route('/login', methods=['GET', 'POST'])
def login():
    """Show login form and handle magic link request."""
    if current_user.is_authenticated:
        return redirect(url_for('users.profile'))

    if request.method == 'POST':
        email = request.form.get('email', '').strip().lower()

        if not email:
            flash(t('auth.email_required'), 'error')
            return render_template('auth/login.html')

        user = User.get_by_email(email)

        # Dev mode: auto-create user
        if user is None and current_app.config.get('DEV_MAGIC'):
            user = User.create(email=email, name=email.split('@')[0])
            log.info('dev_user_created', email=email)

        if user:
            token = _create_magic_link(email)

            if current_app.config.get('DEV_MAGIC'):
                # Show link in dev mode
                magic_url = url_for('auth.verify_magic', token=token, _external=True)
                flash(f'Dev mode - Magic link: {magic_url}', 'info')
            else:
                # Send email in production
                _send_magic_email(email, token)

        # Always show success (don't reveal if email exists)
        flash(t('auth.magic_link_sent'), 'success')
        return render_template('auth/login.html')

    return render_template('auth/login.html')


@bp.route('/auth/magic/<token>')
def verify_magic(token: str):
    """Verify magic link and log user in."""
    link_data = _magic_links.get(token)

    if not link_data:
        flash(t('auth.invalid_link'), 'error')
        return redirect(url_for('auth.login'))

    if link_data['used']:
        flash(t('auth.link_already_used'), 'error')
        return redirect(url_for('auth.login'))

    if utcnow() > link_data['expires_at']:
        flash(t('auth.link_expired'), 'error')
        return redirect(url_for('auth.login'))

    # Mark as used
    link_data['used'] = True

    # Get user
    user = User.get_by_email(link_data['email'])
    if not user:
        flash(t('auth.user_not_found'), 'error')
        return redirect(url_for('auth.login'))

    # Login with session regeneration
    session.clear()
    login_user(user, remember=True)

    log.info('user_logged_in', user_id=user.id)
    flash(t('auth.welcome_back', name=user.name), 'success')

    return redirect(url_for('users.profile'))


@bp.route('/logout', methods=['POST'])
def logout():
    """Log out the current user."""
    if current_user.is_authenticated:
        log.info('user_logged_out', user_id=current_user.id)

    logout_user()
    session.clear()

    flash(t('auth.logged_out'), 'success')
    return redirect(url_for('main.index'))


def _create_magic_link(email: str) -> str:
    """Create magic link token."""
    token = secrets.token_urlsafe(48)

    _magic_links[token] = {
        'email': email,
        'expires_at': utcnow() + timedelta(minutes=15),
        'used': False,
    }

    return token


def _send_magic_email(email: str, token: str):
    """Send magic link email (implement in production)."""
    # TODO: Implement email sending
    pass
```

---

## Protected Routes

### Using Decorators

```python
from flask_login import login_required, current_user

@bp.route('/profile')
@login_required
def profile():
    """Protected route - requires authentication."""
    return render_template('users/profile.html', user=current_user)
```

### API Endpoints

```python
# app/middleware/auth.py
from functools import wraps
from flask import jsonify
from flask_login import current_user


def login_required_api(f):
    """Decorator for API routes - returns JSON instead of redirect."""
    @wraps(f)
    def decorated(*args, **kwargs):
        if not current_user.is_authenticated:
            return jsonify({'error': 'Authentication required'}), 401
        return f(*args, **kwargs)
    return decorated


# Usage
@bp.route('/api/user')
@login_required_api
def api_user():
    return jsonify(current_user.to_dict())
```

---

## OAuth (Optional)

### Google OAuth Setup

```python
# uv add flask-dance

from flask_dance.contrib.google import make_google_blueprint, google

google_bp = make_google_blueprint(
    client_id=os.environ.get('GOOGLE_CLIENT_ID'),
    client_secret=os.environ.get('GOOGLE_CLIENT_SECRET'),
    scope=['openid', 'email', 'profile'],
)


@google_bp.route('/authorized')
def google_authorized():
    if not google.authorized:
        return redirect(url_for('google.login'))

    resp = google.get('/oauth2/v2/userinfo')
    if resp.ok:
        info = resp.json()
        email = info['email']
        name = info.get('name', email.split('@')[0])

        # Find or create user
        user = User.get_by_email(email)
        if not user:
            user = User.create(email=email, name=name)

        login_user(user)
        return redirect(url_for('users.profile'))

    return redirect(url_for('auth.login'))
```

---

## Security Checklist

### Session Security

- [x] `SESSION_COOKIE_HTTPONLY = True` (no JS access)
- [x] `SESSION_COOKIE_SECURE = True` in production (HTTPS only)
- [x] `SESSION_COOKIE_SAMESITE = 'Lax'` (CSRF protection)
- [x] Regenerate session after login
- [x] Clear session on logout

### Magic Link Security (`auth: magic_links`)

- [x] Use `secrets.token_urlsafe(48)` for tokens
- [x] Short expiry (15 minutes)
- [x] Single-use tokens
- [x] Don't reveal if email exists
- [x] Clean up expired tokens

### Password Security (`auth: password`)

- [x] Werkzeug `generate_password_hash` / `check_password_hash` — never roll your own
- [x] Only `password_hash` is ever stored; plaintext never logged or flashed
- [x] One generic failure message — no user-enumeration oracle
- [x] Current password required to change password
- [x] CLI reset command as the email-independent recovery path
- [x] Rate limit `/login`

### General

- [x] Rate limit login attempts
- [x] Log auth events
- [x] HTTPS in production

---

## Time Handling

All expiry timestamps use `utcnow()` from `app/models/base.py`, and all datetime
columns are declared `DateTime(timezone=True)`.

Mixing the two conventions is a live bug rather than a style question: comparing
a naive `datetime.utcnow()` against a timezone-aware column raises
`TypeError: can't compare offset-naive and offset-aware datetimes` on PostgreSQL,
while SQLite silently compares them wrong and expires tokens at the offset
between your server's clock and UTC. See
[portability.md](portability.md#timestamps).

---

**Next:** [Security](security.md) | [MVC Pattern](mvc.md)
