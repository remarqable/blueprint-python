.PHONY: help venv install compile css css-watch assets migrate run test test-postgres clean deploy provision

TAILWIND_VERSION := v4.1.14
TAILWIND_BIN := bin/tailwindcss
HTMX_VERSION := 2.0.4
ALPINE_VERSION := 3.14.9

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  venv       Create virtual environment and install dependencies"
	@echo "  install    Install/update dependencies"
	@echo "  compile    Compile requirements.in to requirements.txt"
	@echo "  assets     Vendor htmx + alpine into app/static/js"
	@echo "  css        Build Tailwind CSS (commit the output)"
	@echo "  css-watch  Rebuild Tailwind CSS on change"
	@echo "  migrate    Apply database migrations"
	@echo "  run        Build CSS, apply migrations, run the application"
	@echo "  test       Run tests (SQLite in-memory)"
	@echo "  test-postgres  Run tests against PostgreSQL"
	@echo "  clean      Remove venv and cache files"
	@echo "  deploy     Deploy the application"
	@echo "  provision  Show server provisioning instructions"

venv:
	rm -rf venv
	python3 -m venv venv
	./venv/bin/pip install -r requirements.txt
	@echo "Done. Run 'source venv/bin/activate' to activate."

install:
	./venv/bin/pip install -r requirements.txt

compile:
	./venv/bin/pip install pip-tools
	./venv/bin/pip-compile requirements.in -o requirements.txt

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
	./venv/bin/flask db upgrade

run: css migrate
	./venv/bin/python run.py

test:
	./venv/bin/pytest tests/ -v

test-postgres:
	TEST_DATABASE_URL=postgresql+psycopg://postgres:postgres@localhost:5432/test \
		./venv/bin/pytest tests/ -v

clean:
	rm -rf venv bin *.db
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true
	find . -type d -name ".pytest_cache" -exec rm -rf {} + 2>/dev/null || true

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
