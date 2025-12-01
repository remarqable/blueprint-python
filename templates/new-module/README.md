# New Module Template

This is a template for creating new Blueprint modules.

## Quick Start

1. Copy this folder to `modules/yourmodulename/`
2. Rename all occurrences of `modulename` to your module name:
   - In `__manifest__.py`: Update name, main_route, description
   - In `__init__.py`: Rename `ModuleNameModule` class import
   - In `module.py`: Rename class and url_prefix in `get_routes()`
   - In `controllers/routes.py`: Rename blueprint name
   - Rename `views/templates/modulename/` folder
   - Update template references
3. Restart the app: `python app.py`
4. Navigate to `http://localhost:8000/yourmodulename`

## Files

```
yourmodule/
├── __init__.py          # Exports module_instance
├── __manifest__.py      # Module metadata
├── module.py            # Module class with hooks
├── controllers/
│   └── routes.py        # Flask blueprint
├── models/
│   └── item.py          # Example model (delete if not needed)
├── views/
│   ├── templates/yourmodule/
│   │   └── index.html   # Main template
│   └── assets/css/      # Module CSS (optional)
└── lang/
    └── en.json          # English translations
```

## Checklist

- [ ] Update `__manifest__.py` with correct metadata
- [ ] Rename all `modulename` references
- [ ] Add your models in `models/`
- [ ] Add your routes in `controllers/routes.py`
- [ ] Update templates in `views/templates/`
- [ ] Add translations in `lang/`
- [ ] Test the module loads correctly
