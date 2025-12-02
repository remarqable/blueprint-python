# Frontend Patterns

> Complete guide to frontend development in Blueprint Go using HTMX, minimal Alpine.js, and Bootstrap.

---

## Philosophy

### Core Principles

1. **Server-rendered HTML first** - Use html/template for all rendering
2. **HTMX for interactivity** - Server-driven updates without page reloads
3. **CSS-first animations** - Use CSS transitions/animations, not JavaScript
4. **Alpine.js minimally** - Only when CSS/HTML cannot achieve the goal
5. **Locality of behavior** - Keep CSS close to where it's used
6. **No build step** - Use CDN versions for simplicity

### When to Use Alpine.js vs CSS

| Task | Use CSS | Use Alpine.js |
|------|---------|---------------|
| Show/hide on hover | ✅ `:hover` | ❌ |
| Fade in/out | ✅ `transition` | ❌ |
| Slide animations | ✅ `transform` | ❌ |
| Rotate/scale | ✅ `transform` | ❌ |
| Toggle visibility | ❌ | ✅ `x-show` |
| Conditional logic | ❌ | ✅ `x-if` |
| Form state | ❌ | ✅ `x-model` |
| Complex interactions | ❌ | ✅ |

**Rule of thumb:** If you can do it with CSS, do it with CSS.

---

## Tech Stack

| Library | Purpose | CDN |
|---------|---------|-----|
| Bootstrap 5 | CSS framework | bootstrap.min.css, bootstrap.bundle.min.js |
| HTMX | Server-driven updates | htmx.min.js |
| Alpine.js | Reactive components (minimal use) | alpine.min.js |
| FontAwesome 6 | Icons | fontawesome/css/all.min.css |

---

## CSS-First Animations

### Hover Effects (No JS Needed)

```html
<!-- Button with hover effect -->
<style>
.btn-hover {
    transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.btn-hover:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}
</style>

<button class="btn btn-primary btn-hover">
    Hover Me
</button>
```

### Fade Transitions

```html
<!-- Fade in on load -->
<style>
@keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
}
.fade-in {
    animation: fadeIn 0.3s ease forwards;
}
</style>

<div class="fade-in">
    Content fades in on page load
</div>
```

### Slide Down Menu (CSS Only)

```html
<style>
.dropdown-menu-css {
    max-height: 0;
    overflow: hidden;
    transition: max-height 0.3s ease, opacity 0.3s ease;
    opacity: 0;
}
.dropdown:hover .dropdown-menu-css,
.dropdown:focus-within .dropdown-menu-css {
    max-height: 300px;
    opacity: 1;
}
</style>

<div class="dropdown">
    <button class="btn btn-secondary">Options</button>
    <ul class="dropdown-menu-css">
        <li><a href="#">Edit</a></li>
        <li><a href="#">Delete</a></li>
    </ul>
</div>
```

### Accordion (CSS Only with Details/Summary)

```html
<style>
details summary {
    cursor: pointer;
    padding: 0.75rem 1rem;
    background: #f8f9fa;
    border-radius: 4px;
}
details[open] summary {
    border-radius: 4px 4px 0 0;
}
details .content {
    padding: 1rem;
    border: 1px solid #dee2e6;
    border-top: none;
    border-radius: 0 0 4px 4px;
}
</style>

<details>
    <summary>Click to expand</summary>
    <div class="content">
        Hidden content here
    </div>
</details>
```

### Loading Spinner (CSS Animation)

```html
<style>
.spinner {
    width: 20px;
    height: 20px;
    border: 2px solid #f3f3f3;
    border-top: 2px solid #3498db;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}
@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}
</style>

<div class="spinner"></div>
```

### Card Hover Animation

```html
<style>
.card-hover {
    transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.card-hover:hover {
    transform: translateY(-4px);
    box-shadow: 0 8px 25px rgba(0,0,0,0.1);
}
</style>

<div class="card card-hover">
    <div class="card-body">
        Hover to lift
    </div>
</div>
```

---

## Locality of Behavior

### Inline Styles for Component-Specific CSS

Keep CSS in the same file where it's used:

```html
<!-- modules/tasks/views/templates/tasks/index.html -->
{% extends "base.html" %}

{% block styles %}
<style>
/* Task-specific styles - only used in this template */
.task-card {
    border-left: 4px solid var(--task-color, #6c757d);
    transition: border-color 0.2s ease;
}
.task-card.priority-high { --task-color: #dc3545; }
.task-card.priority-medium { --task-color: #ffc107; }
.task-card.priority-low { --task-color: #28a745; }

.task-card:hover {
    border-left-width: 6px;
}

.task-complete {
    opacity: 0.6;
    text-decoration: line-through;
}
</style>
{% endblock %}

{% block content %}
<div class="task-card priority-high">
    High priority task
</div>
{% endblock %}
```

### When to Extract to CSS File

Only extract CSS when used in **3+ places**:

```
modules/core/views/assets/css/
├── common.css      # Used everywhere (base styles)
└── buttons.css     # Shared button variations

modules/tasks/views/assets/css/
└── (empty - use inline styles)
```

