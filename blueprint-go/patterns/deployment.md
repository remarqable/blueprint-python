# Deployment Patterns

> Complete guide to deploying Blueprint Go applications.

---

## Overview

Blueprint Go applications can be deployed as:

- Single binary (compiled Go executable)
- Docker container
- Systemd service
- Cloud platforms (Railway, Fly.io, etc.)

---

## Building

### Development Build

```bash
# Build for current platform
make build
# Output: bin/blueprint

# Run directly
go run cmd/server/main.go
```

### Production Build

```bash
# Optimized build with smaller binary
make prod
# Output: bin/blueprint (stripped, optimized)

# Or manually
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/blueprint cmd/server/main.go
```

### Cross-Compilation

```bash
# Linux (most servers)
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o bin/blueprint-linux cmd/server/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o bin/blueprint-darwin cmd/server/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/blueprint.exe cmd/server/main.go
```

**Note**: CGO is required for SQLite. For cross-compilation with SQLite, use Docker or a cross-compiler.

---

## Environment Configuration

### .env File

```bash
# .env.production
SECRET_KEY=your-very-long-random-secret-key-here
DATABASE_URL=./data/app.db
DEBUG=false
PORT=8000
GIN_MODE=release
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECRET_KEY` | Session encryption key | `dev` (insecure) |
| `DATABASE_URL` | SQLite file path | `app.db` |
| `DEBUG` | Enable debug mode | `false` |
| `PORT` | HTTP port | `8000` |
| `GIN_MODE` | Gin mode (`debug`/`release`) | `debug` |

---

## Docker Deployment

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /blueprint cmd/server/main.go

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /blueprint .

