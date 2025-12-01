"""Module class with lifecycle hooks."""

from typing import TYPE_CHECKING

from system.db.database import db
from system.module.hooks import hookimpl

if TYPE_CHECKING:
    from flask import Blueprint


class ModuleNameModule:
    """Module class implementing lifecycle hooks.

    Rename 'ModuleName' and 'modulename' to your actual module name.
    """

    def get_routes(self) -> list[tuple["Blueprint", str]]:
        """Return list of (blueprint, url_prefix) tuples.

        Called by ModuleLoader.register_routes() during app startup.
        The url_prefix should match main_route in __manifest__.py.
        """
        from .controllers.routes import blueprint

        return [(blueprint, "/modulename")]

    @hookimpl
    def init_database(self) -> None:
        """Initialize database tables and sample data.

        Called after all modules are loaded and db.create_all() runs.
        Use try/except to handle errors gracefully.
        """
        db.create_all()

        # Optional: Create sample data
        # try:
        #     from .models.item import Item
        #     Item.create_sample_data()
        # except Exception as e:
        #     import logging
        #     logging.getLogger(__name__).error(f"Sample data error: {e}")