### Scoped Styles Pattern

```html
<!-- Use unique prefixes for component styles -->
<style>
/* Prefix with module/component name */
.tasks-list { }
.tasks-card { }
.tasks-badge { }
.tasks-filter { }

/* Not global names that could conflict */
.list { }     /* ❌ Too generic */
.card { }     /* ❌ Conflicts with Bootstrap */
</style>
```

---

## Minimal Alpine.js

### When Alpine.js IS Required

Only use Alpine when you need:
1. **State that toggles** - Show/hide that can't use CSS `:hover`
2. **Form state** - Track input values before submit
3. **Conditional rendering** - Different UI based on data
4. **Event handling** - Complex click sequences

### Toggle Visibility (Alpine Required)

```html
<!-- Can't do this with pure CSS - needs click toggle -->
<div x-data="{ open: false }">
    <button @click="open = !open">
        {{T "Toggle"}}
    </button>
    <div x-show="open" x-transition>
        Content shown/hidden on click
    </div>
</div>
```

### Form Validation State

```html
<!-- Need to track form state before submit -->
<form x-data="{ valid: false }" @submit.prevent="valid && $el.submit()">
    <input type="email"
           required
           @input="valid = $el.checkValidity()">
    <button type="submit" :disabled="!valid">
        Submit
    </button>
</form>
```

### When NOT to Use Alpine.js

```html
<!-- ❌ DON'T: Alpine for hover effects -->
<div x-data="{ hover: false }"
     @mouseenter="hover = true"
     @mouseleave="hover = false">
    <div :class="hover ? 'visible' : 'hidden'">Tooltip</div>
</div>

<!-- ✅ DO: CSS hover instead -->
<style>
.tooltip-trigger .tooltip { opacity: 0; transition: opacity 0.2s; }
.tooltip-trigger:hover .tooltip { opacity: 1; }
</style>
<div class="tooltip-trigger">
    Hover me
    <div class="tooltip">Tooltip</div>
</div>
```

```html
<!-- ❌ DON'T: Alpine for simple animations -->
<div x-data="{ show: true }"
     x-transition:enter="transition ease-out duration-300"
     x-show="show">

<!-- ✅ DO: CSS animation instead -->
<style>
.fade-in { animation: fadeIn 0.3s ease; }
</style>
<div class="fade-in">Content</div>
```

---

## HTMX Patterns

### Basic Request

```html
<button hx-get="/items/list"
        hx-target="#item-list"
        hx-swap="innerHTML">
    {{T "Refresh"}}
</button>

<div id="item-list">
    <!-- Content loaded here -->
</div>
```

### Form Submission

```html
<form hx-post="/items/create"
      hx-target="#item-list"
      hx-swap="afterbegin">
    <input type="text" name="name" required>
    <button type="submit">{{T "Add"}}</button>
</form>
```

### Delete with Confirmation

```html
<button hx-delete="/items/{{.item.ID}}"
        hx-target="closest .item-row"
        hx-swap="outerHTML"
        hx-confirm="{{T "Are you sure?"}}">
    {{T "Delete"}}
</button>
```

### Loading Indicator with CSS

```html
<style>
.htmx-request .htmx-indicator { display: inline-block; }
.htmx-indicator { display: none; }
</style>

<button hx-get="/slow-operation" hx-target="#result">
    {{T "Load"}}
    <span class="htmx-indicator spinner"></span>
</button>
```

### Search with Debounce

```html
<input type="search"
       name="q"
       hx-get="/items/search"
       hx-trigger="input changed delay:300ms"
       hx-target="#results"
       placeholder="{{T "Search..."}}">

<div id="results"></div>
```

### Modal Loading

```html
<!-- Trigger -->
<button hx-get="/items/{{.item.ID}}/edit"
        hx-target="#modal-container"
        hx-swap="innerHTML">
    {{T "Edit"}}
</button>

<!-- Container -->
<div id="modal-container"></div>
```

### Modal Template

```html
<!-- modules/yourmodule/views/templates/yourmodule/partials/_edit_modal.html -->
<style>
/* Modal-specific styles - locality of behavior */
.modal-edit .form-control:focus {
    border-color: #0d6efd;
    box-shadow: 0 0 0 0.2rem rgba(13,110,253,.25);
}
</style>

<div class="modal fade show modal-edit" style="display: block;" tabindex="-1">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">{{T "Edit Item"}}</h5>
                <button type="button" class="btn-close"
                        onclick="this.closest('.modal').remove();
                                 document.querySelector('.modal-backdrop').remove()">
                </button>
            </div>
            <form hx-put="/items/{{.item.ID}}"
                  hx-target="#item-{{.item.ID}}"
                  hx-swap="outerHTML">
                <div class="modal-body">
                    <input type="text" name="name" value="{{.item.Name}}"
                           class="form-control" required>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary"
                            onclick="this.closest('.modal').remove();
                                     document.querySelector('.modal-backdrop').remove()">
                        {{T "Cancel"}}
                    </button>
                    <button type="submit" class="btn btn-primary">
                        {{T "Save"}}
                    </button>
                </div>
            </form>
        </div>
    </div>
</div>
<div class="modal-backdrop fade show"></div>
```

