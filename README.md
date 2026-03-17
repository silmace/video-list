# Video List

Video List is a web-based file manager with media streaming and video editing capabilities. It's designed for remote deployment on NAS servers and provides secure, password-protected access to manage and edit video files.

**Key Features:**
- 🎥 Web-based video file manager and streamer
- ✏️ Video editing and batch processing support
- 🔒 Optional password protection with brute-force defense
- 📝 Comprehensive audit logging
- 🐳 Docker support for easy deployment
- 🔐 Security-hardened with proper file permissions
- 📱 Responsive web interface

## Quick Start

### Binary Installation (Linux)

```bash
# Download and run the installation script
sudo bash install.sh [version]

# Or install manually:
# 1. Download binary from releases
# 2. Run: sudo ./video-list --config /etc/video-list/config.yaml
```

### Docker Deployment

```bash
# Using docker-compose (recommended)
docker-compose up -d

# Or build and run manually
docker build -t video-list .
docker run -d \
  -p 3001:3001 \
  -v video-list-config:/app/config \
  -v video-list-logs:/app/logs \
  --name video-list \
  video-list
```

### Manual Build & Run

```bash
# Prerequisites: Node.js 18+, Go 1.21+

git clone https://github.com/silmace/video-list.git
cd video-list

# Build frontend
npm install
npm run build

# Build backend
go build -o video-list .

# Run
./video-list
```

## Configuration

Configuration is managed via YAML. The default paths are:

- **Windows:** `%APPDATA%/video-list/config.yaml`
- **Linux/macOS:** `~/.video-list/config.yaml`
- **Docker:** `/app/config/config.yaml`

The configuration file is automatically created with defaults on first run. Here's an example:

```yaml
# Directory for media files (must be configured in config.yaml only)
baseDir: /home/user/Videos

# Directory for processed/output videos
videoOutputDir: /home/user/Videos/output

# Show hidden files and folders in the interface
showHiddenItems: false

# Logging configuration
logDir: /var/log/video-list
logLevel: info
logRotationHours: 24
logMaxAgeDays: 7

# Task polling interval (milliseconds)
taskPollIntervalMs: 1500

# Password hash (leave empty to disable authentication)
# Set via the web interface - never edit directly
passwordHash: ""
```

### Command-Line Flags

```bash
./video-list [options]

Options:
  -config string
      Path to config file (default: auto-detected)
  -h, -help
      Show this help message
```

## Usage

1. **Start the application:**
   ```bash
   ./video-list
   ```

2. **Access the web interface:**
   - Open http://localhost:3001 in your browser

3. **Set password (optional):**
   - Navigate to Settings tab
   - Enter and confirm a password (minimum 6 characters)
   - The password is hashed using bcrypt and never stored in plain text

4. **Manage files:**
   - Browse, upload, rename, and delete files
   - Edit videos with the built-in editor
   - Process videos in batch

## Security

### Authentication & Brute-Force Protection

- **Password Protection:** Optional password protection with bcrypt hashing
- **Brute-Force Defense:** After 5 failed login attempts, the IP is blocked for 30 minutes
- **Session Management:** Secure token-based sessions with 24-hour TTL
- **Request ID Sanitization:** All request IDs are sanitized to prevent log injection attacks

### File Permissions

- **Config File:** 600 (owner read/write only) - prevents password hash exposure
- **Log Directory:** 700 (owner access only)
- **Application:** Runs as unprivileged user when using install.sh

### Systemd Security Hardening (Linux)

When installed via install.sh, the service runs with:

```ini
[Service]
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
RestrictRealtime=true
RestrictSUIDSGID=true
RemoveIPC=true
```

### Docker Security

When using the provided Dockerfile/docker-compose:

- Runs as non-root user (uid: 1000)
- Read-only root filesystem
- Security options: `no-new-privileges=true`
- Network isolation via bridge network

## Development

### Prerequisites
- Node.js 18+
- Go 1.21+
- Git

### Build Commands

```bash
# Install dependencies
npm install

# Development server with hot reload
npm run dev

# Build frontend
npm run build

# Run tests
npm run test
go test ./...

# Lint
npm run lint
go vet ./...

# Security audit
npm audit
go list -json -m all | nancy sleuth
```

### Project Structure

```
video-list/
├── main.go                 # Entry point
├── internal/app/          # Backend application logic
│   ├── server.go
│   ├── auth_session.go
│   ├── config_settings.go
│   ├── logger.go
│   └── ...
├── src/                   # Frontend (Vue 3 + TypeScript)
│   ├── components/
│   ├── views/
│   ├── services/
│   └── ...
├── dist/                  # Built frontend (embedded in binary)
├── Dockerfile             # Container image
├── docker-compose.yml     # Local deployment config
├── install.sh             # System installation script
└── README.md
```

## Troubleshooting

### Application won't start

```bash
# Check if port 3001 is in use
lsof -i :3001

# Run with debug logging
./video-list --config config.yaml
# Then check logs at configured logDir
```

### Permission denied errors

```bash
# Fix ownership and permissions
sudo chown -R video-list:video-list /opt/video-list
sudo chmod 700 /opt/video-list
sudo chmod 600 /opt/video-list/config.yaml
```

### Docker container exits

```bash
# View logs
docker logs video-list

# Check service health
docker inspect --format='{{.State.Health.Status}}' video-list

# Restart container
docker restart video-list
```

## Performance Tuning

### For Large Video Directories

1. Increase task polling interval:
   ```yaml
   taskPollIntervalMs: 3000  # Increase from default 1500
   ```

2. Enable log compression:
   ```yaml
   logDir: /var/log/video-list
   logMaxAgeDays: 3  # Keep fewer old logs
   ```

### For Remote Access

1. Use a reverse proxy (nginx, Apache, etc.)
2. Enable HTTPS/TLS
3. Restrict access by IP when possible
4. Use strong passwords and enable authentication

## Logging

Logs are written to the configured `logDir` (default: next to config file). Log files are:

- **Rotated daily** (configurable)
- **Kept for 7 days** (configurable)
- **In JSON format** for easy parsing
- **Accessible at:** `tail -f <logDir>/video-list.log*`

Key log fields:
- `timestamp`: ISO 8601 UTC time
- `level`: debug, info, warn, error
- `message`: Human-readable message
- `request_id`: Unique request identifier
- Additional context fields for each event

## Building Releases

```bash
# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o video-list-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o video-list-linux-arm64 .
GOOS=windows GOARCH=amd64 go build -o video-list-windows-amd64.exe .
GOOS=darwin GOARCH=amd64 go build -o video-list-darwin-amd64 .
```

## License

MIT