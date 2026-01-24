.PHONY: venv install compile run test clean deploy

# Create virtual environment
venv:
	python3 -m venv venv
	@echo "Virtual environment created. Run 'source venv/bin/activate' to activate."

# Install dependencies
install: venv
	./venv/bin/pip install -r requirements.txt

# Compile requirements.in -> requirements.txt
compile:
	./venv/bin/pip install pip-tools
	./venv/bin/pip-compile requirements.in -o requirements.txt

# Run development server
run:
	./venv/bin/python run.py

# Run tests
test:
	./venv/bin/pytest tests/ -v

# Remove venv and cached files
clean:
	rm -rf venv
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete 2>/dev/null || true
	find . -type d -name ".pytest_cache" -exec rm -rf {} + 2>/dev/null || true

# Deploy to production
deploy:
	./scripts/deploy.sh