---

## Base Template

```html
<!-- modules/core/views/templates/base.html -->
<!DOCTYPE html>
<html lang="{{.lang}}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Blueprint{{end}}</title>

    <!-- Bootstrap CSS -->
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">

    <!-- FontAwesome -->
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" rel="stylesheet">

    <!-- Common animations -->
    <style>
    /* Only universal styles here */
    .htmx-indicator { display: none; }
    .htmx-request .htmx-indicator { display: inline-block; }

    @keyframes fadeIn {
        from { opacity: 0; }
        to { opacity: 1; }
    }
    .fade-in { animation: fadeIn 0.3s ease; }
    </style>

    {{block "styles" .}}{{end}}
</head>
<body hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>
    {{template "header" .}}

    <main class="fade-in">
        {{block "content" .}}{{end}}
    </main>

    {{template "footer" .}}

    <!-- Bootstrap JS -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>

    <!-- HTMX -->
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>

    <!-- Alpine.js (load last, defer) -->
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

    {{block "scripts" .}}{{end}}
</body>
</html>
```

---

## Template Organization

```
modules/yourmodule/views/templates/yourmodule/
├── index.html              # Main list page
├── detail.html             # Detail view
├── form.html               # Create/Edit form page
└── partials/
    ├── _list.html          # List partial for HTMX
    ├── _item.html          # Single item partial
    ├── _form.html          # Form partial
    └── _modal.html         # Modal partial
```

### Naming Conventions

- **Full pages**: `name.html` (extend base.html)
- **Partials**: `_name.html` (prefixed with underscore)
- **Partials folder**: Keep in `partials/` subdirectory

---

## Handler for Partials

```go
func (h *ItemHandler) List(c *gin.Context) {
    items := h.itemRepo.GetAll()

    // Check if HTMX request
    if c.GetHeader("HX-Request") == "true" {
        c.HTML(http.StatusOK, "yourmodule/partials/_list.html", gin.H{
            "items": items,
        })
        return
    }

    c.HTML(http.StatusOK, "yourmodule/index.html", gin.H{
        "items": items,
    })
}
```

---

## HTMX Response Headers

```go
// Trigger client-side event
c.Header("HX-Trigger", "itemCreated")

// Trigger with data (for toasts)
c.Header("HX-Trigger", `{"showToast": {"message": "Item created!", "type": "success"}}`)

// Redirect after action
c.Header("HX-Redirect", "/items")

// Refresh page
c.Header("HX-Refresh", "true")

// Push URL to history
c.Header("HX-Push-Url", "/items/123")
```

---

## Toast Notifications (CSS Animation)

```html
<!-- In base.html -->
<style>
#toast-container {
    position: fixed;
    top: 1rem;
    right: 1rem;
    z-index: 1050;
}
.toast {
    animation: slideIn 0.3s ease;
}
@keyframes slideIn {
    from {
        transform: translateX(100%);
        opacity: 0;
    }
    to {
        transform: translateX(0);
        opacity: 1;
    }
}
.toast.hiding {
    animation: slideOut 0.3s ease forwards;
}
@keyframes slideOut {
    to {
        transform: translateX(100%);
        opacity: 0;
    }
}
</style>

<div id="toast-container"></div>

<script>
// Minimal JS for toast handling - triggered by HTMX events
document.body.addEventListener('showToast', function(e) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast show';
    toast.innerHTML = `
        <div class="toast-header">
            <strong class="me-auto">${e.detail.title || 'Notification'}</strong>
            <button type="button" class="btn-close" onclick="this.closest('.toast').remove()"></button>
        </div>
        <div class="toast-body">${e.detail.message}</div>
    `;
    container.appendChild(toast);
    setTimeout(() => {
        toast.classList.add('hiding');
        setTimeout(() => toast.remove(), 300);
    }, 5000);
});
</script>
```

---

## Icons

Use FontAwesome classes:

```html
<!-- Solid icons -->
<i class="fas fa-check"></i>
<i class="fas fa-times"></i>
<i class="fas fa-edit"></i>
<i class="fas fa-trash"></i>

<!-- With button -->
<button class="btn btn-primary">
    <i class="fas fa-plus me-1"></i> {{T "Add Item"}}
</button>
```

---

## Best Practices Summary

### DO

1. **Use CSS for animations** - Transitions, transforms, keyframes
2. **Use HTMX for server interaction** - Forms, updates, modals
3. **Keep styles local** - In template `<style>` blocks
4. **Use Alpine.js sparingly** - Only for client state management
5. **Prefix component classes** - `.tasks-card` not `.card`
6. **Progressive enhancement** - Work without JS when possible

### DON'T

1. **Don't use Alpine for hover effects** - Use CSS `:hover`
2. **Don't use Alpine for simple animations** - Use CSS `transition`
3. **Don't create global CSS for one-off styles** - Keep inline
4. **Don't add JS when CSS works** - Always try CSS first
5. **Don't use build tools** - Keep it simple with CDN

---

**Next:** [HTMX Patterns](htmx.md) | [Security](security.md)
