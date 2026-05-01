# Color System

> Purple + Gray design system for consistent, professional UI.

---

## Philosophy

The design system uses a **restrained color palette**:
- **Purple** (`#7c3aed`) - Brand color, interactive elements
- **Gray scale** - Everything else (text, borders, backgrounds)
- **Semantic colors** - Only when communicating meaning (success, error, warning)

This creates a distinctive, professional look while maintaining accessibility.

---

## CSS Variables

All colors are defined in `modules/base/core/views/assets/css/base.css` as CSS custom properties.

### Primary Colors (Purple)

| Variable | Value | Use |
|----------|-------|-----|
| `--color-primary` | `#7c3aed` | Main brand color, buttons, links |
| `--color-primary-dark` | `#6d28d9` | Hover states |
| `--color-primary-light` | `#a78bfa` | Badges, accents |
| `--color-primary-lighter` | `#ddd6fe` | Light backgrounds |
| `--color-primary-lightest` | `#f3e8ff` | Very light backgrounds |

### Gray Scale

| Variable | Value | Use |
|----------|-------|-----|
| `--color-gray-900` | `#111827` | Primary text |
| `--color-gray-700` | `#374151` | Secondary text, icons |
| `--color-gray-500` | `#6b7280` | Placeholder text, muted |
| `--color-gray-300` | `#d1d5db` | Borders, dividers |
| `--color-gray-100` | `#f3f4f6` | Backgrounds, hover |
| `--color-gray-50` | `#f9fafb` | Page background |
| `--color-white` | `#ffffff` | Cards, inputs |

### Semantic Colors (Use Sparingly)

| Variable | Value | Use |
|----------|-------|-----|
| `--color-success` | `#10b981` | Success messages, positive actions |
| `--color-success-light` | `#d1fae5` | Success backgrounds |
| `--color-danger` | `#ef4444` | Errors, delete actions |
| `--color-danger-light` | `#fee2e2` | Error backgrounds |
| `--color-warning` | `#f59e0b` | Warnings, caution |
| `--color-warning-light` | `#fef3c7` | Warning backgrounds |
| `--color-info` | `#3b82f6` | Information, links |
| `--color-info-light` | `#dbeafe` | Info backgrounds |

### Module Color

Each module defines its own `--module-color` for theming:

```css
/* In module CSS file */
.yourmodule-app { --module-color: var(--color-primary); }
```

This is used for:
- Header border accent
- Navigation active states
- Module-specific icons

---

## Usage Examples

### Text

```css
/* Primary text */
color: var(--color-gray-900);

/* Secondary/muted text */
color: var(--color-gray-500);

/* Links */
color: var(--color-primary);
```

### Backgrounds

```css
/* Page background */
background-color: var(--color-gray-50);

/* Cards */
background-color: var(--color-white);

/* Hover states */
background-color: var(--color-gray-100);

/* Selected/active */
background-color: var(--color-primary-lightest);
```

### Borders

```css
/* Standard border */
border: 1px solid var(--color-gray-300);

/* Subtle border */
border: 1px solid var(--color-gray-100);

/* Active/focus border */
border-color: var(--color-primary);
```

### Buttons

```css
/* Primary button */
background-color: var(--color-primary);
color: var(--color-white);

/* Primary button hover */
background-color: var(--color-primary-dark);

/* Outline button (default gray, color on hover) */
color: var(--color-gray-500);
border-color: var(--color-gray-300);

/* Outline button hover */
color: var(--color-white);
background-color: var(--color-primary);
```

### Badges

```css
/* Use Bootstrap classes with our colors */
.badge.bg-success  /* Green - positive */
.badge.bg-warning  /* Amber - caution */
.badge.bg-danger   /* Red - negative */
.badge.bg-info     /* Blue - neutral info */
.badge.bg-secondary /* Gray - default */
```

---

## Module CSS Pattern

Every module should have a CSS file that:
1. Defines its `--module-color`
2. Uses CSS variables from base.css
3. Contains only module-specific styles

```css
/* data/modules/apps/yourapp/views/assets/css/yourapp.css */

/* Module color definition */
.yourapp-app { --module-color: var(--color-primary); }

/* Module-specific styles only */
.yourapp-special-card {
    border-left: 4px solid var(--module-color);
    background: var(--color-white);
}

.yourapp-icon {
    color: var(--module-color);
}
```

---

## Do's and Don'ts

### Do

- Use CSS variables for all colors
- Use gray scale for most UI elements
- Reserve purple for interactive elements
- Use semantic colors only for meaning
- Keep module CSS minimal (<100 lines)

### Don't

- Hardcode hex colors in CSS
- Use multiple brand colors (no rainbow)
- Overuse semantic colors decoratively
- Create custom badge styles (use Bootstrap)
- Duplicate base.css patterns in modules

---

## Bootstrap Integration

CSS variables are mapped to Bootstrap for compatibility:

```css
--bs-primary: #7c3aed;
--bs-secondary: #6b7280;
--bs-success: #10b981;
--bs-danger: #ef4444;
--bs-warning: #f59e0b;
--bs-info: #3b82f6;
```

Use Bootstrap utility classes when possible:
- `text-primary`, `text-secondary`, `text-muted`
- `bg-primary`, `bg-light`, `bg-white`
- `border-primary`, `border-secondary`
- `btn-primary`, `btn-outline-primary`

---

## Accessibility

All color combinations meet WCAG AA standards:
- Gray-900 on white: 16.5:1 ratio
- Gray-700 on white: 9.5:1 ratio
- Gray-500 on white: 4.6:1 ratio
- Primary on white: 5.4:1 ratio

For low-contrast text (gray-500), ensure font size is at least 14px.

---

**Next:** [Frontend Patterns](frontend.md) | [Module System](module-system.md)
