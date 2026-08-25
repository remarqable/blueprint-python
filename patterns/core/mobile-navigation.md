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

Your App uses **device-aware rendering** with separate mobile and desktop templates:

1. **Detect device** via User-Agent (with query param override for testing)
2. **Render appropriate template** using `render_device_template()`
3. **Mobile-first content** - same data, optimized layout
4. **Progressive enhancement** - HTMX works on both

**Key rules:**

- Mobile templates are optional - falls back to desktop if missing
- Same controller logic, different presentation
- Use Tailwind's responsive prefixes (`md:`, `lg:`) for minor differences
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
<html lang="{{ lang }}" dir="{{ 'rtl' if is_rtl else 'ltr' }}">
<head>
  <meta charset="UTF-8">
  <!-- viewport-fit=cover is required for env(safe-area-inset-*) to report
       real values on notched iPhones. Without it, insets are always 0. -->
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <title>{% block title %}Your App{% endblock %}</title>

  <link rel="stylesheet" href="{{ url_for('static', filename='css/app.css') }}">
  <script src="{{ url_for('static', filename='js/htmx.min.js') }}"></script>
  <script defer src="{{ url_for('static', filename='js/alpine.min.js') }}"></script>
  {% block head %}{% endblock %}
</head>
<body class="bg-slate-50 text-slate-900 dark:bg-slate-900 dark:text-slate-100">

  {% block mobile_header %}{% include 'partials/_mobile_header.html' %}{% endblock %}

  <!-- Padding clears the fixed header and bottom nav, plus the iOS home bar. -->
  <main class="px-4 pt-[calc(3.5rem+var(--inset-top))]
               pb-[calc(3.5rem+var(--inset-bottom))]">
    {% include 'partials/_flash.html' %}
    {% block content %}{% endblock %}
  </main>

  {% block mobile_nav %}{% include 'partials/_mobile_nav.html' %}{% endblock %}
  {% include 'partials/_toasts.html' %}
  {% block scripts %}{% endblock %}
</body>
</html>
```

One stylesheet, shared with desktop. Tailwind's compiled output is the same file
for both template trees — there is no separate `mobile.css` to keep in sync.

---

## Navigation Components

### Icons

Use inline SVG copied into a partial (Heroicons, Lucide, or your own). An icon
webfont costs a render-blocking request and ships hundreds of glyphs to deliver
five.

```html
<!-- app/views/partials/icons/_home.html -->
<svg class="size-6" viewBox="0 0 24 24" fill="none" stroke="currentColor"
     stroke-width="1.5" aria-hidden="true">
  <path stroke-linecap="round" stroke-linejoin="round"
        d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.5
           a1.125 1.125 0 0 0 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25
           c.621 0 1.125.504 1.125 1.125V21.375h4.125c.621 0 1.125-.504 1.125-1.125V9.75" />
</svg>
```

`aria-hidden="true"` on every decorative icon, with the accessible name on the
surrounding link or button.

### Mobile Header

```html
<!-- app/views/partials/_mobile_header.html -->
<header class="fixed inset-x-0 top-0 z-40 border-b border-slate-200 bg-white
               pt-[var(--inset-top)] dark:border-slate-700 dark:bg-slate-800">
  <div class="flex h-14 items-center gap-2 px-2">
    {% block header_left %}
    <button onclick="history.back()"
            class="grid size-11 place-items-center rounded-full hover:bg-slate-100
                   dark:hover:bg-slate-700">
      <span class="sr-only">{{ _('nav.back') }}</span>
      {% include 'partials/icons/_chevron_start.html' %}
    </button>
    {% endblock %}

    <h1 class="flex-1 truncate text-center text-base font-semibold">
      {% block header_title %}{{ _('app.name') }}{% endblock %}
    </h1>

    {% block header_right %}<div class="size-11"></div>{% endblock %}
  </div>
