#!/bin/bash
set -e

# Configuration - customize these for your app
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
git push -q origin master 2>/dev/null || true
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

# Install dependencies
ssh $REMOTE "cd $REMOTE_DIR && ./venv/bin/pip install -q -r requirements.txt"
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