# Copy templates and static files
COPY --from=builder /app/internal/modules/*/views ./internal/modules/

# Create data directory
RUN mkdir -p /app/data

# Set environment
ENV GIN_MODE=release
ENV PORT=8000

EXPOSE 8000

CMD ["./blueprint"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8000:8000"
    environment:
      - SECRET_KEY=${SECRET_KEY}
      - DATABASE_URL=/app/data/app.db
      - DEBUG=false
      - GIN_MODE=release
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

### Docker Commands

```bash
# Build image
docker build -t blueprint-go .

# Run container
docker run -d \
  -p 8000:8000 \
  -e SECRET_KEY=your-secret-key \
  -v $(pwd)/data:/app/data \
  --name blueprint \
  blueprint-go

# View logs
docker logs -f blueprint

# Stop
docker stop blueprint
```

---

## Systemd Service

### Service File

```ini
# /etc/systemd/system/blueprint.service
[Unit]
Description=Blueprint Go Application
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/blueprint
ExecStart=/opt/blueprint/bin/blueprint
Restart=on-failure
RestartSec=5

# Environment
Environment=SECRET_KEY=your-secret-key
Environment=DATABASE_URL=/opt/blueprint/data/app.db
Environment=DEBUG=false
Environment=GIN_MODE=release
Environment=PORT=8000

# Security
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/blueprint/data

[Install]
WantedBy=multi-user.target
```

### Installation

```bash
# Create directory
sudo mkdir -p /opt/blueprint/data
sudo chown -R www-data:www-data /opt/blueprint

# Copy binary and assets
sudo cp bin/blueprint /opt/blueprint/bin/
sudo cp -r internal/modules /opt/blueprint/

# Install service
sudo cp blueprint.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable blueprint
sudo systemctl start blueprint

# Check status
sudo systemctl status blueprint
sudo journalctl -u blueprint -f
```

---

## Reverse Proxy

### Nginx Configuration

```nginx
# /etc/nginx/sites-available/blueprint
server {
    listen 80;
    server_name yourdomain.com;

    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    # SSL certificates (use Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # SSL settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    # Proxy to Go app
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }

    # Static files (optional optimization)
    location /static/ {
        alias /opt/blueprint/static/;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

### Caddy Configuration

```caddyfile
yourdomain.com {
    reverse_proxy localhost:8000
}
```

---

## Cloud Deployment

### Railway

```toml
# railway.toml
[build]
builder = "dockerfile"

[deploy]
startCommand = "./blueprint"
healthcheckPath = "/health"
```

### Fly.io

```toml
# fly.toml
app = "blueprint-go"
primary_region = "ord"

[build]
  dockerfile = "Dockerfile"

[env]
  GIN_MODE = "release"
  PORT = "8080"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 256
```

### Render

```yaml
# render.yaml
services:
  - type: web
    name: blueprint-go
    env: docker
    dockerfilePath: ./Dockerfile
    envVars:
      - key: SECRET_KEY
        generateValue: true
      - key: GIN_MODE
        value: release
```

---

## Database Management

### SQLite in Production

```bash
# Backup database
sqlite3 data/app.db ".backup 'backup-$(date +%Y%m%d).db'"

# Or using cp (safe when app is stopped)
cp data/app.db "backups/app-$(date +%Y%m%d).db"
```

### Migrations

```bash
# Run migrations
make migrate-up

# Check status
make migrate-status

# Rollback
make migrate-down
```

### Backup Script

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/opt/blueprint/backups"
DB_PATH="/opt/blueprint/data/app.db"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup
sqlite3 "$DB_PATH" ".backup '$BACKUP_DIR/app_$DATE.db'"

# Keep only last 7 days
find "$BACKUP_DIR" -name "app_*.db" -mtime +7 -delete
```

Add to crontab:
```bash
0 2 * * * /opt/blueprint/backup.sh
```

---

## Health Check

Add a health endpoint:

```go
// In routes
r.GET("/health", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "healthy",
        "time":   time.Now().UTC(),
    })
})
```

---

## Logging

### Production Logging

```go
// In app initialization
if os.Getenv("GIN_MODE") == "release" {
    gin.SetMode(gin.ReleaseMode)
    // Log to file
    f, _ := os.OpenFile("logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
}
```

### Log Rotation

```bash
# /etc/logrotate.d/blueprint
/opt/blueprint/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    postrotate
        systemctl reload blueprint
    endscript
}
```

---

## Security Checklist

Before deploying to production:

- [ ] Set strong `SECRET_KEY` (32+ random characters)
- [ ] Set `GIN_MODE=release`
- [ ] Set `DEBUG=false`
- [ ] Enable HTTPS (TLS certificates)
- [ ] Configure firewall (only expose port 80/443)
- [ ] Set proper file permissions
- [ ] Enable automatic backups
- [ ] Set up log rotation
- [ ] Configure rate limiting (in nginx)
- [ ] Enable HSTS headers

---

## Monitoring

### Basic Metrics

```go
// Add to app
import "expvar"

func init() {
    expvar.NewInt("requests_total")
    expvar.NewInt("requests_errors")
}

// Middleware
func MetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        expvar.Get("requests_total").(*expvar.Int).Add(1)
        c.Next()
        if c.Writer.Status() >= 400 {
            expvar.Get("requests_errors").(*expvar.Int).Add(1)
        }
    }
}
```

### External Monitoring

- **Uptime**: UptimeRobot, Pingdom
- **Errors**: Sentry, Rollbar
- **Metrics**: Prometheus + Grafana
- **Logs**: Loki, Papertrail

---

## Quick Deploy Commands

```bash
# Full deployment
./deploy.sh

# deploy.sh
#!/bin/bash
set -e

echo "Building..."
make prod

echo "Stopping service..."
sudo systemctl stop blueprint

echo "Deploying..."
sudo cp bin/blueprint /opt/blueprint/bin/
sudo cp -r internal/modules /opt/blueprint/

echo "Running migrations..."
cd /opt/blueprint && ./bin/blueprint migrate

echo "Starting service..."
sudo systemctl start blueprint

echo "Deployed successfully!"
```
