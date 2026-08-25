#!/bin/bash
set -e

# ============================================================================
# Server Provisioning Script for Blueprint Flask Apps
# ============================================================================
# Run this script on a fresh Debian/Ubuntu server to set up all dependencies.
#
# Usage:
#   ssh root@your-server.com
#   curl -sL https://raw.githubusercontent.com/yourorg/blueprint/main/scripts/provision-server.sh | bash
#
# Or copy and run manually:
#   scp scripts/provision-server.sh root@your-server.com:/tmp/
#   ssh root@your-server.com "bash /tmp/provision-server.sh"
# ============================================================================

# Configuration - customize these
APP_NAME="${APP_NAME:-yourapp}"
APP_DIR="${APP_DIR:-/opt/$APP_NAME}"
APP_USER="${APP_USER:-www-data}"
APP_DOMAIN="${APP_DOMAIN:-yourapp.com}"
GIT_REPO="${GIT_REPO:-}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'
CHECK="${GREEN}✓${NC}"
CROSS="${RED}✗${NC}"
INFO="${YELLOW}→${NC}"

echo ""
echo "============================================"
echo "  Blueprint Server Provisioning"
echo "============================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${CROSS} Please run as root"
    exit 1
fi

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    VERSION=$VERSION_ID
else
    echo -e "${CROSS} Cannot detect OS"
    exit 1
fi

echo -e "${INFO} Detected: $OS $VERSION"
echo ""

# ============================================================================
# 1. System Updates
# ============================================================================
echo -e "${INFO} Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq
echo -e "${CHECK} System updated"

# ============================================================================
# 2. Install Python + uv
# ============================================================================
echo -e "${INFO} Installing Python and uv..."
apt-get install -y -qq python3 curl
# System-wide so root, deploy scripts, and systemd all find it.
# uv downloads a managed Python if the system one is older than 3.11.
curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh
echo -e "${CHECK} Python $(python3 --version | cut -d' ' -f2) and uv $(uv --version | cut -d' ' -f2) installed"

# ============================================================================
# 3. Install Git
# ============================================================================
echo -e "${INFO} Installing Git..."
apt-get install -y -qq git
echo -e "${CHECK} Git installed"

# ============================================================================
# 4. Install Caddy
# ============================================================================
echo -e "${INFO} Installing Caddy..."
apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl

# Add Caddy GPG key and repo
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' 2>/dev/null | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg 2>/dev/null || true
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' 2>/dev/null | tee /etc/apt/sources.list.d/caddy-stable.list > /dev/null

apt-get update -qq
apt-get install -y -qq caddy
echo -e "${CHECK} Caddy installed"

# ============================================================================
# 5. Install useful utilities
# ============================================================================
echo -e "${INFO} Installing utilities..."
apt-get install -y -qq \
    ufw \
    fail2ban \
    htop \
    ncdu \
    jq
echo -e "${CHECK} Utilities installed"

# ============================================================================
# 6. Configure Firewall
# ============================================================================
echo -e "${INFO} Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow http
ufw allow https
ufw --force enable
echo -e "${CHECK} Firewall configured (SSH, HTTP, HTTPS allowed)"

# ============================================================================
# 7. Create app directory
# ============================================================================
echo -e "${INFO} Creating app directory..."
mkdir -p "$APP_DIR"
mkdir -p "$APP_DIR/data"
echo -e "${CHECK} Created $APP_DIR"

# ============================================================================
# 8. Clone repository (if GIT_REPO provided)
# ============================================================================
if [ -n "$GIT_REPO" ]; then
    echo -e "${INFO} Cloning repository..."
    git clone "$GIT_REPO" "$APP_DIR" 2>/dev/null || {
        echo -e "${YELLOW}!${NC} Directory not empty, pulling instead..."
        cd "$APP_DIR" && git pull
    }
    echo -e "${CHECK} Repository cloned"
fi

# ============================================================================
# 9. Create virtual environment
# ============================================================================
if [ -d "$APP_DIR" ] && [ -f "$APP_DIR/pyproject.toml" ]; then
    echo -e "${INFO} Installing dependencies..."
    cd "$APP_DIR"
    uv sync --locked --no-dev -q
    echo -e "${CHECK} Virtual environment created and locked dependencies installed"
fi

# ============================================================================
# 10. Create systemd service
# ============================================================================
echo -e "${INFO} Creating systemd service..."
cat > /etc/systemd/system/$APP_NAME.service <<EOF
[Unit]
Description=$APP_NAME Flask Application
After=network.target

[Service]
User=$APP_USER
Group=$APP_USER
WorkingDirectory=$APP_DIR
Environment="PATH=$APP_DIR/.venv/bin"
EnvironmentFile=$APP_DIR/.env
# Migrations run once, before any worker starts. Must exit 0 or the
# unit fails -- better than serving against a half-migrated schema.
ExecStartPre=$APP_DIR/.venv/bin/flask db upgrade
ExecStart=$APP_DIR/.venv/bin/gunicorn wsgi:app -w 4 -b 127.0.0.1:8000
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
echo -e "${CHECK} Systemd service created: $APP_NAME.service"

# ============================================================================
# 11. Create Caddyfile
# ============================================================================
echo -e "${INFO} Creating Caddyfile..."
cat > /etc/caddy/Caddyfile <<EOF
$APP_DOMAIN {
    reverse_proxy localhost:8000
}

www.$APP_DOMAIN {
    redir https://$APP_DOMAIN{uri} permanent
}
EOF
echo -e "${CHECK} Caddyfile created for $APP_DOMAIN"

# ============================================================================
# 12. Set permissions
# ============================================================================
echo -e "${INFO} Setting permissions..."
chown -R $APP_USER:$APP_USER "$APP_DIR"
chmod 755 "$APP_DIR/data"
echo -e "${CHECK} Permissions set"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "============================================"
echo -e "${GREEN}  Provisioning Complete!${NC}"
echo "============================================"
echo ""
echo "Installed:"
echo "  - Python $(python3 --version | cut -d' ' -f2)"
echo "  - uv $(uv --version 2>/dev/null | cut -d' ' -f2 || echo 'installed')"
echo "  - Git $(git --version | cut -d' ' -f3)"
echo "  - Caddy $(caddy version 2>/dev/null | head -1 || echo 'installed')"
echo "  - UFW firewall (SSH, HTTP, HTTPS)"
echo "  - fail2ban"
echo ""
echo "Next steps:"
echo ""
echo "  1. Clone your repo (if not done):"
echo "     cd $APP_DIR && git clone YOUR_REPO ."
echo ""
echo "  2. Install dependencies:"
echo "     uv sync --locked --no-dev"
echo ""
echo "  3. Create .env file:"
echo "     cat > $APP_DIR/.env <<EOF"
echo "     APP_ENV=production"
echo "     PORT=8000"
echo "     SECRET_KEY=\$(python3 -c \"import secrets; print(secrets.token_hex(32))\")"
echo "     DATABASE_URL=sqlite:///data/app.db"
echo "     EOF"
echo ""
echo "  4. Update Caddyfile domain:"
echo "     nano /etc/caddy/Caddyfile"
echo ""
echo "  5. Start services:"
echo "     systemctl enable $APP_NAME"
echo "     systemctl start $APP_NAME"
echo "     systemctl reload caddy"
echo ""
echo "  6. Verify:"
echo "     systemctl status $APP_NAME"
echo "     curl -s https://$APP_DOMAIN/health"
echo ""
