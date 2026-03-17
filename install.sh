#!/bin/bash

#
# video-list Installation Script
# Installs video-list application with proper security configurations
#
# Usage: sudo bash install.sh [version]
# Default version: latest
#

set -e

INSTALL_VERSION="${1:-latest}"
APP_NAME="video-list"
INSTALL_DIR="/opt/video-list"
BIN_NAME="video-list"
USER_NAME="video-list"
GROUP_NAME="video-list"
LOG_DIR="/var/log/video-list"
CONFIG_DIR="/etc/video-list"
SYSTEMD_SERVICE="/etc/systemd/system/video-list.service"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

check_dependencies() {
    log_info "Checking dependencies..."
    
    local missing_deps=()
    
    for cmd in curl systemctl uname; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies: ${missing_deps[*]}"
        exit 1
    fi
    
    log_info "All dependencies satisfied"
}

detect_architecture() {
    log_info "Detecting system architecture..."
    
    local arch
    local os

    arch="$(uname -m)" || {
        log_error "Failed to detect architecture"
        exit 1
    }
    os="$(uname -s)" || {
        log_error "Failed to detect operating system"
        exit 1
    }
    
    # Validate operating system
    if [ "$os" != "Linux" ]; then
        log_error "Unsupported OS: $os. This script only supports Linux."
        exit 1
    fi
    
    # Map architecture
    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armhf)
            ARCH="armv7"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            log_info "Supported architectures: amd64, arm64, armv7"
            exit 1
            ;;
    esac
    
    log_info "Detected architecture: $ARCH"
}

download_binary() {
    log_info "Downloading $APP_NAME binary (version: $INSTALL_VERSION) for $ARCH..."
    
    # Construct download URL based on architecture
    local url=""
    local binary_name="${BIN_NAME}-linux-${ARCH}"
    
    if [ "$INSTALL_VERSION" = "latest" ]; then
        url="https://github.com/silmace/video-list/releases/latest/download/${binary_name}"
    else
        url="https://github.com/silmace/video-list/releases/download/${INSTALL_VERSION}/${binary_name}"
    fi
    
    log_info "Downloading from: $url"
    
    if ! curl -fsSL --fail --show-error "$url" -o "$INSTALL_DIR/$BIN_NAME"; then
        log_error "Failed to download binary for architecture: $ARCH"
        log_error "Please check that the release exists at: $url"
        exit 1
    fi
    
    # Verify binary exists and is executable
    if [ ! -f "$INSTALL_DIR/$BIN_NAME" ]; then
        log_error "Downloaded file is missing"
        exit 1
    fi
    
    # Set permissions on binary
    chmod 755 "$INSTALL_DIR/$BIN_NAME"
    chown "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR/$BIN_NAME"
    
    log_info "Binary downloaded successfully for $ARCH"
}

create_user() {
    log_info "Creating system user: $USER_NAME"
    
    if id "$USER_NAME" &> /dev/null; then
        log_warn "User $USER_NAME already exists"
    else
        useradd -r -s /bin/false -d "$INSTALL_DIR" "$USER_NAME"
        log_info "User created successfully"
    fi
}

create_directories() {
    log_info "Creating directories..."
    
    # Install directory
    mkdir -p "$INSTALL_DIR"
    chmod 755 "$INSTALL_DIR"
    chown "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR"
    
    # Config directory
    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    chown "$USER_NAME:$GROUP_NAME" "$CONFIG_DIR"
    
    # Log directory
    mkdir -p "$LOG_DIR"
    chmod 700 "$LOG_DIR"
    chown "$USER_NAME:$GROUP_NAME" "$LOG_DIR"
    
    log_info "Directories created successfully"
}

create_systemd_service() {
    log_info "Creating systemd service..."
    
    cat > "$SYSTEMD_SERVICE" << 'EOF'
[Unit]
Description=video-list - Web-based video file manager
After=network.target

[Service]
Type=simple
User=video-list
Group=video-list
WorkingDirectory=/opt/video-list

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/video-list /etc/video-list
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictSUIDSGID=true
RemoveIPC=true

# Runtime
ExecStart=/opt/video-list/video-list --config /etc/video-list/config.yaml
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=video-list

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 "$SYSTEMD_SERVICE"
    log_info "Systemd service created successfully"
}

create_sample_config() {
    log_info "Creating sample configuration file..."
    
    local config_file="$CONFIG_DIR/config.yaml"
    
    if [ -f "$config_file" ]; then
        log_warn "Configuration file already exists at $config_file"
        return
    fi
    
    cat > "$config_file" << 'EOF'
# video-list Configuration
# Adjust these settings as needed

# Video output directory for processed videos
videoOutputDir: /home/user/Videos/output

# Show hidden files/folders
showHiddenItems: false

# Logging configuration
logDir: /var/log/video-list
logLevel: info
logRotationHours: 24
logMaxAgeDays: 7

# Task management
taskPollIntervalMs: 1500

# Password protection (optional)
# Leave empty to disable authentication
# You must set a password through the web interface first time you run the app
passwordHash: ""
EOF

    chmod 600 "$config_file"
    chown "$USER_NAME:$GROUP_NAME" "$config_file"
    log_info "Sample configuration created at $config_file (permissions: 600)"
}

enable_service() {
    log_info "Enabling systemd service..."
    
    systemctl daemon-reload
    systemctl enable video-list.service
    
    log_info "Service enabled successfully"
}

print_summary() {
    log_info "Installation completed successfully!"
    echo ""
    echo "=========================================="
    echo "Installation Summary"
    echo "=========================================="
    echo "Binary location:      $INSTALL_DIR/$BIN_NAME"
    echo "Configuration:        $CONFIG_DIR/config.yaml"
    echo "Logs:                 $LOG_DIR/"
    echo "Systemd service:      video-list.service"
    echo ""
    echo "Next steps:"
    echo "1. Edit configuration: sudo nano $CONFIG_DIR/config.yaml"
    echo "2. Start service:      sudo systemctl start video-list"
    echo "3. Check status:       sudo systemctl status video-list"
    echo "4. View logs:          sudo journalctl -u video-list -f"
    echo ""
    echo "Access the application at: http://localhost:3001"
    echo "=========================================="
}

print_security_notes() {
    log_info "Security Configuration Notes:"
    echo ""
    echo "✓ App runs as unprivileged user: $USER_NAME"
    echo "✓ Configuration directory permissions: 700 (owner only)"
    echo "✓ Config file permissions: 600 (prevents password hash exposure)"
    echo "✓ Systemd service has security hardening enabled:"
    echo "  - NoNewPrivileges=true"
    echo "  - PrivateTmp=true"
    echo "  - ProtectSystem=strict"
    echo "  - ProtectHome=true"
    echo "✓ Logs stored with restricted permissions"
    echo ""
}

# Main installation flow
main() {
    log_info "Starting $APP_NAME installation..."
    
    check_root
    check_dependencies
    detect_architecture
    create_user
    create_directories
    download_binary
    create_systemd_service
    create_sample_config
    enable_service
    
    print_security_notes
    print_summary
}

# Run main function
main "$@"
