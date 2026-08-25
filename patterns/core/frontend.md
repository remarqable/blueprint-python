# Frontend Architecture

> Bootstrap 5 + HTMX. No build pipeline.

---

## Table of Contents

- [Philosophy](#philosophy)
- [File Structure](#file-structure)
- [Bootstrap Setup](#bootstrap-setup)
- [CSS Customization](#css-customization)
- [Component Patterns](#component-patterns)
- [Responsive Design](#responsive-design)
- [RTL Support](#rtl-support)
- [Accessibility](#accessibility)

---

## Philosophy

### Core Principles

- ✅ **Reuse Bootstrap classes** - Never invent custom classes
- ✅ **One app.css** - Brand overrides only (~50-100 lines)
- ✅ **No build pipeline** - No npm, webpack, vite
- ✅ **HTMX for interactivity** - No custom JavaScript
- ✅ **Progressive enhancement** - Works without JS

### What We Avoid

- ❌ Custom CSS frameworks
- ❌ Tailwind (requires build step)
- ❌ JavaScript frameworks (React, Vue, etc.)
- ❌ CSS-in-JS
- ❌ Sass/Less compilation

---

## File Structure

```
app/static/
├── css/
│   ├── bootstrap.min.css      # Vendored, version-locked
│   ├── bootstrap.rtl.min.css  # RTL version (optional)
│   └── app.css                # Brand overrides (~50 lines)
├── js/
│   ├── htmx.min.js            # Vendored
│   ├── bootstrap.bundle.min.js # Vendored (includes Popper)
│   └── app.js                 # Rarely needed (<20 lines)
└── img/
    ├── logo.svg
    └── favicon.ico
```

### Vendoring Libraries

Download and commit to repo (no CDN dependency):

```bash
# Bootstrap 5.3
curl -o app/static/css/bootstrap.min.css https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css
curl -o app/static/js/bootstrap.bundle.min.js https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/js/bootstrap.bundle.min.js

# HTMX
curl -o app/static/js/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js
```

---

## Bootstrap Setup

### Base Layout

```html
<!-- app/views/layouts/base.html -->
<!DOCTYPE html>
<html lang="{{ lang }}" dir="{{ 'rtl' if is_rtl else 'ltr' }}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{% block title %}Your App{% endblock %}</title>

  <!-- Bootstrap CSS -->
  {% if is_rtl %}
  <link href="/static/css/bootstrap.rtl.min.css" rel="stylesheet">
  {% else %}
  <link href="/static/css/bootstrap.min.css" rel="stylesheet">
  {% endif %}

  <!-- Brand overrides -->
  <link href="/static/css/app.css" rel="stylesheet">

  <!-- HTMX -->
  <script src="/static/js/htmx.min.js"></script>
</head>
<body class="bg-light">
  {% include 'partials/_navbar.html' %}

  <main class="container py-4">
    {% block content %}{% endblock %}
  </main>

  <!-- Bootstrap JS (includes Popper) -->
  <script src="/static/js/bootstrap.bundle.min.js"></script>
</body>
</html>
```

---

## CSS Customization

### app.css Template

```css
/* app/static/css/app.css */
/* Brand overrides only - keep under 100 lines */

/* ============================================
   CSS Variables (Brand Colors)
   ============================================ */
:root {
  --bs-primary: #6366f1;      /* Indigo */
  --bs-primary-rgb: 99, 102, 241;

  /* Typography */
  --bs-font-sans-serif: 'Inter', system-ui, -apple-system, sans-serif;

  /* Border radius */
  --bs-border-radius: 0.5rem;
  --bs-border-radius-lg: 0.75rem;
}

/* ============================================
   Component Tweaks (<5 lines each)
   ============================================ */

/* Primary button uses brand color */
.btn-primary {
  --bs-btn-bg: var(--bs-primary);
  --bs-btn-border-color: var(--bs-primary);
}

/* Navbar brand */
.navbar-brand {
  font-weight: 600;
}

/* Card shadows */
.card {
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

/* ============================================
   HTMX Loading Indicator
   ============================================ */
.htmx-indicator {
  display: none;
}

.htmx-request .htmx-indicator {
  display: inline-block;
}

.htmx-request.htmx-indicator {
  display: inline-block;
}
```

### What NOT to Put in app.css

```css
/* ❌ DON'T: Custom utility classes */
.mt-custom { margin-top: 23px; }

/* ❌ DON'T: Component-specific styles */
.user-card-special { ... }

/* ❌ DON'T: Overriding Bootstrap internals */
.btn { all: unset; }

/* ❌ DON'T: Media queries (use Bootstrap's) */
@media (max-width: 768px) { ... }
```

---

## Component Patterns

### Cards

```html
<div class="card">
  <div class="card-header">
    <h5 class="mb-0">Title</h5>
  </div>
  <div class="card-body">
    <p>Content here</p>
  </div>
  <div class="card-footer text-end">
    <button class="btn btn-primary">Save</button>
  </div>
</div>
```

### Forms

```html
<form>
  <div class="mb-3">
    <label for="email" class="form-label">Email</label>
    <input type="email" class="form-control" id="email" name="email">
    <div class="form-text">We'll never share your email.</div>
  </div>

  <div class="mb-3">
    <label for="theme" class="form-label">Theme</label>
    <select class="form-select" id="theme" name="theme">
      <option value="light">Light</option>
      <option value="dark">Dark</option>
    </select>
  </div>

  <div class="form-check form-switch mb-3">
    <input class="form-check-input" type="checkbox" id="notifications">
    <label class="form-check-label" for="notifications">
      Email notifications
    </label>
  </div>

  <button type="submit" class="btn btn-primary">Save</button>
</form>
```

### Alerts

```html
<!-- Flash messages -->
{% with messages = get_flashed_messages(with_categories=true) %}
  {% for category, message in messages %}
  <div class="alert alert-{{ 'danger' if category == 'error' else category }} alert-dismissible fade show">
    {{ message }}
    <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
  </div>
  {% endfor %}
{% endwith %}
```

### Navbar

```html
<nav class="navbar navbar-expand-lg navbar-light bg-white border-bottom">
  <div class="container">
    <a class="navbar-brand" href="/">Your App</a>

    <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNav">
      <span class="navbar-toggler-icon"></span>
    </button>

    <div class="collapse navbar-collapse" id="navbarNav">
      <ul class="navbar-nav me-auto">
        <li class="nav-item">
          <a class="nav-link" href="/">Home</a>
        </li>
      </ul>

      <ul class="navbar-nav">
        {% if current_user.is_authenticated %}
        <li class="nav-item dropdown">
          <a class="nav-link dropdown-toggle" href="#" data-bs-toggle="dropdown">
            {{ current_user.name }}
          </a>
          <ul class="dropdown-menu dropdown-menu-end">
            <li><a class="dropdown-item" href="/profile">Profile</a></li>
            <li><a class="dropdown-item" href="/settings">Settings</a></li>
            <li><hr class="dropdown-divider"></li>
            <li>
              <form action="/logout" method="POST">
                <button type="submit" class="dropdown-item">Logout</button>
              </form>
            </li>
          </ul>
        </li>
        {% else %}
        <li class="nav-item">
          <a class="nav-link" href="/login">Login</a>
        </li>
        {% endif %}
      </ul>
    </div>
  </div>
</nav>
```

---

## Responsive Design

### Grid System

```html
<!-- Two columns on md+, single column on mobile -->
<div class="row">
  <div class="col-12 col-md-6">Column 1</div>
  <div class="col-12 col-md-6">Column 2</div>
</div>

<!-- Centered content -->
<div class="row justify-content-center">
  <div class="col-12 col-md-8 col-lg-6">
    Centered content
  </div>
</div>
```

### Breakpoints

| Breakpoint | Class infix | Dimensions |
|------------|-------------|------------|
| Extra small | (none) | <576px |
| Small | `sm` | ≥576px |
| Medium | `md` | ≥768px |
| Large | `lg` | ≥992px |
| Extra large | `xl` | ≥1200px |
| XXL | `xxl` | ≥1400px |

### Hide/Show by Breakpoint

```html
<!-- Hide on mobile -->
<div class="d-none d-md-block">Desktop only</div>

<!-- Show only on mobile -->
<div class="d-md-none">Mobile only</div>
```

---

## RTL Support

### Detection

```html
<html lang="{{ lang }}" dir="{{ 'rtl' if is_rtl else 'ltr' }}">
```

### Bootstrap RTL

```html
{% if is_rtl %}
<link href="/static/css/bootstrap.rtl.min.css" rel="stylesheet">
{% else %}
<link href="/static/css/bootstrap.min.css" rel="stylesheet">
{% endif %}
```

### Manual Fixes (if needed)

```css
/* app.css - RTL fixes */
[dir="rtl"] .me-3 {
  margin-left: 1rem !important;
  margin-right: 0 !important;
}
```

---

## Accessibility

### Semantic HTML

```html
<!-- Use semantic elements -->
<nav>...</nav>
<main>...</main>
<footer>...</footer>

<!-- Proper headings hierarchy -->
<h1>Page Title</h1>
<h2>Section</h2>
<h3>Subsection</h3>
```

### ARIA Labels

```html
<!-- Buttons with icons -->
<button class="btn btn-danger" aria-label="Delete item">
  <svg>...</svg>
</button>

<!-- Loading state -->
<button class="btn btn-primary" disabled aria-busy="true">
  <span class="spinner-border spinner-border-sm" aria-hidden="true"></span>
  Loading...
</button>
```

### Focus Management

```html
<!-- Skip link -->
<a href="#main-content" class="visually-hidden-focusable">
  Skip to main content
</a>

<main id="main-content">...</main>
```

### Color Contrast

Use Bootstrap's default colors - they meet WCAG AA standards.

---

**Next:** [HTMX Patterns](htmx.md) | [Security](security.md)