</header>
```

The empty `size-11` spacer keeps the title optically centred when there is no
action button. `size-11` is 44px — the minimum comfortable touch target.

> The back chevron must mirror in RTL. Either use a `rotate-180 rtl:rotate-0`
> pair or keep separate start/end icon partials, as here.

### Mobile Bottom Navigation

```html
<!-- app/views/partials/_mobile_nav.html -->
<nav x-data="{ menu: false }"
     class="fixed inset-x-0 bottom-0 z-40 border-t border-slate-200 bg-white
            pb-[var(--inset-bottom)] dark:border-slate-700 dark:bg-slate-800"
     aria-label="{{ _('nav.primary') }}">
  <div class="grid grid-cols-4">

    {% macro nav_item(endpoint, icon, label, active) %}
    <a href="{{ url_for(endpoint) }}"
       class="flex min-h-14 flex-col items-center justify-center gap-1 text-xs
              {{ 'text-brand-600' if active else 'text-slate-500' }}"
       {{ 'aria-current="page"' if active }}>
      {% include 'partials/icons/_' ~ icon ~ '.html' %}
      <span>{{ _(label) }}</span>
    </a>
    {% endmacro %}

    {{ nav_item('main.index', 'home', 'nav.home', request.endpoint == 'main.index') }}
    {{ nav_item('directory.results', 'search', 'nav.find',
                request.endpoint and request.endpoint.startswith('directory')) }}

    {% if current_user.is_authenticated %}
      {{ nav_item('dashboard.index', 'user', 'nav.profile',
                  request.endpoint and request.endpoint.startswith('dashboard')) }}
    {% else %}
      {{ nav_item('auth.login', 'login', 'nav.login', request.endpoint == 'auth.login') }}
    {% endif %}

    <button @click="menu = true" :aria-expanded="menu" aria-controls="more-menu"
            class="flex min-h-14 flex-col items-center justify-center gap-1
                   text-xs text-slate-500">
      {% include 'partials/icons/_menu.html' %}
      <span>{{ _('nav.more') }}</span>
    </button>
  </div>

  <!-- Bottom sheet: replaces Bootstrap's offcanvas -->
  <div x-show="menu" x-cloak class="fixed inset-0 z-50">
    <div class="absolute inset-0 bg-black/50" @click="menu = false" aria-hidden="true"
         x-transition.opacity></div>

    <div id="more-menu" role="dialog" aria-modal="true"
         aria-label="{{ _('nav.more') }}"
         @keydown.escape.window="menu = false"
         x-trap.noscroll="menu"
         x-transition:enter-start="translate-y-full"
         x-transition:leave-end="translate-y-full"
         class="absolute inset-x-0 bottom-0 max-h-[50vh] overflow-y-auto
                rounded-t-2xl bg-white pb-[var(--inset-bottom)] transition
                dark:bg-slate-800">
      <div class="mx-auto my-3 h-1 w-10 rounded-full bg-slate-300" aria-hidden="true"></div>

      <a href="{{ url_for('main.faq') }}"
         class="flex min-h-14 items-center gap-3 px-5 hover:bg-slate-50
                dark:hover:bg-slate-700">
        {% include 'partials/icons/_help.html' %} {{ _('nav.faq') }}
      </a>
      {% if current_user.is_authenticated %}
      <form action="{{ url_for('auth.logout') }}" method="POST">
        <input type="hidden" name="csrf_token" value="{{ csrf_token }}">
        <button type="submit"
                class="flex min-h-14 w-full items-center gap-3 px-5 text-start
                       text-red-600 hover:bg-slate-50 dark:hover:bg-slate-700">
          {% include 'partials/icons/_logout.html' %} {{ _('nav.logout') }}
        </button>
      </form>
      {% endif %}
    </div>
  </div>
</nav>
```

`x-trap.noscroll` requires the Alpine Focus plugin — see
[frontend.md § Accessibility](frontend.md#accessibility). Without it, keyboard
focus escapes behind the open sheet and the page scrolls under it.

Logout is a POST form, not a link: a GET logout is triggerable by any image tag
on any page.

### Navigation Rules

| Rule | Guideline |
|------|-----------|
| Max tabs | 4–5 including More |
| Icons | Inline SVG, `aria-hidden="true"` |
| Labels | One word |
| Touch target | `min-h-14` (56px); never below 44px |
| Active state | `text-brand-600` **and** `aria-current="page"` |

Colour alone cannot signal the active tab — `aria-current` is what a screen
reader announces.

---

## Safe Areas

The only mobile styling Tailwind utilities do not cover is the iOS safe area.
Register the insets as theme values once and they become ordinary utilities:

```css
/* app/static/css/input.css */
@theme {
  --inset-top: env(safe-area-inset-top, 0px);
  --inset-bottom: env(safe-area-inset-bottom, 0px);
}
```

```html
<nav class="pb-[var(--inset-bottom)]">…</nav>
```

Two rules:

1. `viewport-fit=cover` in the viewport meta, or every inset reports `0px`.
2. Pad **fixed** elements (header, bottom nav, bottom sheet) with the inset, and
   pad `<main>` by the element height *plus* the inset. Skip this and the home
   indicator sits on top of your bottom nav.

Everything else that used to live in a hand-written `mobile.css` — headers,
cards, empty states, list rows — is utilities on the element now. Do not
reintroduce a mobile stylesheet; a second source of truth for layout is how the
mobile and desktop trees drift apart.

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
5. [ ] Style with Tailwind utilities; add nothing to a mobile stylesheet
6. [ ] Override `{% block mobile_nav %}` if page needs custom nav
7. [ ] Test with `?device=mobile` query param
8. [ ] Verify touch targets are 44px minimum (`min-h-11` / `size-11`)
9. [ ] Check safe-area padding on notched devices (`viewport-fit=cover` set?)
10. [ ] Update controller to use `render_device_template()`
11. [ ] Run `make css` and commit the rebuilt `app.css`
12. [ ] Use logical properties (`ms-`/`me-`), not `ml-`/`mr-`

---

**Next:** [Frontend Patterns](frontend.md) | [HTMX Patterns](htmx.md)
