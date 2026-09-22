.PHONY: help install upgrade css css-watch assets migrate run test test-postgres clean deploy provision skills

TAILWIND_VERSION := v4.1.14
TAILWIND_BIN := bin/tailwindcss
HTMX_VERSION := 2.0.4
ALPINE_VERSION := 3.14.9

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  install    Create .venv and install dependencies (uv sync)"
	@echo "  upgrade    Upgrade all dependencies and update uv.lock"
	@echo "  assets     Vendor htmx + alpine into app/static/js"
	@echo "  css        Build Tailwind CSS (commit the output)"
	@echo "  css-watch  Rebuild Tailwind CSS on change"
	@echo "  migrate    Apply database migrations"
	@echo "  run        Build CSS, apply migrations, run the application"
	@echo "  test       Run tests (SQLite in-memory)"
	@echo "  test-postgres  Run tests against PostgreSQL"
	@echo "  skills     Link blueprint skills into .claude/skills"
	@echo "  clean      Remove .venv and cache files"
	@echo "  deploy     Deploy the application"
	@echo "  provision  Show server provisioning instructions"

install:
	uv sync

upgrade:
	uv lock --upgrade
	uv sync

$(TAILWIND_BIN):
	@mkdir -p bin
	@case "$$(uname -s)-$$(uname -m)" in \
	  Darwin-arm64)  F=tailwindcss-macos-arm64 ;; \
	  Darwin-x86_64) F=tailwindcss-macos-x64 ;; \
	  Linux-aarch64) F=tailwindcss-linux-arm64 ;; \
	  Linux-x86_64)  F=tailwindcss-linux-x64 ;; \
	  *) echo "Unsupported platform: $$(uname -s)-$$(uname -m)"; exit 1 ;; \
	esac; \
	echo "Downloading tailwindcss $(TAILWIND_VERSION) ($$F)..."; \
	curl -sL -o $(TAILWIND_BIN) \
	  https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$$F
	@chmod +x $(TAILWIND_BIN)

css: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i app/static/css/input.css -o app/static/css/app.css --minify

css-watch: $(TAILWIND_BIN)
	$(TAILWIND_BIN) -i app/static/css/input.css -o app/static/css/app.css --watch

assets:
	@mkdir -p app/static/js
	curl -sL -o app/static/js/htmx.min.js   https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
	curl -sL -o app/static/js/alpine.min.js https://unpkg.com/alpinejs@$(ALPINE_VERSION)/dist/cdn.min.js
	@echo "Vendored htmx $(HTMX_VERSION) and alpine $(ALPINE_VERSION)."

migrate:
	uv run flask db upgrade

run: css migrate
	uv run python run.py

test:
	uv run pytest tests/ -v

test-postgres:
	TEST_DATABASE_URL=postgresql+psycopg://postgres:postgres@localhost:5432/test \
		uv run pytest tests/ -v

clean:
	rm -rf .venv bin *.db
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true
	find . -type d -name ".pytest_cache" -exec rm -rf {} + 2>/dev/null || true

skills:
	@set -e; \
	if [ -d skills ]; then BP=.; \
	else BP=$$(git config -f .gitmodules --get-regexp '\.path$$' 2>/dev/null \
	     | awk '{print $$2}' \
	     | while read -r p; do if [ -d "$$p/skills" ]; then echo "$$p"; break; fi; done); \
	fi; \
	if [ -z "$$BP" ] || [ ! -d "$$BP/skills" ]; then \
	  echo "No blueprint skills found."; \
	  echo "The blueprint submodule is missing or not initialized. Run:"; \
	  echo "  git submodule update --init --recursive"; \
	  exit 1; \
	fi; \
	mkdir -p .claude/skills; \
	for d in "$$BP"/skills/*/; do \
	  [ -f "$$d/SKILL.md" ] || continue; \
	  n=$$(basename "$$d"); \
	  rm -rf ".claude/skills/$$n"; \
	  ln -s "$$(cd "$$d" && pwd)" ".claude/skills/$$n"; \
	  echo "  linked /$$n"; \
	done; \
	if [ -f .gitignore ] && ! grep -qxF '.claude/skills/' .gitignore; then \
	  echo '.claude/skills/' >> .gitignore; \
	fi; \
	echo "Skills linked. Restart Claude Code to pick them up."

deploy:
	./scripts/deploy.sh

provision:
	@echo ""
	@echo "Server Provisioning"
	@echo "==================="
	@echo ""
	@echo "Run this on a fresh Debian/Ubuntu server:"
	@echo ""
	@echo "  scp scripts/provision-server.sh root@your-server:/tmp/"
	@echo "  ssh root@your-server 'bash /tmp/provision-server.sh'"
	@echo ""
	@echo "Or with custom config:"
	@echo ""
	@echo "  ssh root@your-server"
	@echo "  APP_NAME=yourapp APP_DOMAIN=yourapp.com bash /tmp/provision-server.sh"
	@echo ""
