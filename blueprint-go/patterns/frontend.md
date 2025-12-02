# Frontend Patterns

> Complete guide to frontend development in Blueprint Go using HTMX, Alpine.js, and Bootstrap.

---

## Overview

Blueprint uses a **server-rendered HTML** approach:

- **html/template** for server-side rendering
- **HTMX** for dynamic updates without page reloads
- **Alpine.js** for reactive client-side components
- **Bootstrap 5** for styling (no build step required)
- **FontAwesome 6** for icons

---

## Tech Stack

| Library | Purpose | CDN |
|---------|---------|-----|
| Bootstrap 5 | CSS framework | bootstrap.min.css, bootstrap.bundle.min.js |
| HTMX | Server-driven updates | htmx.min.js |
| Alpine.js | Reactive components | alpine.min.js |
| FontAwesome 6 | Icons | fontawesome/css/all.min.css |

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

    {{block "styles" .}}{{end}}
</head>
<body>
    {{template "header" .}}

    <main>
        {{block "content" .}}{{end}}
    </main>

    {{template "footer" .}}

    <!-- Bootstrap JS -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>

    <!-- HTMX -->
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>

    <!-- Alpine.js -->
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

    {{block "scripts" .}}{{end}}
</body>
</html>
```

---

## HTMX Patterns

### Basic Request

```html
<!-- Load content into element -->
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

### Loading Indicators

```html
<button hx-get="/slow-operation"
        hx-target="#result"
        hx-indicator="#spinner">
    {{T "Load Data"}}
</button>

<span id="spinner" class="htmx-indicator">
    <i class="fas fa-spinner fa-spin"></i>
</span>
```

### Modal Loading

```html
<!-- Trigger -->
<button hx-get="/items/{{.item.ID}}/edit"
        hx-target="#modal-container"
        hx-swap="innerHTML">
    {{T "Edit"}}
</button>

<!-- Container for modals -->
<div id="modal-container"></div>
```

### Partial Template (Modal)

```html
<!-- modules/yourmodule/views/templates/yourmodule/partials/_edit_modal.html -->
<div class="modal fade show" style="display: block;" tabindex="-1">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">{{T "Edit Item"}}</h5>
                <button type="button" class="btn-close"
                        onclick="this.closest('.modal').remove()"></button>
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
                            onclick="this.closest('.modal').remove()">
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

## Alpine.js Patterns

### Basic Reactivity

```html
<div x-data="{ count: 0 }">
    <button @click="count++">
        Clicked: <span x-text="count"></span>
    </button>
</div>
```

### Toggle Visibility

```html
<div x-data="{ open: false }">
    <button @click="open = !open">
        {{T "Toggle Details"}}
    </button>

    <div x-show="open" x-transition>
        <p>{{T "Hidden content here"}}</p>
    </div>
</div>
```

### Form Validation

```html
<form x-data="{ email: '', valid: false }"
      @submit.prevent="valid && $el.submit()">
    <input type="email"
           x-model="email"
           @input="valid = email.includes('@')"
           required>

    <button type="submit"
            :disabled="!valid"
            :class="valid ? 'btn-primary' : 'btn-secondary'">
        {{T "Submit"}}
    </button>
</form>
```

### Dropdown Menu

```html
<div x-data="{ open: false }" class="dropdown">
    <button @click="open = !open"
            @click.away="open = false"
            class="btn btn-secondary dropdown-toggle">
        {{T "Options"}}
    </button>

    <ul x-show="open" class="dropdown-menu show">
        <li><a class="dropdown-item" href="#">{{T "Edit"}}</a></li>
        <li><a class="dropdown-item" href="#">{{T "Delete"}}</a></li>
    </ul>
</div>
```

### Tab Component

```html
<div x-data="{ tab: 'overview' }">
    <ul class="nav nav-tabs">
        <li class="nav-item">
            <a class="nav-link"
               :class="{ 'active': tab === 'overview' }"
               @click.prevent="tab = 'overview'"
               href="#">{{T "Overview"}}</a>
        </li>
        <li class="nav-item">
            <a class="nav-link"
               :class="{ 'active': tab === 'details' }"
               @click.prevent="tab = 'details'"
               href="#">{{T "Details"}}</a>
        </li>
    </ul>

    <div class="tab-content">
        <div x-show="tab === 'overview'">
            Overview content
        </div>
        <div x-show="tab === 'details'">
            Details content
        </div>
    </div>
</div>
```

---

## Combining HTMX + Alpine.js

### Search with Debounce

```html
<div x-data="{ query: '' }">
    <input type="search"
           x-model="query"
           hx-get="/items/search"
           hx-trigger="input changed delay:300ms"
           hx-target="#results"
           name="q"
           placeholder="{{T "Search..."}}">

    <div id="results"></div>
</div>
```

### Inline Edit

```html
<div x-data="{ editing: false }" class="item">
    <template x-if="!editing">
        <span>
            {{.item.Name}}
            <button @click="editing = true" class="btn btn-sm btn-link">
                <i class="fas fa-edit"></i>
            </button>
        </span>
    </template>

    <template x-if="editing">
        <form hx-patch="/items/{{.item.ID}}"
              hx-target="closest .item"
              hx-swap="outerHTML"
              @htmx:after-request="editing = false">
            <input type="text" name="name" value="{{.item.Name}}" class="form-control">
            <button type="submit" class="btn btn-primary btn-sm">{{T "Save"}}</button>
            <button type="button" @click="editing = false" class="btn btn-secondary btn-sm">
                {{T "Cancel"}}
            </button>
        </form>
    </template>
</div>
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
// Return full page for normal request
// Return partial for HTMX request
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

// Redirect after action
c.Header("HX-Redirect", "/items")

// Refresh page
c.Header("HX-Refresh", "true")

// Push URL to history
c.Header("HX-Push-Url", "/items/123")
```

---

## Flash Messages

### Handler

```go
import "github.com/gin-contrib/sessions"

func (h *ItemHandler) Create(c *gin.Context) {
    // ... create item ...

    session := sessions.Default(c)
    session.AddFlash("Item created successfully", "success")
    session.Save()

    c.Redirect(http.StatusFound, "/items")
}
```

### Template

```html
{{range $flash := .flashes}}
<div class="alert alert-{{$flash.Type}} alert-dismissible fade show" role="alert">
    {{$flash.Message}}
    <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
</div>
{{end}}
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

<!-- Regular icons -->
<i class="far fa-calendar"></i>

<!-- Brand icons -->
<i class="fab fa-github"></i>

<!-- With button -->
<button class="btn btn-primary">
    <i class="fas fa-plus me-1"></i> {{T "Add Item"}}
</button>
```

---

## Responsive Design

```html
<div class="container-fluid">
    <div class="row">
        <!-- Sidebar on medium+ screens -->
        <div class="col-md-3 d-none d-md-block">
            {{template "sidebar" .}}
        </div>

        <!-- Main content -->
        <div class="col-12 col-md-9">
            {{block "content" .}}{{end}}
        </div>
    </div>
</div>
```

---

## Best Practices

1. **Partial templates** - Use for HTMX responses
2. **Progressive enhancement** - Work without JS when possible
3. **Loading states** - Show spinners during requests
4. **Error handling** - Display errors gracefully
5. **Accessibility** - Use semantic HTML and ARIA labels
6. **Performance** - Minimize DOM updates
7. **No build step** - Use CDN versions for simplicity
