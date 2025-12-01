"""Example model with full type annotations for SQLAlchemy 2.0.

Rename to match your domain. Delete this file if not needed.
"""

from datetime import datetime
from typing import ClassVar, Self

from sqlalchemy import DateTime, String, func
from sqlalchemy.orm import Mapped, mapped_column

from system.db.database import db
from system.db.decorators import ModelRegistry


@ModelRegistry.register
class Item(db.Model):  # type: ignore[name-defined]
    """Example model for the module."""

    __tablename__: ClassVar[str] = "modulename_item"  # Prefix with module name

    id: Mapped[int] = mapped_column(primary_key=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    created_at: Mapped[datetime | None] = mapped_column(DateTime, default=func.current_timestamp())
    updated_at: Mapped[datetime | None] = mapped_column(DateTime, onupdate=func.current_timestamp())

    # --- Class Methods (CRUD) ---

    @classmethod
    def create(cls, name: str) -> Self:
        """Create a new item."""
        item = cls(name=name)
        db.session.add(item)
        db.session.commit()
        return item

    @classmethod
    def get_all(cls) -> list[Self]:
        """Get all items."""
        return list(cls.query.all())

    @classmethod
    def get_by_id(cls, item_id: int) -> Self | None:
        """Get item by ID."""
        return cls.query.get(item_id)  # type: ignore[return-value]

    @classmethod
    def delete(cls, item_id: int) -> bool:
        """Delete an item."""
        item = cls.query.get(item_id)
        if item:
            db.session.delete(item)
            db.session.commit()
            return True
        return False

    @classmethod
    def create_sample_data(cls) -> None:
        """Create sample data if table is empty.

        Called from module's init_database() hook.
        Must be idempotent (safe to call multiple times).
        """
        if not cls.query.first():
            cls.create("Sample Item 1")
            cls.create("Sample Item 2")
