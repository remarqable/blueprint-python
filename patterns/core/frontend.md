# Frontend Architecture

> Tailwind CSS v4 + Alpine.js + HTMX. One build step, run locally, never on the server.

---

## Table of Contents

- [Philosophy](#philosophy)
- [The Build Step](#the-build-step)
- [File Structure](#file-structure)
- [Design Tokens](#design-tokens)
- [Visual Design Rules](#visual-design-rules)
- [The Component Layer](#the-component-layer)
- [Alpine.js](#alpinejs)
- [Alpine + HTMX](#alpine--htmx)
- [Component Patterns](#component-patterns)
- [Responsive Design](#responsive-design)
- [RTL Support](#rtl-support)
- [Dark Mode](#dark-mode)
- [Accessibility](#accessibility)

---

## Philosophy

- ✅ **Utility-first CSS.** Styling lives next to the markup it styles.
- ✅ **A thin component layer** for the handful of things repeated everywhere
  (buttons, inputs, cards) — not a framework.
- ✅ **Alpine for behavior**, HTMX for server interaction. No SPA.
- ✅ **Build locally, commit the output.** The server runs `git pull`, never a
  build.
- ✅ **Progressive enhancement.** Forms work without JavaScript.

### Honest tradeoffs

This stack is a deliberate trade against Bootstrap, and it is not free:

| | Cost | Benefit |
|---|------|---------|
| Build step | One vendored binary + `make css` | No npm, no Node, no `node_modules` |
| Components | You own the navbar, modal, dropdown | No fighting someone else's markup contract |
| Accessibility | ARIA is yours to get right | See [Accessibility](#accessibility) — treat it as required |
| Browser support | Safari 16.4+, Chrome 111+, Firefox 128+ | Native cascade layers, `oklch()`, container queries |

Tailwind v4 uses modern CSS that older browsers do not implement. If you must
support pre-2023 browsers, stay on Bootstrap — this is the one requirement that
cannot be worked around.

**Do not half-migrate.** Tailwind classes with no component layer, or Tailwind
markup still relying on `data-bs-*` behavior, is worse than either framework
alone.

---

## The Build Step

Tailwind v4 ships a standalone binary. No Node, no `package.json`, no
`node_modules`.

```makefile
# Makefile
TAILWIND_VERSION := v4.1.14
TAILWIND_BIN := bin/tailwindcss

$(TAILWIND_BIN):
	@mkdir -p bin
	@case "$$(uname -s)-$$(uname -m)" in \
	  Darwin-arm64)  F=tailwindcss-macos-arm64 ;; \
	  Darwin-x86_64) F=tailwindcss-macos-x64 ;; \
	  Linux-aarch64) F=tailwindcss-linux-arm64 ;; \
	  Linux-x86_64)  F=tailwindcss-linux-x64 ;; \
	  *) echo "Unsupported platform"; exit 1 ;; \
	esac; \
	curl -sL -o $(TAILWIND_BIN) \
	  https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$$F
	@chmod +x $(TAILWIND_BIN)

css: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i app/static/css/input.css -o app/static/css/app.css --minify

css-watch: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i app/static/css/input.css -o app/static/css/app.css --watch
```

```gitignore
bin/                      # the binary is per-platform, downloaded on demand
```

**Commit `app/static/css/app.css`.** The compiled stylesheet is a build artifact,
but committing it is what keeps deployment a `git pull` with no toolchain on the
server. The alternative — building during deploy — puts a downloader and a
compiler in your release path for no benefit.

Add a CI check so the committed CSS cannot drift from the templates:

```yaml
- run: make css && git diff --exit-code app/static/css/app.css
```

Without that check, someone edits a template, forgets `make css`, and ships
markup whose classes were never compiled. The page renders unstyled in
production and nowhere else.

---

## File Structure

```
app/static/
├── css/
│   ├── input.css          # source: tokens + component layer (~120 lines)
│   └── app.css            # BUILT — committed, never edited by hand
└── js/
    ├── htmx.min.js        # vendored
    └── alpine.min.js      # vendored
```

```bash
curl -o app/static/js/htmx.min.js   https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
curl -o app/static/js/alpine.min.js https://unpkg.com/alpinejs@3.14.9/dist/cdn.min.js
```

Vendor rather than hotlink a CDN: no third-party availability in your uptime, no
third-party script in your CSP, and a version you control.

---

## Design Tokens

Tailwind v4 is configured in CSS. There is no `tailwind.config.js`.

```css
/* app/static/css/input.css */
@import "tailwindcss";

/* Explicit sources beat auto-detection: Jinja templates live outside the CSS
   directory, and plugin templates live outside app/ entirely. */
@source "../../views";
@source "../../../plugins";

@theme {
  /* Brand scale. Every utility compiles to var(--color-brand-N), which is what
     makes per-tenant theming a variable override. See ../theming.md. */
  --color-brand-50:  oklch(0.97 0.02 275);
  --color-brand-100: oklch(0.93 0.05 275);
  --color-brand-500: oklch(0.59 0.20 275);
  --color-brand-600: oklch(0.52 0.20 275);
  --color-brand-700: oklch(0.45 0.18 275);

  --font-sans: "Inter", ui-sans-serif, system-ui, sans-serif;
  --radius-card: 0.75rem;
}

/* Class-based dark mode; drop this line to follow the OS setting instead. */
@custom-variant dark (&:where(.dark, .dark *));
```

`bg-brand-500` compiles to `background-color: var(--color-brand-500)`. Override
that variable anywhere in the cascade and every utility using it follows — the
mechanism per-tenant branding relies on.

---

## Visual Design Rules

> Constraints, not suggestions. Break one and say so in the commit message,
> with the reason. `tests/test_platform/test_ui_budgets.py` enforces the
> countable ones.

### Type

- **Four sizes and two weights per screen.** Reaching for a fifth size usually
  means a hierarchy problem being solved with type instead of with spacing.
- **No bespoke sizes.** `text-[11px]` is how a scale of four becomes a scale of
  nine. Use the scale; if the scale is wrong, change the scale.
- Reuse a size across roles rather than inventing one for each role.

### Spacing

- **Every spacing, size and radius divisible by 4, preferably 8.** Tailwind's
  `.5` steps are 2px multiples — `mt-1.5` is 6px, `gap-2.5` is 10px — so they
  break this rule while looking like they belong to the scale.
- Space *inside* elements deliberately, not only between them.
- Group related things tight and unrelated things loose. Proximity does work
  that labels should not have to.

### Color

- **60 / 30 / 10.** Sixty percent neutral, thirty complementary (near-black,
  borders, secondary text), ten brand accent.
- **Build depth with tints of one color, not with more colors.**
- **The accent is a budget.** Spend it on the one thing the reader should look
  at. Everything highlighted means nothing is.

### Visuals

- Flat over gradient, simple over flashy. No stacked shadows.
- Visuals communicate before they decorate.
- **Reuse a motif to connect related parts of a screen.** The highest-leverage
  rule here: a category's tone appears as its pill, the band on its card, its
  icon and its rule in the home feed, so the same colour means the same thing
  in four places.
- Cut one thing before calling it done.

### Copy

- Shortest phrasing that stays unambiguous. Two words beat four.
- Do not repeat a word from the nearby heading. Under "Categories", the button
  is "Add", not "Add category".
- Name the action, not the abstraction: "Save changes", never "Submit".
- An action keeps its name through the flow: a **Publish** button produces a
  **Published** toast.
- Active voice, sentence case. Errors say what broke and how to fix it; empty
  states invite an action.

### Motion

There are no route transitions to choreograph in a server-rendered app, so
this is short:

- Motion explains a state change — where something came from, what it became.
  One orchestrated moment beats five scattered effects.
- **`prefers-reduced-motion` is honoured globally** in `input.css`, not per
  component, because the components that forget are the ones it is for.
  Durations collapse to a single frame rather than to `none`: a spinner that
  stops spinning stops saying anything.

### The two standing exceptions

Written down so nobody deletes them citing a rule, and nobody adds a third
without arguing for it.

**Six category hues, against "tints of one color."** The hue is the
identifier, not decoration — it is the motif that ties an archive card to a
pill to a row in the feed. Kept deliberately quiet: `-50` grounds, `-600`
marks, and a short band, so it reads as a system rather than as a palette.

**The accent is the tenant's, so 60/30/10 cannot be guaranteed.**
`--color-brand-*` is org-configurable (see [theming](../theming.md)). An
organization that picks red gets primary buttons that read as destructive, and
no rule here can prevent it. Either constrain the hue range in the brand
picker or accept it; today we accept it.

### Self-check before calling a UI change done

The first three are the test; run it. The rest need eyes.

1. `uv run pytest tests/test_platform/test_ui_budgets.py` — type, bespoke
   sizes, off-grid spacing, reduced motion.
2. Count accent uses on the screen. More than roughly a tenth of the surface?
3. Read every label aloud. Any word repeated from its own heading? Any label
   describing the system rather than the reader's action?
4. Name the one element the reader should notice first. Is it the most
   emphasized thing there?
5. Remove one accessory. Which one did you remove?

### Standing debt

The budgets in the test are ratchets set at what each page does today, not at
the target. Off-grid spacing is the real debt — several dozen `.5` steps
across the shell. They are not rounded in bulk because that is a visual change
needing an eye rather than a script. Lower a budget when you have removed
something; raising one is a decision to argue for, not a way to make a build
pass.

---

## The Component Layer

Utility-first does not mean repeating fourteen classes on every button. Define
the few genuinely repeated elements once:

```css
/* app/static/css/input.css (continued) */
@layer components {
  .btn {
    @apply inline-flex items-center justify-center gap-2 rounded-md px-4 py-2
           text-sm font-medium transition-colors
           focus-visible:outline-2 focus-visible:outline-offset-2
           disabled:pointer-events-none disabled:opacity-50;
  }
  .btn-primary   { @apply btn bg-brand-600 text-white hover:bg-brand-700
                          focus-visible:outline-brand-600; }
  .btn-secondary { @apply btn bg-white text-slate-700 ring-1 ring-slate-300
                          hover:bg-slate-50 dark:bg-slate-800 dark:text-slate-100
                          dark:ring-slate-600; }
  .btn-danger    { @apply btn bg-red-600 text-white hover:bg-red-700; }

  .label { @apply block text-sm font-medium text-slate-700 dark:text-slate-200; }
  .input {
    @apply block w-full rounded-md border-0 px-3 py-2 text-slate-900
           ring-1 ring-inset ring-slate-300 placeholder:text-slate-400
           focus:ring-2 focus:ring-inset focus:ring-brand-600
           dark:bg-slate-800 dark:text-slate-100 dark:ring-slate-600;
  }
  .select   { @apply input pe-10; }
  .textarea { @apply input min-h-24; }
  .form-error { @apply mt-1 text-sm text-red-600 dark:text-red-400; }

  .card {
    @apply rounded-[--radius-card] bg-white p-6 shadow-sm ring-1 ring-slate-200
           dark:bg-slate-800 dark:ring-slate-700;
  }
}
```

**Keep this list short.** Buttons, form controls, and cards earn a class because
they appear on nearly every page with identical styling. A one-off panel does
not — put the utilities inline where you can see them.

> `.select { @apply input pe-10; }` uses `pe-` (padding-inline-end), not `pr-`.
> Logical properties are why RTL needs no second stylesheet. See
> [RTL Support](#rtl-support).

---

## Alpine.js

Alpine supplies the behavior Bootstrap's JS bundle used to: toggles, dropdowns,
modals, dismissible alerts. About 15KB, no build step, declarative in the markup.

```html
<script defer src="/static/js/alpine.min.js"></script>
```

`defer` is required — Alpine initializes on `DOMContentLoaded` and will miss the
DOM without it.

> Your Content-Security-Policy must include `'unsafe-eval'`, or Alpine's
> directives silently do nothing. The policy in
> [security.md](security.md#why-unsafe-eval-is-in-there) already has it.

Add the `x-cloak` rule or every `x-show` element flashes visible on first paint:

```css
@layer base {
  [x-cloak] { display: none !important; }
}
```

The three directives that cover most needs:

```html
<div x-data="{ open: false }">
  <button @click="open = !open" :aria-expanded="open">Menu</button>
  <div x-show="open" @click.outside="open = false" x-cloak>…</div>
</div>
```

Use Alpine's `$store` for state shared across components (toasts, a global
drawer). Do not reach for it otherwise — local `x-data` is easier to follow.

---

## Alpine + HTMX

They compose well, with two things to know.

**Alpine initializes swapped-in content automatically.** Alpine 3 watches the DOM
with a MutationObserver, so markup HTMX swaps in gets initialized without a hook.

**State inside a swapped element is destroyed with it.** If a dropdown's `open`
state must survive a swap, hoist `x-data` to an ancestor that HTMX does not
replace:

```html
<!-- x-data survives; only the list inside is swapped -->
<div x-data="{ selected: null }">
  <div id="results" hx-get="/search" hx-trigger="keyup changed delay:300ms from:#q">
    {% include 'partials/_results.html' %}
  </div>
</div>
```

### Toasts from the server

`HX-Trigger` lets a controller raise a client-side event, which Alpine turns into
a toast — the replacement for Bootstrap's toast component:

```python
return Response(html, headers={
    'HX-Trigger': json.dumps({'toast': {'message': _('settings.saved'), 'level': 'success'}})
})
```

```html
<!-- app/views/partials/_toasts.html -->
<div x-data="{ toasts: [] }"
     @toast.window="
       const id = Date.now();
       toasts.push({ id, ...$event.detail });
       setTimeout(() => toasts = toasts.filter(t => t.id !== id), 5000)"
     class="fixed bottom-4 end-4 z-50 flex flex-col gap-2"
     role="status" aria-live="polite">
  <template x-for="t in toasts" :key="t.id">
    <div x-transition
         class="rounded-md px-4 py-3 text-sm shadow-lg ring-1"
         :class="t.level === 'success'
                 ? 'bg-green-50 text-green-800 ring-green-200'
                 : 'bg-red-50 text-red-800 ring-red-200'"
         x-text="t.message"></div>
  </template>
</div>
```

`aria-live="polite"` is what makes a toast reach a screen reader. Without it the
message is purely visual.

---

## Component Patterns

### Base layout

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

  <a href="#main" class="sr-only focus:not-sr-only focus:absolute focus:m-3
                         focus:rounded focus:bg-white focus:p-3">
    {{ _('nav.skip_to_content') }}
  </a>

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

One `hx-headers` on `<body>` attaches the CSRF token to every HTMX request. See
[security.md](security.md).

### Navbar with mobile toggle and user dropdown

```html
<!-- app/views/partials/_navbar.html -->
<nav x-data="{ mobile: false }"
     class="border-b border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800">
  <div class="mx-auto flex max-w-5xl items-center gap-4 px-4 py-3">
    <a href="/" class="font-semibold">{{ _('app.name') }}</a>

    <div class="hidden gap-4 md:flex">
      <a href="/" class="text-sm hover:text-brand-600">{{ _('nav.home') }}</a>
      {% for item in plugin_nav %}
        <a href="{{ url_for(item.endpoint) }}"
           class="text-sm hover:text-brand-600">{{ _(item.label) }}</a>
      {% endfor %}
    </div>

    <div class="ms-auto flex items-center gap-2">
      {% if current_user.is_authenticated %}
      <div x-data="{ open: false }" class="relative">
        <button @click="open = !open" @keydown.escape="open = false"
                :aria-expanded="open" aria-haspopup="menu"
                class="btn-secondary">{{ current_user.name }}</button>

        <div x-show="open" @click.outside="open = false" x-cloak x-transition
             role="menu"
             class="absolute end-0 mt-2 w-48 rounded-md bg-white py-1 shadow-lg
                    ring-1 ring-black/5 dark:bg-slate-800">
          <a href="/profile" role="menuitem"
             class="block px-4 py-2 text-sm hover:bg-slate-50 dark:hover:bg-slate-700">
            {{ _('nav.profile') }}</a>
          <form action="/logout" method="POST">
            <input type="hidden" name="csrf_token" value="{{ csrf_token }}">
            <button type="submit" role="menuitem"
                    class="block w-full px-4 py-2 text-start text-sm
                           hover:bg-slate-50 dark:hover:bg-slate-700">
              {{ _('nav.logout') }}</button>
          </form>
        </div>
      </div>
      {% else %}
      <a href="/login" class="btn-primary">{{ _('nav.login') }}</a>
      {% endif %}

      <button @click="mobile = !mobile" :aria-expanded="mobile"
              aria-controls="mobile-menu" class="btn-secondary md:hidden">
        <span class="sr-only">{{ _('nav.toggle_menu') }}</span>☰
      </button>
    </div>
  </div>

  <div id="mobile-menu" x-show="mobile" x-cloak class="border-t md:hidden">
    <a href="/" class="block px-4 py-3 text-sm">{{ _('nav.home') }}</a>
    {% for item in plugin_nav %}
      <a href="{{ url_for(item.endpoint) }}"
         class="block px-4 py-3 text-sm">{{ _(item.label) }}</a>
    {% endfor %}
  </div>
</nav>
```

### Dismissible flash messages

```html
<!-- app/views/partials/_flash.html -->
{% with messages = get_flashed_messages(with_categories=true) %}
  {% for category, message in messages %}
  <div x-data="{ show: true }" x-show="show" x-transition role="alert"
       class="mb-4 flex items-start gap-3 rounded-md p-4 text-sm ring-1
              {{ 'bg-red-50 text-red-800 ring-red-200' if category in ('error','danger')
                 else 'bg-green-50 text-green-800 ring-green-200' }}">
    <span class="flex-1">{{ message }}</span>
    <button @click="show = false" class="opacity-60 hover:opacity-100">
      <span class="sr-only">{{ _('common.dismiss') }}</span>✕
    </button>
  </div>
  {% endfor %}
{% endwith %}
```

### Modal

```html
<!-- app/views/partials/_modal.html -->
<div x-data="{ open: false }" @open-modal.window="open = true">
  <div x-show="open" x-cloak class="fixed inset-0 z-50 flex items-center justify-center">
    <div class="fixed inset-0 bg-black/50" @click="open = false" aria-hidden="true"></div>

    <div role="dialog" aria-modal="true" aria-labelledby="modal-title"
         @keydown.escape.window="open = false"
         x-transition class="relative z-10 w-full max-w-md card">
      <h2 id="modal-title" class="text-lg font-semibold">{% block modal_title %}{% endblock %}</h2>
      <div class="mt-4">{% block modal_body %}{% endblock %}</div>
      <div class="mt-6 flex justify-end gap-2">
        <button @click="open = false" class="btn-secondary">{{ _('common.cancel') }}</button>
        {% block modal_actions %}{% endblock %}
      </div>
    </div>
  </div>
</div>
```

A dialog also needs focus moved in on open, restored on close, and trapped while
open. Escape and `aria-modal` alone are not enough — see
[Accessibility](#accessibility).

### Forms

```html
<form method="POST" class="space-y-4">
  <input type="hidden" name="csrf_token" value="{{ csrf_token }}">

  <div>
    <label for="email" class="label">{{ _('user.email') }}</label>
    <input type="email" id="email" name="email" class="input mt-1"
           value="{{ user.email }}"
           {% if errors.email %}aria-invalid="true" aria-describedby="email-error"{% endif %}>
    {% if errors.email %}
    <p id="email-error" class="form-error">{{ errors.email }}</p>
    {% endif %}
  </div>

  <button type="submit" class="btn-primary">{{ _('common.save') }}</button>
</form>
```

`aria-invalid` and `aria-describedby` are what connect an error message to its
field for assistive technology. Colour alone does not.

---

## Responsive Design

Mobile-first: unprefixed utilities apply everywhere, prefixed ones apply upward.

```html
<div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">…</div>

<div class="hidden md:block">Desktop only</div>
<div class="md:hidden">Mobile only</div>
```

| Prefix | Min width |
|--------|-----------|
| (none) | 0 |
| `sm:` | 640px |
| `md:` | 768px |
| `lg:` | 1024px |
| `xl:` | 1280px |
| `2xl:` | 1536px |

---

## RTL Support

**Use logical properties and RTL is automatic.** One `dir` attribute, one
stylesheet, no per-direction overrides.

```html
<html dir="{{ 'rtl' if is_rtl else 'ltr' }}">
```

| Use | Not | Meaning |
|-----|-----|---------|
| `ms-4` / `me-4` | `ml-4` / `mr-4` | margin-inline-start / end |
| `ps-4` / `pe-4` | `pl-4` / `pr-4` | padding-inline-start / end |
| `text-start` / `text-end` | `text-left` / `text-right` | |
| `start-0` / `end-0` | `left-0` / `right-0` | |
| `border-s` / `border-e` | `border-l` / `border-r` | |

Reserve physical properties for things that are genuinely physical — a drop
shadow offset, an icon that must not mirror.

This deletes the whole `bootstrap.rtl.min.css` branch, along with the manual
`[dir="rtl"]` overrides that came with it.

---

## Dark Mode

With the `@custom-variant` from [Design Tokens](#design-tokens), `dark:` follows
a `dark` class on `<html>`:

```html
<html class="{{ 'dark' if theme_mode == 'dark' }}">
```

Persist the choice server-side as a user setting so it survives across devices
and does not flash on first paint. If you prefer the OS setting, delete the
`@custom-variant` line and `dark:` follows `prefers-color-scheme` with no markup
at all.

Apply `dark:` to the component layer, not to every call site — that is most of
the work, and it is why `.card` and `.input` carry their own dark variants above.

---

## Accessibility

Bootstrap shipped ARIA wiring inside its components. Owning the components means
owning that too. This is the real cost of the migration; budget for it.

**Required on every interactive component you write:**

| Component | Needs |
|-----------|-------|
| Toggle button | `:aria-expanded`, `aria-controls` |
| Dropdown | `aria-haspopup="menu"`, `role="menu"`/`menuitem`, Escape to close |
| Modal | `role="dialog"`, `aria-modal`, `aria-labelledby`, focus trap, focus restore |
| Alert / toast | `role="alert"` or `role="status"` + `aria-live` |
| Icon-only button | `<span class="sr-only">` label |
| Form error | `aria-invalid` + `aria-describedby` |

**Focus trapping** is the one thing Alpine does not give you declaratively. Add
the Focus plugin and use `x-trap`:

```html
<script defer src="/static/js/alpine-focus.min.js"></script>
<script defer src="/static/js/alpine.min.js"></script>

<div role="dialog" aria-modal="true" x-trap.noscroll="open">…</div>
```

Load the plugin **before** Alpine core — plugins register against `window.Alpine`
during its initialization.

**Focus visibility:** never remove outlines without replacing them. The component
layer uses `focus-visible:outline-2`, which shows for keyboard users and not for
mouse clicks.

**Contrast:** Tailwind's default palette is not automatically WCAG AA. On white,
`text-slate-500` is borderline for body text and `text-slate-400` fails. Use
`text-slate-600` or darker for anything a user must read, and check your brand
scale against its background rather than assuming.

---

**Next:** [HTMX Patterns](htmx.md) | [Mobile Navigation](mobile-navigation.md) | [Security](security.md)
