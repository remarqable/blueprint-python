# New Module Template

This directory contains starter files for creating a new Blueprint Go module.

## Usage

1. **Copy this directory** to your project's modules folder:
   ```bash
   cp -r blueprint-go/templates/new-module/ internal/modules/yourmodule/
   ```

2. **Rename placeholders** - Replace all occurrences of:
   - `modulename` → your module name (lowercase)
   - `ModuleName` → your module name (PascalCase)
   - `yourmodule` → your module name

3. **Update files**:
   - `manifest.go` - Set name, description, routes, icon
   - `module.go` - Add any initialization logic
   - `models/item.go` - Create your models
   - `handlers/routes.go` - Define your routes
   - `views/templates/` - Create your templates
   - `lang/en.json` - Add translations

4. **Register module** in your app's module loader

## File Structure

```
yourmodule/
├── manifest.go              # Module metadata
├── module.go                # Module interface implementation
├── models/
│   └── item.go              # Model + Repository
├── handlers/
│   └── routes.go            # Gin handlers
├── views/
│   └── templates/
│       └── modulename/
│           └── index.html   # Main template
└── lang/
    └── en.json              # English translations
```

## Quick Start Commands

```bash
# From project root
MODULE=tasks

# Create module directory
mkdir -p internal/modules/$MODULE/{models,handlers,views/templates/$MODULE,lang}

# Copy templates
cp blueprint-go/templates/new-module/manifest.go internal/modules/$MODULE/
cp blueprint-go/templates/new-module/module.go internal/modules/$MODULE/
cp blueprint-go/templates/new-module/models/item.go internal/modules/$MODULE/models/
cp blueprint-go/templates/new-module/handlers/routes.go internal/modules/$MODULE/handlers/
cp blueprint-go/templates/new-module/views/templates/modulename/index.html internal/modules/$MODULE/views/templates/$MODULE/
cp blueprint-go/templates/new-module/lang/en.json internal/modules/$MODULE/lang/

# Find and replace
find internal/modules/$MODULE -type f -exec sed -i "s/modulename/$MODULE/g" {} \;
find internal/modules/$MODULE -type f -exec sed -i "s/ModuleName/Tasks/g" {} \;
```
