# Deployment Guide

> Production deployment with Gunicorn, systemd, and Caddy on Digital Ocean

---

## Table of Contents

- [Overview](#overview)
- [Server Provisioning](#server-provisioning)
- [Application Entry Points](#application-entry-points)
- [Environment Variables](#environment-variables)
- [Local Development](#local-development)
- [Production Setup](#production-setup)
- [Deploy Script](#deploy-script)
- [Health Check](#health-check)
- [Production Checklist](#production-checklist)

---

## Overview

**Stack:**
- **Server**: Digital Ocean Droplet (or any VPS)
- **Process Manager**: systemd
- **WSGI Server**: Gunicorn
- **Reverse Proxy**: Caddy (automatic HTTPS)
- **Database**: SQLite (file-based, no separate service)

**Why this stack?**
- Simple, reliable, low maintenance
- Automatic HTTPS with Caddy
- systemd handles restarts, logging, boot
- No containers, no orchestration complexity
- Scales to thousands of users on a $12/month droplet

---

## Server Provisioning

Use the provisioning script to set up a fresh server with all dependencies.

### Quick Provision (Recommended)

```bash
# SSH into your fresh server
ssh root@your-server.com

# Run provisioning script with your config
APP_NAME=yourapp \
APP_DOMAIN=yourapp.com \
GIT_REPO=https://github.com/yourorg/yourapp.git \
bash <(curl -sL https://raw.githubusercontent.com/yourorg/blueprint/main/scripts/provision-server.sh)
```

### What It Installs

- **uv** (creates the virtualenv and installs locked dependencies)
- **Git** (for deployments)
- **Caddy** (reverse proxy with automatic HTTPS)
- **UFW firewall** (SSH, HTTP, HTTPS only)
- **fail2ban** (brute force protection)
- **Utilities**: htop, ncdu, jq

### Manual Provision

If you prefer to run steps manually:

```bash
# Update system
apt update && apt upgrade -y

# Install uv (system-wide, so root and systemd can find it).
# uv installs Python itself if the system lacks 3.11+.
curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh

# Install Git
apt install -y git

# Install Caddy
apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update && apt install -y caddy

# Configure firewall
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow http
ufw allow https
ufw enable
```

---

## Application Entry Points

### Development

```python
# run.py
"""Development server entry point."""
from dotenv import load_dotenv
load_dotenv('config/local.env')

from app import create_app

app = create_app()

if __name__ == '__main__':
    import os
    port = int(os.environ.get('PORT', 8000))
    print(f'\n  App running at http://localhost:{port}\n')
    app.run(host='0.0.0.0', port=port, debug=True)
```

### Production

```python
# wsgi.py
"""Production WSGI entry point."""
from app import create_app
app = create_app()
```

---

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `SECRET_KEY` | Session encryption key | `python -c "import secrets; print(secrets.token_hex(32))"` |
| `DATABASE_URL` | Database connection | `sqlite:///app.db` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | Environment name | `dev` |
| `PORT` | Server port | `8000` |

### Example config/local.env

```bash
APP_ENV=dev
PORT=8000
SECRET_KEY=dev-secret-key-change-in-production
DATABASE_URL=sqlite:///app.db
DEV_MAGIC=true
```

### Production .env

```bash
APP_ENV=production
PORT=8000
SECRET_KEY=<generate-with-secrets.token_hex(32)>
DATABASE_URL=sqlite:///data/app.db
```

---

## Local Development

```bash
# Setup
make install

# Run
make run
```

---

## Production Setup

### 1. Server Preparation (Digital Ocean Droplet)

```bash
# SSH into server
ssh root@your-server.com

# Create app directory
mkdir -p /opt/yourapp
cd /opt/yourapp

# Clone repository
git clone https://github.com/yourorg/yourapp.git .

# Install locked dependencies into .venv (no dev tools on the server)
uv sync --locked --no-dev

# Create data directory for SQLite
mkdir -p data

# Create production .env
cat > .env <<EOF
APP_ENV=production
PORT=8000
SECRET_KEY=$(python -c "import secrets; print(secrets.token_hex(32))")
DATABASE_URL=sqlite:///data/app.db
EOF
```

### 2. Systemd Service

```ini
# /etc/systemd/system/yourapp.service
[Unit]
Description=YourApp Flask Application
After=network.target

[Service]
User=www-data
Group=www-data
WorkingDirectory=/opt/yourapp
Environment="PATH=/opt/yourapp/.venv/bin"
EnvironmentFile=/opt/yourapp/.env
# Migrations run once here, NOT inside create_app: N workers would race
# the same Alembic upgrade on boot. Must exit 0 or the unit fails.
ExecStartPre=/opt/yourapp/.venv/bin/flask db upgrade
ExecStart=/opt/yourapp/.venv/bin/gunicorn wsgi:app -w 4 -b 127.0.0.1:8000
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable yourapp
sudo systemctl start yourapp

# Check status
sudo systemctl status yourapp

# View logs
sudo journalctl -u yourapp -f
```

### 3. Caddy Reverse Proxy

```bash
# Install Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

```
# /etc/caddy/Caddyfile
yourapp.com {
    reverse_proxy localhost:8000
}
```

```bash
# Reload Caddy
sudo systemctl reload caddy
```

Caddy automatically:
- Obtains and renews SSL certificates
- Redirects HTTP to HTTPS
- Enables HTTP/2

### 4. File Permissions

```bash
# Set ownership
sudo chown -R www-data:www-data /opt/yourapp

# Ensure data directory is writable
sudo chmod 755 /opt/yourapp/data
```

---

## Deploy Script

Create `scripts/deploy.sh` for one-command deployments:

```bash
#!/bin/bash
set -e

# Configuration - customize these
REMOTE="root@yourapp.com"
REMOTE_DIR="/opt/yourapp"
SERVICE="yourapp"
HEALTH_URL="https://yourapp.com/health"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'
CHECK="${GREEN}✓${NC}"
CROSS="${RED}✗${NC}"

echo ""

# Check for uncommitted changes
cd "$(git rev-parse --show-toplevel)"
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${CROSS} Uncommitted changes. Please commit first."
    git status --short
    exit 1
fi
echo -e "${CHECK} Working tree clean"

# Push to origin
git push -q origin main 2>/dev/null || true
echo -e "${CHECK} Pushed to origin"

# Compare local and remote commits
LOCAL_COMMIT=$(git rev-parse HEAD)
REMOTE_COMMIT=$(ssh $REMOTE "cd $REMOTE_DIR && git rev-parse HEAD")

if [ "$LOCAL_COMMIT" = "$REMOTE_COMMIT" ]; then
    echo -e "${YELLOW}⚡${NC} Already up to date (${LOCAL_COMMIT:0:7})"
    echo ""
    exit 0
fi

echo -e "   Deploying ${LOCAL_COMMIT:0:7} (server has ${REMOTE_COMMIT:0:7})"

# Pull on remote (clean pycache first)
ssh $REMOTE "cd $REMOTE_DIR && find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null; git pull -q"
echo -e "${CHECK} Pulled on server"

# Write git SHA for health endpoint
ssh $REMOTE "cd $REMOTE_DIR && git rev-parse --short HEAD > .git_sha"

# Install dependencies (exactly what uv.lock pins, no dev tools)
ssh $REMOTE "cd $REMOTE_DIR && uv sync --locked --no-dev -q"
echo -e "${CHECK} Dependencies updated"

# Restart service
ssh $REMOTE "systemctl restart $SERVICE"
echo -e "${CHECK} Service restarted"

# Verify service is running
sleep 2
if ssh $REMOTE "systemctl is-active --quiet $SERVICE"; then
    echo -e "${CHECK} Service running"
else
    echo -e "${CROSS} Service failed to start"
    ssh $REMOTE "journalctl -u $SERVICE -n 20 --no-pager"
    exit 1
fi

# Verify health endpoint
HEALTH_RESPONSE=$(curl -s --max-time 10 "${HEALTH_URL}" || echo '{"status":"error"}')
STATUS=$(echo "$HEALTH_RESPONSE" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$STATUS" = "healthy" ]; then
    echo -e "${CHECK} Health check passed"
else
    echo -e "${CROSS} Health check failed"
    echo "  Response: ${HEALTH_RESPONSE}"
    exit 1
fi

echo ""
echo -e "${GREEN}Deployed successfully${NC}"
echo ""
```

Make it executable:

```bash
chmod +x scripts/deploy.sh
```

Usage:

```bash
make deploy
# or
./scripts/deploy.sh
```

---

## Health Check

### Health Endpoint

```python
# app/controllers/main.py
import os

@bp.route('/health')
def health():
    """Health check endpoint for monitoring."""
    # Read git SHA if available
    sha = None
    sha_file = os.path.join(os.path.dirname(__file__), '../../.git_sha')
    if os.path.exists(sha_file):
        with open(sha_file) as f:
            sha = f.read().strip()

    return {
        'status': 'healthy',
        'version': sha
    }
```

### API Health (with DB check)

```python
@bp.route('/api/v1/health')
def api_health():
    """API health check with database verification."""
    from app.extensions import db

    try:
        # Verify database connection
        db.session.execute(sa.text('SELECT 1'))   # raw strings raise in SQLAlchemy 2.0
        db_status = 'connected'
    except Exception as e:
        db_status = f'error: {str(e)}'

    sha = None
    sha_file = os.path.join(os.path.dirname(__file__), '../../.git_sha')
    if os.path.exists(sha_file):
        with open(sha_file) as f:
            sha = f.read().strip()

    return {
        'status': 'healthy' if db_status == 'connected' else 'degraded',
        'database': db_status,
        'version': sha
    }
```

---

## Production Checklist

### Security

- [ ] `SECRET_KEY` is random (use `secrets.token_hex(32)`)
- [ ] `APP_ENV=production` (not `dev`)
- [ ] HTTPS enabled (Caddy handles this)
- [ ] CSRF protection enabled
- [ ] Rate limiting on auth endpoints

### Database

- [ ] SQLite database in persistent location (`/opt/yourapp/data/`)
- [ ] Regular backups of database file
- [ ] Migrations auto-run on startup

### Application

- [ ] Gunicorn with multiple workers (`-w 4`)
- [ ] Health check endpoint working
- [ ] Structured logging enabled
- [ ] Error tracking configured (optional: Sentry)

### Infrastructure

- [ ] systemd service enabled and running
- [ ] Caddy configured with domain
- [ ] SSL certificate active
- [ ] Firewall configured (only 80, 443, 22)
- [ ] Log rotation configured

### Backup Strategy

```bash
# Simple SQLite backup (add to cron)
# /etc/cron.daily/backup-yourapp
#!/bin/bash
cp /opt/yourapp/data/app.db /backups/yourapp/app-$(date +%Y%m%d).db
find /backups/yourapp -mtime +7 -delete  # Keep 7 days
```

---

## Troubleshooting

### Service won't start

```bash
# Check logs
sudo journalctl -u yourapp -n 50

# Test manually
cd /opt/yourapp
uv run python -c "from app import create_app; create_app()"
```

### Permission errors

```bash
sudo chown -R www-data:www-data /opt/yourapp
sudo chmod 755 /opt/yourapp/data
```

### Caddy not serving

```bash
# Check Caddy status
sudo systemctl status caddy

# Check Caddy logs
sudo journalctl -u caddy -n 50

# Validate Caddyfile
caddy validate --config /etc/caddy/Caddyfile
```

---

**Next:** [Security](security.md) | [Testing](testing.md)
