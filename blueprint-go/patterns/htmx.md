# HTMX Patterns

> Server-rendered interactivity without JavaScript frameworks

---

## Table of Contents

- [Philosophy](#philosophy)
- [Core Concepts](#core-concepts)
- [Common Patterns](#common-patterns)
- [Handler Patterns](#handler-patterns)
- [Form Handling](#form-handling)
- [Toast Notifications](#toast-notifications)
- [Loading States](#loading-states)
- [Error Handling](#error-handling)

---

## Philosophy

### HATEOAS (Hypermedia as the Engine of Application State)

- Server returns HTML, not JSON
- Links and forms drive application state
- Progressive enhancement (works without JS)
- No client-side routing or state management

### When to Use HTMX

✅ **Good for:**
- Form submissions
- Inline editing
- Loading more content
- Search/filter
- Settings updates
- Notifications

❌ **Not ideal for:**
- Real-time collaboration
- Complex drag-and-drop
- Offline-first apps

---

## Core Concepts

### Key Attributes

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `hx-get` | GET request | `hx-get="/users"` |
| `hx-post` | POST request | `hx-post="/settings"` |
| `hx-put` | PUT request | `hx-put="/users/1"` |
| `hx-delete` | DELETE request | `hx-delete="/users/1"` |
| `hx-trigger` | Event to trigger | `hx-trigger="click"` |
| `hx-target` | Where to put response | `hx-target="#result"` |
| `hx-swap` | How to swap content | `hx-swap="innerHTML"` |
| `hx-vals` | Extra values to send | `hx-vals='{"key": "value"}'` |

### Swap Methods

| Method | Description |
|--------|-------------|
| `innerHTML` | Replace inner HTML (default) |
| `outerHTML` | Replace entire element |
| `beforeend` | Append to end |
| `afterbegin` | Prepend to start |
| `delete` | Delete target element |
| `none` | Don't swap (for side effects) |

---

## Common Patterns

### Inline Edit

```html
<!-- Display mode -->
<div id="name-display">
    <span>{{.user.FirstName}}</span>
    <button hx-get="/profile/edit-name"
            hx-target="#name-display"
            hx-swap="outerHTML"
            class="btn btn-sm btn-link">Edit</button>
</div>
```

```html
<!-- Edit mode (returned by server) -->
<form id="name-display"
      hx-post="/profile/name"
      hx-target="#name-display"
      hx-swap="outerHTML">
    <input type="text" name="first_name" value="{{.user.FirstName}}" class="form-control">
    <button type="submit" class="btn btn-primary btn-sm">Save</button>
    <button type="button"
            hx-get="/profile/name-display"
            hx-target="#name-display"
            hx-swap="outerHTML"
            class="btn btn-secondary btn-sm">Cancel</button>
</form>
```

### Search with Debounce

```html
<input type="search"
       name="q"
       placeholder="Search..."
       hx-get="/search"
       hx-trigger="keyup changed delay:300ms"
       hx-target="#search-results"
       class="form-control">

<div id="search-results">
    <!-- Results appear here -->
</div>
```

### Infinite Scroll

```html
<div id="items">
    {{range .items}}
    <div class="item">{{.Name}}</div>
    {{end}}

    {{if .hasMore}}
    <div hx-get="/items?page={{.nextPage}}"
         hx-trigger="revealed"
         hx-swap="outerHTML"
         hx-target="this">
        <span class="spinner-border spinner-border-sm"></span> Loading...
    </div>
    {{end}}
</div>
```

### Auto-Save Settings

```html
<select name="value"
        hx-post="/settings"
        hx-trigger="change"
        hx-vals='{"key": "theme"}'
        hx-target="#theme-status"
        class="form-select">
    <option value="light">Light</option>
    <option value="dark">Dark</option>
</select>
<div id="theme-status" class="form-text text-success"></div>
```

### Delete with Confirmation

```html
<button hx-delete="/items/{{.item.ID}}"
        hx-confirm="Are you sure you want to delete this?"
        hx-target="closest .item"
        hx-swap="outerHTML"
        class="btn btn-danger btn-sm">
    Delete
</button>
```

### Modal Loading

```html
<!-- Trigger button -->
<button hx-get="/items/{{.item.ID}}/edit"
        hx-target="#modal-container"
        hx-swap="innerHTML"
        class="btn btn-primary">
    Edit
</button>

<!-- Modal container -->
<div id="modal-container"></div>
```

```html
<!-- Modal partial (returned by server) -->
<div class="modal fade show" style="display: block;" tabindex="-1">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">Edit Item</h5>
                <button type="button" class="btn-close"
                        onclick="this.closest('.modal').remove()"></button>
            </div>
            <form hx-put="/items/{{.item.ID}}"
                  hx-target="#item-{{.item.ID}}"
                  hx-swap="outerHTML">
                <div class="modal-body">
                    <input type="text" name="name" value="{{.item.Name}}" class="form-control">
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary"
                            onclick="this.closest('.modal').remove()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save</button>
                </div>
            </form>
        </div>
    </div>
</div>
<div class="modal-backdrop fade show"></div>
```

---

## Handler Patterns

### Detecting HTMX Requests

```go
// Helper function
func isHTMXRequest(c *gin.Context) bool {
    return c.GetHeader("HX-Request") == "true"
}

// In handler
func (h *ItemHandler) List(c *gin.Context) {
    items := h.itemRepo.GetAll()

    if isHTMXRequest(c) {
        // Return partial for HTMX
        c.HTML(http.StatusOK, "items/partials/_list.html", gin.H{
            "items": items,
        })
        return
    }

    // Return full page for normal request
    c.HTML(http.StatusOK, "items/index.html", gin.H{
        "items": items,
    })
}
```

### Returning Partials

```go
func (h *SettingsHandler) Update(c *gin.Context) {
    user := auth.GetUser(c)
    key := c.PostForm("key")
    value := c.PostForm("value")

    h.settingRepo.Set(user.ID, key, value)

    // HTMX: return simple confirmation
    if isHTMXRequest(c) {
        c.String(http.StatusOK, "Saved")
        return
    }

    // Normal: redirect
    c.Redirect(http.StatusSeeOther, "/settings")
}
```

### HTMX Response Headers

```go
func (h *ItemHandler) Create(c *gin.Context) {
    name := c.PostForm("name")
    item, err := h.itemRepo.Create(name)
    if err != nil {
        c.String(http.StatusBadRequest, err.Error())
        return
    }

    // Trigger client-side event
    c.Header("HX-Trigger", "itemCreated")

    // Or trigger with data
    c.Header("HX-Trigger", `{"showToast": {"message": "Item created!", "type": "success"}}`)

    c.HTML(http.StatusOK, "items/partials/_item.html", gin.H{
        "item": item,
    })
}

// Other useful headers
c.Header("HX-Redirect", "/items")        // Redirect client
c.Header("HX-Refresh", "true")           // Refresh page
c.Header("HX-Push-Url", "/items/123")    // Push URL to history
```

---

## Form Handling

### Standard Form with HTMX

```html
<form hx-post="/profile/edit"
      hx-target="#form-container"
      hx-swap="outerHTML">
    <input type="hidden" name="csrf_token" value="{{.csrf_token}}">

    <div class="mb-3">
        <label for="name" class="form-label">Name</label>
        <input type="text" name="name" id="name"
               value="{{.user.FirstName}}"
               class="form-control {{if .errors.name}}is-invalid{{end}}">
        {{if .errors.name}}
        <div class="invalid-feedback">{{.errors.name}}</div>
        {{end}}
    </div>

    <button type="submit" class="btn btn-primary">Save</button>
</form>
```

### Form Validation Handler

```go
func (h *UsersHandler) EditProfile(c *gin.Context) {
    user := auth.GetUser(c)
    errors := make(map[string]string)

    if c.Request.Method == http.MethodPost {
        name := strings.TrimSpace(c.PostForm("name"))

        // Validation
        if name == "" {
            errors["name"] = "Name is required"
        } else if len(name) > 100 {
            errors["name"] = "Name too long"
        }

        if len(errors) > 0 {
            // Return form with errors (422 status for HTMX)
            c.HTML(http.StatusUnprocessableEntity, "users/_edit_form.html", gin.H{
                "user":   user,
                "errors": errors,
            })
            return
        }

        // Update and return success
        h.userRepo.Update(user.ID, map[string]interface{}{"first_name": name})
        c.HTML(http.StatusOK, "users/_profile_card.html", gin.H{
            "user": user,
        })
        return
    }

    // GET: show form
    c.HTML(http.StatusOK, "users/_edit_form.html", gin.H{
        "user":   user,
        "errors": errors,
    })
}
```

### Field-Level Validation

```html
<input type="email"
       name="email"
       hx-post="/validate/email"
       hx-trigger="blur"
       hx-target="#email-error"
       class="form-control">
<div id="email-error"></div>
```

```go
func (h *ValidationHandler) ValidateEmail(c *gin.Context) {
    email := c.PostForm("email")

    if email == "" {
        c.String(http.StatusOK, `<span class="text-danger">Email required</span>`)
        return
    }

    if !isValidEmail(email) {
        c.String(http.StatusOK, `<span class="text-danger">Invalid email format</span>`)
        return
    }

    // Check uniqueness
    existing, _ := h.userRepo.GetByEmail(email)
    if existing != nil {
        c.String(http.StatusOK, `<span class="text-danger">Email already registered</span>`)
        return
    }

    c.String(http.StatusOK, `<span class="text-success">Email available</span>`)
}
```

---

## Toast Notifications

### Setup in Base Template

```html
<!-- In base.html -->
<div id="toast-container" class="toast-container position-fixed top-0 end-0 p-3"></div>

<script>
document.body.addEventListener('showToast', function(e) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast show';
    toast.innerHTML = `
        <div class="toast-header">
            <strong class="me-auto">${e.detail.title || 'Notification'}</strong>
            <button type="button" class="btn-close" data-bs-dismiss="toast"></button>
        </div>
        <div class="toast-body">${e.detail.message}</div>
    `;
    container.appendChild(toast);
    setTimeout(() => toast.remove(), 5000);
});
</script>
```

### Triggering from Handler

```go
func (h *ItemHandler) Delete(c *gin.Context) {
    id := c.Param("id")
    h.itemRepo.Delete(parseUint(id))

    c.Header("HX-Trigger", `{"showToast": {"title": "Success", "message": "Item deleted"}}`)
    c.String(http.StatusOK, "")
}
```

---

## Loading States

### Spinner During Request

```html
<button hx-post="/action"
        hx-target="#result"
        class="btn btn-primary">
    <span class="htmx-indicator spinner-border spinner-border-sm me-1"></span>
    Save
</button>

<style>
.htmx-indicator { display: none; }
.htmx-request .htmx-indicator { display: inline-block; }
</style>
```

### Disable During Request

```html
<form hx-post="/save" hx-disabled-elt="button">
    <input type="text" name="data">
    <button type="submit">Save</button>
</form>
```

### Skeleton Loading

```html
<div id="content"
     hx-get="/content"
     hx-trigger="load"
     hx-swap="innerHTML">
    <!-- Skeleton placeholder -->
    <div class="placeholder-glow">
        <span class="placeholder col-12"></span>
        <span class="placeholder col-8"></span>
    </div>
</div>
```

### Loading Overlay

```html
<div id="data-container" class="position-relative">
    <!-- Content here -->

    <style>
    #data-container.htmx-request::after {
        content: "";
        position: absolute;
        inset: 0;
        background: rgba(255,255,255,0.7);
        display: flex;
        align-items: center;
        justify-content: center;
    }
    </style>
</div>
```

---

## Error Handling

### Server-Side Errors

```go
func (h *ItemHandler) Create(c *gin.Context) {
    name := c.PostForm("name")

    if name == "" {
        c.String(http.StatusBadRequest, `<span class="text-danger">Name is required</span>`)
        return
    }

    item, err := h.itemRepo.Create(name)
    if err != nil {
        c.String(http.StatusInternalServerError, `<span class="text-danger">Failed to create item</span>`)
        return
    }

    c.HTML(http.StatusOK, "items/partials/_item.html", gin.H{"item": item})
}
```

### Client-Side Error Handling

```html
<div hx-post="/action"
     hx-target="#result"
     hx-on::response-error="showError(event)">
    ...
</div>

<script>
function showError(event) {
    const status = event.detail.xhr.status;
    if (status === 422) {
        // Validation error - response contains error HTML
    } else if (status >= 500) {
        alert('Server error. Please try again.');
    }
}
</script>
```

### Global Error Handler

```html
<script>
document.body.addEventListener('htmx:responseError', function(e) {
    console.error('HTMX error:', e.detail.xhr.status);

    // Show generic error toast
    const event = new CustomEvent('showToast', {
        detail: {
            title: 'Error',
            message: 'Something went wrong. Please try again.'
        }
    });
    document.body.dispatchEvent(event);
});
</script>
```

### Retry Pattern

```html
<button hx-post="/action"
        hx-target="#result"
        hx-on::response-error="this.setAttribute('hx-trigger', 'click')"
        class="btn btn-primary">
    Try Again
</button>
```

---

## CSRF Protection with HTMX

### Global Header Setup

```html
<!-- In base.html <body> tag -->
<body hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>
```

### Per-Request (if needed)

```html
<form hx-post="/action"
      hx-headers='{"X-CSRF-Token": "{{.csrf_token}}"}'>
    ...
</form>
```

---

**Next:** [Frontend](frontend.md) | [Security](security.md)
