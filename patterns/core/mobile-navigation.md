# Mobile Navigation Patterns

> Device-aware navigation with separate mobile/desktop templates.

---

## Table of Contents

- [Philosophy](#philosophy)
- [Device Detection](#device-detection)
- [Template Structure](#template-structure)
- [Mobile Base Layout](#mobile-base-layout)
- [Navigation Components](#navigation-components)
- [Page Patterns](#page-patterns)
- [CSS Variables](#css-variables)
- [Controller Integration](#controller-integration)
- [Testing](#testing)

---

## Philosophy

GlossedOver uses **device-aware rendering** with separate mobile and desktop templates:

1. **Detect device** via User-Agent (with query param override for testing)
2. **Render appropriate template** using `render_device_template()`
3. **Mobile-first content** - same data, optimized layout
4. **Progressive enhancement** - HTMX works on both

**Key rules:**

- Mobile templates are optional - falls back to desktop if missing
- Same controller logic, different presentation
- Use Bootstrap responsive classes for minor differences
- Create separate mobile template only when layout differs significantly

---

## Device Detection

### How It Works

Device detection lives in `app/platform/devices/`:

```
app/platform/devices/
├── __init__.py       # Exports all functions
├── detection.py      # User-Agent parsing + session management
└── template.py       # render_device_template() helper
```

### Detection Priority

1. **Query parameter** (`?device=mobile` or `?device=desktop`) - for testing
2. **Session override** - user preference via `set_device_type()`
3. **User-Agent detection** - automatic parsing

### Available Functions

```python
from app.platform.devices import (
    detect_device,           # Parse User-Agent → 'mobile' | 'desktop'
    get_device_type,         # Get with priority logic
    is_mobile,               # Boolean helper
    set_device_type,         # Manual override (saves to session)
    clear_device_type,       # Clear session override
    render_device_template,  # Template helper
)
```

### Template Context

Device info is automatically injected into all templates:

```python
# In app/__init__.py context processor
{
    'device_type': 'mobile' | 'desktop',
    'is_mobile': True | False,
}
```

---

## Template Structure

### Directory Convention

```
app/views/
├── layouts/
│   ├── base.html              # Desktop base (existing)
│   └── mobile_base.html       # Mobile base (new)
├── partials/
│   ├── _navbar.html           # Desktop navbar
│   ├── _footer.html           # Desktop footer
│   └── _mobile_nav.html       # Mobile bottom nav (new)
├── profiles/
│   ├── desktop/
│   │   └── profile.html       # Desktop profile page
│   └── mobile/
│       └── profile.html       # Mobile profile page
├── dashboard/
│   ├── desktop/
│   │   └── index.html         # Desktop dashboard
│   └── mobile/
│       └── index.html         # Mobile dashboard
└── directory/
    ├── desktop/
    │   └── results.html       # Desktop search results
    └── mobile/
        └── results.html       # Mobile search results
```

### Template Resolution

`render_device_template()` automatically swaps `desktop/` for `mobile/`:

```python
# Controller calls:
render_device_template('profiles/desktop/profile.html', stylist=stylist)

# On mobile device, looks for:
# 1. profiles/mobile/profile.html (if exists, use it)
# 2. profiles/desktop/profile.html (fallback)
```

---

## Mobile Base Layout

### `layouts/mobile_base.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
    <meta name="apple-mobile-web-app-capable" content="yes">
    <meta name="apple-mobile-web-app-status-bar-style" content="default">
    <title>{% block title %}GlossedOver{% endblock %}</title>

    <!-- Bootstrap -->
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.1/font/bootstrap-icons.css" rel="stylesheet">

    <!-- App CSS -->
    <link rel="stylesheet" href="{{ url_for('static', filename='css/app.css') }}">
    <link rel="stylesheet" href="{{ url_for('static', filename='css/mobile.css') }}">

    {% block head %}{% endblock %}
</head>
<body class="mobile-body {% block body_class %}{% endblock %}">
    <!-- Mobile Header -->
    {% block mobile_header %}
    <header class="mobile-header">
        <div class="mobile-header-content">
            {% block header_left %}
            <a href="{{ url_for('main.index') }}" class="mobile-header-back">
                <i class="bi bi-chevron-left"></i>
            </a>
            {% endblock %}

            <h1 class="mobile-header-title">
                {% block header_title %}GlossedOver{% endblock %}
            </h1>

            {% block header_right %}
            <div class="mobile-header-action"></div>
            {% endblock %}
        </div>
    </header>
    {% endblock %}

    <!-- Flash Messages -->
    {% with messages = get_flashed_messages(with_categories=true) %}
        {% for category, message in messages %}
        <div class="mobile-toast alert alert-{{ 'danger' if category == 'error' else category }}">
            {{ message }}
        </div>
        {% endfor %}
    {% endwith %}

    <!-- Main Content -->
    <main class="mobile-main">
        {% block content %}{% endblock %}
    </main>

    <!-- Mobile Bottom Navigation -->
    {% block mobile_nav %}
    {% include 'partials/_mobile_nav.html' %}
    {% endblock %}

    <!-- Scripts -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/js/bootstrap.bundle.min.js"></script>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    {% block scripts %}{% endblock %}
</body>
</html>
```

---

## Navigation Components

### Mobile Header

Fixed header with back button, title, and optional action:

```html
<header class="mobile-header">
    <div class="mobile-header-content">
        <a href="javascript:history.back()" class="mobile-header-back">
            <i class="bi bi-chevron-left"></i>
        </a>
        <h1 class="mobile-header-title">Profile</h1>
        <button class="mobile-header-action">
            <i class="bi bi-share"></i>
        </button>
    </div>
</header>
```

### Mobile Bottom Navigation

Fixed bottom nav for primary actions:

```html
<!-- partials/_mobile_nav.html -->
<nav class="mobile-bottom-nav">
    <a href="{{ url_for('main.index') }}"
       class="mobile-nav-item {% if request.endpoint == 'main.index' %}active{% endif %}">
        <i class="bi bi-house"></i>
        <span>Home</span>
    </a>

    <a href="{{ url_for('directory.results') }}"
       class="mobile-nav-item {% if request.endpoint and request.endpoint.startswith('directory') %}active{% endif %}">
        <i class="bi bi-search"></i>
        <span>Find</span>
    </a>

    {% if current_user.is_authenticated %}
    <a href="{{ url_for('dashboard.index') }}"
       class="mobile-nav-item {% if request.endpoint and request.endpoint.startswith('dashboard') %}active{% endif %}">
        <i class="bi bi-person-circle"></i>
        <span>Profile</span>
    </a>
    {% else %}
    <a href="{{ url_for('auth.login') }}"
       class="mobile-nav-item {% if request.endpoint == 'auth.login' %}active{% endif %}">
        <i class="bi bi-box-arrow-in-right"></i>
        <span>Log In</span>
    </a>
    {% endif %}

    <button class="mobile-nav-item" data-bs-toggle="offcanvas" data-bs-target="#mobileMenu">
        <i class="bi bi-list"></i>
        <span>More</span>
    </button>
</nav>

<!-- More Menu (Offcanvas) -->
<div class="offcanvas offcanvas-bottom" tabindex="-1" id="mobileMenu">
    <div class="offcanvas-header">
        <h5 class="offcanvas-title">Menu</h5>
        <button type="button" class="btn-close" data-bs-dismiss="offcanvas"></button>
    </div>
    <div class="offcanvas-body">
        <div class="list-group list-group-flush">
            <a href="{{ url_for('main.faq') }}" class="list-group-item list-group-item-action">
                <i class="bi bi-question-circle me-3"></i> FAQ
            </a>
            {% if current_user.is_authenticated and current_user.is_admin %}
            <a href="{{ url_for('admin.index') }}" class="list-group-item list-group-item-action">
                <i class="bi bi-gear me-3"></i> Admin
            </a>
            {% endif %}
            {% if current_user.is_authenticated %}
            <a href="{{ url_for('auth.logout') }}" class="list-group-item list-group-item-action text-danger">
                <i class="bi bi-box-arrow-right me-3"></i> Log Out
            </a>
            {% endif %}
        </div>
    </div>
</div>
```

### Navigation Rules

| Rule | Guideline |
|------|-----------|
| Max tabs | 4-5 (including More) |
| Icons | Bootstrap Icons |
| Labels | 1 word max |
| Touch target | Minimum 44px height |
| Active state | Uses brand color |

---

## Page Patterns

### Profile Page (Public)

Mobile profile focuses on key info and actions:

```html
{% extends 'layouts/mobile_base.html' %}

{% block title %}{{ stylist.name }} - GlossedOver{% endblock %}

{% block mobile_header %}
<header class="mobile-header mobile-header-transparent">
    <div class="mobile-header-content">
        <a href="{{ url_for('directory.results') }}" class="mobile-header-back">
            <i class="bi bi-chevron-left"></i>
        </a>
        <h1 class="mobile-header-title"></h1>
        <button class="mobile-header-action" onclick="shareProfile()">
            <i class="bi bi-share"></i>
        </button>
    </div>
</header>
{% endblock %}

{% block content %}
<div class="profile-mobile">
    <!-- Hero Image -->
    <div class="profile-hero">
        {% if stylist.avatar_url %}
        <img src="{{ stylist.avatar_url }}" alt="{{ stylist.name }}" class="profile-hero-image">
        {% endif %}
    </div>

    <!-- Profile Info Card -->
    <div class="profile-info-card">
        <h1 class="profile-name">{{ stylist.name }}</h1>
        {% if stylist.location %}
        <p class="profile-location">
            <i class="bi bi-geo-alt"></i> {{ stylist.location }}
        </p>
        {% endif %}

        <!-- Quick Actions -->
        <div class="profile-actions">
            {% if stylist.booking_url %}
            <a href="{{ stylist.booking_url }}" class="btn btn-primary btn-lg w-100" target="_blank">
                Book Appointment
            </a>
            {% endif %}
        </div>
    </div>

    <!-- About Section -->
    {% if stylist.about %}
    <section class="profile-section">
        <h2>About</h2>
        <p>{{ stylist.about }}</p>
    </section>
    {% endif %}

    <!-- Portfolio Grid -->
    {% if stylist.portfolio_images %}
    <section class="profile-section">
        <h2>Portfolio</h2>
        <div class="portfolio-grid-mobile">
            {% for image in stylist.portfolio_images[:6] %}
            <div class="portfolio-item">
                <img src="{{ image.url }}" alt="Portfolio">
            </div>
            {% endfor %}
        </div>
    </section>
    {% endif %}
</div>
{% endblock %}

{% block mobile_nav %}
<!-- Override: Sticky book button instead of nav -->
{% if stylist.booking_url %}
<div class="mobile-sticky-cta">
    <a href="{{ stylist.booking_url }}" class="btn btn-primary btn-lg w-100" target="_blank">
        Book with {{ stylist.name.split()[0] }}
    </a>
</div>
{% else %}
{{ super() }}
{% endif %}
{% endblock %}
```

### Dashboard (Edit Mode)

Mobile dashboard with simplified editing:

```html
{% extends 'layouts/mobile_base.html' %}

{% block header_title %}My Profile{% endblock %}

{% block header_right %}
<a href="{{ url_for('profiles.view', slug=stylist.slug) }}" class="mobile-header-action">
    <i class="bi bi-eye"></i>
</a>
{% endblock %}

{% block content %}
<div class="dashboard-mobile">
    <!-- Profile Completion -->
    <div class="completion-card">
        <div class="completion-header">
            <span>Profile {{ stylist.completion_percent }}% complete</span>
            <span class="badge bg-{{ 'success' if stylist.status == 'published' else 'warning' }}">
                {{ stylist.status|title }}
            </span>
        </div>
        <div class="progress">
            <div class="progress-bar" style="width: {{ stylist.completion_percent }}%"></div>
        </div>
    </div>

    <!-- Edit Sections -->
    <div class="edit-sections">
        <a href="{{ url_for('dashboard.edit_basics') }}" class="edit-section-item">
            <div class="edit-section-icon">
                <i class="bi bi-person"></i>
            </div>
            <div class="edit-section-content">
                <h3>Basic Info</h3>
                <p>Name, photo, location</p>
            </div>
            <i class="bi bi-chevron-right"></i>
        </a>

        <a href="{{ url_for('dashboard.edit_about') }}" class="edit-section-item">
            <div class="edit-section-icon">
                <i class="bi bi-chat-quote"></i>
            </div>
            <div class="edit-section-content">
                <h3>About</h3>
                <p>Bio and specialties</p>
            </div>
            <i class="bi bi-chevron-right"></i>
        </a>

        <a href="{{ url_for('dashboard.edit_portfolio') }}" class="edit-section-item">
            <div class="edit-section-icon">
                <i class="bi bi-images"></i>
            </div>
            <div class="edit-section-content">
                <h3>Portfolio</h3>
                <p>{{ stylist.portfolio_images|length }} photos</p>
            </div>
            <i class="bi bi-chevron-right"></i>
        </a>

        <a href="{{ url_for('dashboard.edit_booking') }}" class="edit-section-item">
            <div class="edit-section-icon">
                <i class="bi bi-calendar-check"></i>
            </div>
            <div class="edit-section-content">
                <h3>Booking</h3>
                <p>Links and availability</p>
            </div>
            <i class="bi bi-chevron-right"></i>
        </a>
    </div>
</div>
{% endblock %}
```

### Directory Search

Mobile-optimized search results:

```html
{% extends 'layouts/mobile_base.html' %}

{% block mobile_header %}
<header class="mobile-header">
    <div class="mobile-header-content">
        <a href="{{ url_for('main.index') }}" class="mobile-header-back">
            <i class="bi bi-chevron-left"></i>
        </a>
        <div class="mobile-search-input">
            <input type="search"
                   name="q"
                   value="{{ query }}"
                   placeholder="Search stylists..."
                   hx-get="{{ url_for('directory.results') }}"
                   hx-trigger="keyup changed delay:300ms"
                   hx-target="#results"
                   hx-push-url="true">
        </div>
        <button class="mobile-header-action" data-bs-toggle="offcanvas" data-bs-target="#filters">
            <i class="bi bi-sliders"></i>
        </button>
    </div>
</header>
{% endblock %}

{% block content %}
<div id="results" class="results-mobile">
    {% for stylist in stylists %}
    <a href="{{ url_for('profiles.view', slug=stylist.slug) }}" class="result-card">
        <img src="{{ stylist.avatar_url or '/static/img/default-avatar.png' }}"
             alt="{{ stylist.name }}"
             class="result-avatar">
        <div class="result-info">
            <h3 class="result-name">{{ stylist.name }}</h3>
            <p class="result-location">{{ stylist.location }}</p>
            {% if stylist.specialties %}
            <div class="result-specialties">
                {% for specialty in stylist.specialties[:3] %}
                <span class="badge bg-light text-dark">{{ specialty }}</span>
                {% endfor %}
            </div>
            {% endif %}
        </div>
        <i class="bi bi-chevron-right result-arrow"></i>
    </a>
    {% else %}
    <div class="empty-state">
        <i class="bi bi-search"></i>
        <p>No stylists found</p>
    </div>
    {% endfor %}
</div>
{% endblock %}

{% block mobile_nav %}{% endblock %}
```

---

## CSS Variables

### `static/css/mobile.css`

```css
/* ===========================================
   Mobile-Specific Styles
   =========================================== */

/* Safe Areas (iOS) */
:root {
    --safe-area-top: env(safe-area-inset-top, 0px);
    --safe-area-bottom: env(safe-area-inset-bottom, 0px);
    --safe-area-left: env(safe-area-inset-left, 0px);
    --safe-area-right: env(safe-area-inset-right, 0px);

    /* Mobile Layout */
    --mobile-header-height: 56px;
    --mobile-nav-height: 56px;

    /* Brand Colors (from app.css) */
    --brand-primary: #e8d5c4;
    --brand-text: #2c2c2c;
}

/* Body */
.mobile-body {
    padding-top: calc(var(--mobile-header-height) + var(--safe-area-top));
    padding-bottom: calc(var(--mobile-nav-height) + var(--safe-area-bottom));
    min-height: 100vh;
    background: var(--bs-gray-100);
}

/* ===========================================
   Mobile Header
   =========================================== */

.mobile-header {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: calc(var(--mobile-header-height) + var(--safe-area-top));
    padding-top: var(--safe-area-top);
    background: #fff;
    border-bottom: 1px solid var(--bs-gray-200);
    z-index: 100;
}

.mobile-header-transparent {
    background: transparent;
    border-bottom: none;
}

.mobile-header-content {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: var(--mobile-header-height);
    padding: 0 0.5rem;
}

.mobile-header-back,
.mobile-header-action {
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--brand-text);
    text-decoration: none;
    border: none;
    background: none;
    border-radius: 50%;
}

.mobile-header-back:active,
.mobile-header-action:active {
    background: var(--bs-gray-200);
}

.mobile-header-title {
    flex: 1;
    text-align: center;
    font-size: 1.125rem;
    font-weight: 600;
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* ===========================================
   Mobile Bottom Navigation
   =========================================== */

.mobile-bottom-nav {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: calc(var(--mobile-nav-height) + var(--safe-area-bottom));
    padding-bottom: var(--safe-area-bottom);
    background: #fff;
    border-top: 1px solid var(--bs-gray-200);
    display: flex;
    z-index: 100;
}

.mobile-nav-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.25rem;
    color: var(--bs-gray-600);
    text-decoration: none;
    font-size: 0.625rem;
    font-weight: 500;
    border: none;
    background: none;
    padding: 0.5rem;
}

.mobile-nav-item i {
    font-size: 1.25rem;
}

.mobile-nav-item.active {
    color: var(--brand-text);
}

.mobile-nav-item:active {
    background: var(--bs-gray-100);
}

/* ===========================================
   Mobile Sticky CTA
   =========================================== */

.mobile-sticky-cta {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 1rem;
    padding-bottom: calc(1rem + var(--safe-area-bottom));
    background: linear-gradient(to top, #fff 80%, transparent);
    z-index: 100;
}

/* ===========================================
   Mobile Toast
   =========================================== */

.mobile-toast {
    position: fixed;
    top: calc(var(--mobile-header-height) + var(--safe-area-top) + 0.5rem);
    left: 0.5rem;
    right: 0.5rem;
    z-index: 200;
    animation: slideDown 0.3s ease;
}

@keyframes slideDown {
    from {
        transform: translateY(-100%);
        opacity: 0;
    }
    to {
        transform: translateY(0);
        opacity: 1;
    }
}

/* ===========================================
   Mobile Search Input
   =========================================== */

.mobile-search-input {
    flex: 1;
    margin: 0 0.5rem;
}

.mobile-search-input input {
    width: 100%;
    height: 36px;
    padding: 0 1rem;
    border: none;
    border-radius: 18px;
    background: var(--bs-gray-100);
    font-size: 0.875rem;
}

.mobile-search-input input:focus {
    outline: none;
    background: var(--bs-gray-200);
}

/* ===========================================
   Mobile Content Patterns
   =========================================== */

.mobile-main {
    min-height: calc(100vh - var(--mobile-header-height) - var(--mobile-nav-height));
}

/* Profile Hero */
.profile-hero {
    height: 300px;
    background: var(--bs-gray-200);
    margin: calc(-1 * var(--mobile-header-height) - var(--safe-area-top)) -0.75rem 0;
}

.profile-hero-image {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

/* Profile Info Card */
.profile-info-card {
    background: #fff;
    border-radius: 1.5rem 1.5rem 0 0;
    margin-top: -2rem;
    padding: 1.5rem;
    position: relative;
}

.profile-name {
    font-size: 1.5rem;
    font-weight: 700;
    margin: 0 0 0.25rem;
}

.profile-location {
    color: var(--bs-gray-600);
    margin: 0 0 1rem;
}

.profile-actions {
    margin-top: 1rem;
}

/* Profile Sections */
.profile-section {
    padding: 1.5rem;
    background: #fff;
    border-top: 1px solid var(--bs-gray-200);
}

.profile-section h2 {
    font-size: 1rem;
    font-weight: 600;
    margin: 0 0 1rem;
}

/* Portfolio Grid */
.portfolio-grid-mobile {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.25rem;
}

.portfolio-item img {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
}

/* Results */
.results-mobile {
    padding: 0.5rem;
}

.result-card {
    display: flex;
    align-items: center;
    padding: 1rem;
    background: #fff;
    border-radius: 0.75rem;
    margin-bottom: 0.5rem;
    text-decoration: none;
    color: inherit;
}

.result-card:active {
    background: var(--bs-gray-100);
}

.result-avatar {
    width: 56px;
    height: 56px;
    border-radius: 50%;
    object-fit: cover;
    margin-right: 1rem;
}

.result-info {
    flex: 1;
    min-width: 0;
}

.result-name {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
}

.result-location {
    font-size: 0.875rem;
    color: var(--bs-gray-600);
    margin: 0.25rem 0;
}

.result-specialties {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
}

.result-arrow {
    color: var(--bs-gray-400);
}

/* Dashboard Edit Sections */
.edit-sections {
    padding: 0.5rem;
}

.edit-section-item {
    display: flex;
    align-items: center;
    padding: 1rem;
    background: #fff;
    border-radius: 0.75rem;
    margin-bottom: 0.5rem;
    text-decoration: none;
    color: inherit;
}

.edit-section-item:active {
    background: var(--bs-gray-100);
}

.edit-section-icon {
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--brand-primary);
    border-radius: 0.75rem;
    margin-right: 1rem;
    font-size: 1.25rem;
}

.edit-section-content {
    flex: 1;
}

.edit-section-content h3 {
    font-size: 1rem;
    font-weight: 600;
    margin: 0;
}

.edit-section-content p {
    font-size: 0.875rem;
    color: var(--bs-gray-600);
    margin: 0;
}

/* Completion Card */
.completion-card {
    background: #fff;
    padding: 1rem;
    margin: 0.5rem;
    border-radius: 0.75rem;
}

.completion-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
    font-size: 0.875rem;
}

/* Empty State */
.empty-state {
    text-align: center;
    padding: 3rem 1rem;
    color: var(--bs-gray-500);
}

.empty-state i {
    font-size: 3rem;
    margin-bottom: 1rem;
}

/* ===========================================
   Bootstrap Offcanvas Overrides
   =========================================== */

.offcanvas-bottom {
    height: auto;
    max-height: 50vh;
    border-radius: 1rem 1rem 0 0;
}
```

---

## Controller Integration

### Using `render_device_template()`

```python
# app/controllers/profiles.py
from flask import Blueprint
from app.platform.devices import render_device_template
from app.models import Stylist

bp = Blueprint('profiles', __name__)


@bp.route('/<slug>')
def view(slug):
    """View public profile - device-aware."""
    stylist = Stylist.query.filter_by(slug=slug).first_or_404()

    return render_device_template(
        'profiles/desktop/profile.html',
        stylist=stylist,
    )
```

### Device-Specific Logic (When Needed)

```python
from app.platform.devices import is_mobile

@bp.route('/dashboard')
@login_required
def dashboard():
    """Dashboard with device-specific data."""
    stylist = current_user.stylist

    # Load fewer portfolio images on mobile
    portfolio_limit = 6 if is_mobile() else 20

    return render_device_template(
        'dashboard/desktop/index.html',
        stylist=stylist,
        portfolio_images=stylist.portfolio_images[:portfolio_limit],
    )
```

### Testing Device Modes

Use query parameter to override detection:

```
# View mobile version on desktop
http://localhost:3000/jamie-collins?device=mobile

# View desktop version on mobile
http://localhost:3000/jamie-collins?device=desktop
```

---

## Testing

### Manual Testing Checklist

1. [ ] Test with `?device=mobile` on desktop browser
2. [ ] Test actual mobile device (iPhone, Android)
3. [ ] Verify safe area padding on notched phones
4. [ ] Check bottom nav doesn't overlap content
5. [ ] Verify touch targets are 44px minimum
6. [ ] Test landscape orientation
7. [ ] Verify HTMX works on mobile

### Device Simulation

```bash
# Chrome DevTools
1. Open DevTools (F12)
2. Toggle Device Toolbar (Ctrl+Shift+M)
3. Select device or set dimensions
4. Refresh page
```

### Unit Tests

```python
# tests/test_device_detection.py
import pytest
from app.platform.devices import detect_device


def test_detect_mobile_iphone():
    """iPhone User-Agent detected as mobile."""
    ua = 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)'
    assert detect_device(ua) == 'mobile'


def test_detect_desktop_chrome():
    """Chrome desktop User-Agent detected as desktop."""
    ua = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)'
    assert detect_device(ua) == 'desktop'


def test_query_param_override(client):
    """Query param overrides User-Agent detection."""
    response = client.get('/stylists/results?device=mobile')
    assert response.status_code == 200
    # Verify mobile template was rendered
```

---

## Checklist for New Mobile Templates

1. [ ] Create `mobile/` subdirectory in feature folder
2. [ ] Extend `layouts/mobile_base.html`
3. [ ] Override `{% block header_title %}` with page title
4. [ ] Add header actions if needed (`{% block header_right %}`)
5. [ ] Use Bootstrap mobile utilities
6. [ ] Override `{% block mobile_nav %}` if page needs custom nav
7. [ ] Test with `?device=mobile` query param
8. [ ] Verify touch targets are 44px minimum
9. [ ] Check safe area padding on notched devices
10. [ ] Update controller to use `render_device_template()`

---

**Next:** [Frontend Patterns](frontend.md) | [HTMX Patterns](htmx.md)
