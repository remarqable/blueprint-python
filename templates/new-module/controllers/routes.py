"""Module routes - Flask blueprint with typed route handlers."""

from flask import Blueprint, render_template
from flask.typing import ResponseReturnValue
from flask_login import login_required  # type: ignore[import-untyped]

# Create blueprint - name should be unique (modulename_bp)
# Rename 'modulename' to your actual module name
blueprint: Blueprint = Blueprint(
    "modulename_bp",
    __name__,
    template_folder="../views/templates",
    static_folder="../views/assets",
)


@blueprint.route("/")
@login_required
def index() -> ResponseReturnValue:
    """Module home page."""
    return render_template("modulename/index.html")


# Example: List route
# @blueprint.route("/items")
# @login_required
# def list_items() -> ResponseReturnValue:
#     from ..models.item import Item
#     items = Item.get_all()
#     return render_template("modulename/list.html", items=items)


# Example: Create route
# @blueprint.route("/items/add", methods=["POST"])
# @login_required
# def add_item() -> ResponseReturnValue:
#     from flask import request, redirect, url_for, flash
#     name = request.form.get("name")
#     if name:
#         from ..models.item import Item
#         Item.create(name)
#         flash(_("Item created"), "success")
#     return redirect(url_for("modulename_bp.index"))
