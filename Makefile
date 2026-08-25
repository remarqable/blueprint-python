.PHONY: help venv install compile migrate run test test-postgres clean deploy provision

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  venv       Create virtual environment and install dependencies"
	@echo "  install    Install/update dependencies"
	@echo "  compile    Compile requirements.in to requirements.txt"
	@echo "  migrate    Apply database migrations"
	@echo "  run        Apply migrations, then run the application"
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

migrate:
	./venv/bin/flask db upgrade

run: migrate
	./venv/bin/python run.py

test:
	./venv/bin/pytest tests/ -v

test-postgres:
	TEST_DATABASE_URL=postgresql+psycopg://postgres:postgres@localhost:5432/test \
		./venv/bin/pytest tests/ -v

clean:
	rm -rf venv *.db
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
