#!/bin/bash
set -e

# roji installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash
#
# Options:
#   --upgrade    Force upgrade mode (skip version check prompt)
#   --reinstall  Force fresh installation (removes existing installation)

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="${ROJI_INSTALL_DIR:-$HOME/.roji}"
NETWORK_NAME="roji"
DOMAIN="dev.localhost"
DASHBOARD_HOST="roji.dev.localhost"
GITHUB_REPO="kan/roji"

# Parse command line arguments
FORCE_UPGRADE=false
FORCE_REINSTALL=false
for arg in "$@"; do
    case $arg in
        --upgrade)
            FORCE_UPGRADE=true
            ;;
        --reinstall)
            FORCE_REINSTALL=true
            ;;
    esac
done

# Print colored message
print_info() {
    echo -e "${CYAN}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

# Print banner
print_banner() {
    echo ""
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}  🛤️  ${BLUE}roji${NC} - Reverse proxy for local development           ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Check if Docker is installed and running
check_docker() {
    print_info "Checking Docker installation..."

    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        echo ""
        echo "Please install Docker first:"
        echo "  https://docs.docker.com/get-docker/"
        echo ""
        exit 1
    fi

    print_success "Docker is installed"

    print_info "Checking if Docker daemon is running..."

    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running"
        echo ""
        echo "Please start Docker and try again."
        echo ""
        exit 1
    fi

    print_success "Docker daemon is running"
}

# Check if Docker Compose is installed
check_docker_compose() {
    print_info "Checking Docker Compose installation..."

    if ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed"
        echo ""
        echo "Please install Docker Compose first:"
        echo "  https://docs.docker.com/compose/install/"
        echo ""
        exit 1
    fi

    print_success "Docker Compose is installed"
}

# Check if roji is already installed
check_existing_installation() {
    if [ -f "${INSTALL_DIR}/docker-compose.yml" ]; then
        return 0  # Installation exists
    fi
    return 1  # No installation
}

# Get current installed version
get_current_version() {
    local version=""

    # Try to get version from running container
    if docker ps --filter "name=roji" --format "{{.ID}}" | grep -q .; then
        version=$(docker inspect roji --format '{{.Config.Image}}' 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    fi

    # If not running, try to get from docker-compose.yml
    if [ -z "$version" ] && [ -f "${INSTALL_DIR}/docker-compose.yml" ]; then
        version=$(grep -oE 'ghcr\.io/kan/roji:[0-9]+\.[0-9]+\.[0-9]+' "${INSTALL_DIR}/docker-compose.yml" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    fi

    # Default to "latest" if we can't determine version
    if [ -z "$version" ]; then
        version="latest"
    fi

    echo "$version"
}

# Get latest version from GitHub
get_latest_version() {
    local version=""

    # Try to get latest release from GitHub API
    if command -v curl &> /dev/null; then
        version=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep -oE '"tag_name":\s*"v?[0-9]+\.[0-9]+\.[0-9]+"' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    fi

    # If can't get from API, use "latest"
    if [ -z "$version" ]; then
        version="latest"
    fi

    echo "$version"
}

# Compare versions (returns 0 if v1 < v2, 1 otherwise)
version_lt() {
    local v1="$1"
    local v2="$2"

    # Handle "latest" specially
    if [ "$v1" = "latest" ] || [ "$v2" = "latest" ]; then
        return 1  # Can't compare, assume no upgrade needed
    fi

    # Compare versions
    if [ "$v1" = "$v2" ]; then
        return 1  # Same version
    fi

    # Use sort -V for version comparison
    local smaller=$(echo -e "$v1\n$v2" | sort -V | head -n1)
    if [ "$smaller" = "$v1" ]; then
        return 0  # v1 < v2
    fi
    return 1
}

# Backup current configuration
backup_config() {
    local backup_dir="${INSTALL_DIR}/backups/$(date +%Y%m%d_%H%M%S)"

    print_info "Backing up current configuration..."
    mkdir -p "$backup_dir"

    if [ -f "${INSTALL_DIR}/docker-compose.yml" ]; then
        cp "${INSTALL_DIR}/docker-compose.yml" "$backup_dir/"
    fi
    if [ -f "${INSTALL_DIR}/.env" ]; then
        cp "${INSTALL_DIR}/.env" "$backup_dir/"
    fi

    print_success "Configuration backed up to $backup_dir"
    echo "$backup_dir"
}

# Migrate configuration if needed
migrate_config() {
    local compose_file="${INSTALL_DIR}/docker-compose.yml"
    local needs_migration=false
    local migration_notes=""

    print_info "Checking for configuration updates..."

    # Check for missing data volume
    if ! grep -q "./data:/data" "$compose_file" 2>/dev/null; then
        needs_migration=true
        migration_notes="${migration_notes}\n  - Added data volume for project history"
    fi

    # Check for missing ROJI_DATA_DIR environment variable
    if ! grep -q "ROJI_DATA_DIR" "$compose_file" 2>/dev/null; then
        needs_migration=true
        migration_notes="${migration_notes}\n  - Added ROJI_DATA_DIR environment variable"
    fi

    if [ "$needs_migration" = true ]; then
        print_warning "Configuration migration needed"
        echo -e "The following changes will be applied:${migration_notes}"
        echo ""

        # Create data directory
        mkdir -p "${INSTALL_DIR}/data"

        # Regenerate docker-compose.yml with new settings
        create_compose_file

        print_success "Configuration migrated successfully"
    else
        print_success "Configuration is up to date"
    fi
}

# Update image tag in docker-compose.yml
update_image_tag() {
    local new_version="$1"
    local compose_file="${INSTALL_DIR}/docker-compose.yml"

    print_info "Updating image tag to ${new_version}..."

    # Update the image tag
    if [ "$new_version" = "latest" ]; then
        sed -i.bak "s|ghcr\.io/kan/roji:[^ ]*|ghcr.io/kan/roji:latest|g" "$compose_file"
    else
        sed -i.bak "s|ghcr\.io/kan/roji:[^ ]*|ghcr.io/kan/roji:${new_version}|g" "$compose_file"
    fi
    rm -f "${compose_file}.bak"

    print_success "Image tag updated"
}

# Perform upgrade
perform_upgrade() {
    local current_version="$1"
    local latest_version="$2"

    echo ""
    echo -e "${CYAN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}  🔄 ${BLUE}Upgrading roji${NC}                                         ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "  Current version: ${YELLOW}${current_version}${NC}"
    echo -e "  Latest version:  ${GREEN}${latest_version}${NC}"
    echo ""

    # Backup configuration
    local backup_dir=$(backup_config)

    # Migrate configuration if needed
    migrate_config

    # Update image tag
    update_image_tag "$latest_version"

    # Pull new image
    print_info "Pulling new image..."
    cd "${INSTALL_DIR}"
    docker compose pull
    print_success "New image pulled"

    # Restart with new image
    print_info "Restarting roji..."
    docker compose up -d
    print_success "roji restarted"

    # Wait for roji to be ready
    print_info "Waiting for roji to be ready..."
    sleep 3

    # Check if roji is running
    if docker ps --filter "name=roji" --filter "status=running" --format "{{.ID}}" | grep -q .; then
        echo ""
        echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║${NC}  🎉 ${BLUE}Upgrade completed successfully!${NC}                        ${GREEN}║${NC}"
        echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "  roji has been upgraded from ${YELLOW}${current_version}${NC} to ${GREEN}${latest_version}${NC}"
        echo ""
        echo -e "${CYAN}Dashboard:${NC}"
        echo -e "  https://${DASHBOARD_HOST}"
        echo ""
        echo -e "${CYAN}Rollback instructions:${NC}"
        echo "  If you encounter issues, you can rollback to the previous version:"
        echo ""
        echo "    cd ${INSTALL_DIR}"
        echo "    cp ${backup_dir}/docker-compose.yml ."
        echo "    docker compose pull"
        echo "    docker compose up -d"
        echo ""
    else
        print_error "roji failed to start after upgrade"
        echo ""
        echo -e "${YELLOW}Rollback instructions:${NC}"
        echo "  To restore the previous version:"
        echo ""
        echo "    cd ${INSTALL_DIR}"
        echo "    cp ${backup_dir}/docker-compose.yml ."
        echo "    docker compose pull"
        echo "    docker compose up -d"
        echo ""
        exit 1
    fi
}

# Handle existing installation
handle_existing_installation() {
    local current_version=$(get_current_version)
    local latest_version=$(get_latest_version)

    echo ""
    echo -e "${YELLOW}Existing roji installation detected at ${INSTALL_DIR}${NC}"
    echo ""
    echo -e "  Current version: ${YELLOW}${current_version}${NC}"
    echo -e "  Latest version:  ${GREEN}${latest_version}${NC}"
    echo ""

    # Check if force reinstall requested
    if [ "$FORCE_REINSTALL" = true ]; then
        print_warning "Force reinstall requested"
        echo ""
        print_info "Stopping and removing existing installation..."
        cd "${INSTALL_DIR}"
        docker compose down 2>/dev/null || true
        rm -f docker-compose.yml .env
        return 1  # Proceed with fresh installation
    fi

    # Check if upgrade is needed
    if version_lt "$current_version" "$latest_version"; then
        if [ "$FORCE_UPGRADE" = true ]; then
            perform_upgrade "$current_version" "$latest_version"
            exit 0
        else
            echo "A newer version is available!"
            echo ""
            echo -e "${CYAN}Options:${NC}"
            echo "  1. Upgrade to latest version"
            echo "  2. Keep current version"
            echo "  3. Reinstall from scratch"
            echo ""

            # Check if running interactively
            if [ -t 0 ]; then
                read -p "Choose an option [1]: " choice
                choice=${choice:-1}

                case $choice in
                    1)
                        perform_upgrade "$current_version" "$latest_version"
                        exit 0
                        ;;
                    2)
                        echo ""
                        print_success "Keeping current version"
                        echo -e "  roji is running at https://${DASHBOARD_HOST}"
                        echo ""
                        exit 0
                        ;;
                    3)
                        print_warning "Reinstalling from scratch..."
                        echo ""
                        print_info "Stopping and removing existing installation..."
                        cd "${INSTALL_DIR}"
                        docker compose down 2>/dev/null || true
                        rm -f docker-compose.yml .env
                        return 1  # Proceed with fresh installation
                        ;;
                    *)
                        print_error "Invalid option"
                        exit 1
                        ;;
                esac
            else
                # Non-interactive mode: just upgrade
                perform_upgrade "$current_version" "$latest_version"
                exit 0
            fi
        fi
    else
        echo "roji is already up to date."
        echo ""

        # Check if container is running
        if docker ps --filter "name=roji" --filter "status=running" --format "{{.ID}}" | grep -q .; then
            print_success "roji is running at https://${DASHBOARD_HOST}"
        else
            print_warning "roji is not running"
            echo ""
            echo "To start roji:"
            echo "  cd ${INSTALL_DIR}"
            echo "  docker compose up -d"
        fi
        echo ""

        if [ "$FORCE_REINSTALL" != true ]; then
            exit 0
        fi
    fi
}

# Create installation directory
create_install_dir() {
    print_info "Creating installation directory at ${INSTALL_DIR}..."

    mkdir -p "${INSTALL_DIR}"
    cd "${INSTALL_DIR}"

    print_success "Installation directory created"
}

# Create roji network
create_network() {
    print_info "Creating Docker network '${NETWORK_NAME}'..."

    if docker network inspect "${NETWORK_NAME}" &> /dev/null; then
        print_warning "Network '${NETWORK_NAME}' already exists, skipping"
    else
        docker network create "${NETWORK_NAME}" > /dev/null
        print_success "Network '${NETWORK_NAME}' created"
    fi
}

# Create docker-compose.yml
create_compose_file() {
    print_info "Creating docker-compose.yml..."

    # Create directories
    mkdir -p "${INSTALL_DIR}/certs"
    mkdir -p "${INSTALL_DIR}/data"

    cat > docker-compose.yml <<'EOF'
services:
  roji:
    image: ghcr.io/kan/roji:latest
    container_name: roji
    restart: unless-stopped
    user: root  # Run as root to access Docker socket and write certificates
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./certs:/certs
      - ./data:/data
    environment:
      - ROJI_NETWORK=roji
      - ROJI_DOMAIN=dev.localhost
      - ROJI_CERTS_DIR=/certs
      - ROJI_DATA_DIR=/data
      - ROJI_AUTO_CERT=true
      - ROJI_DASHBOARD=roji.dev.localhost
      - ROJI_LOG_LEVEL=info
    networks:
      - roji
    labels:
      - "roji.self=true"
    healthcheck:
      test: ["CMD", "/roji", "health"]
      interval: 30s
      timeout: 3s
      start_period: 10s
      retries: 3

networks:
  roji:
    external: true
EOF

    print_success "docker-compose.yml created"
}

# Create .env file
create_env_file() {
    if [ -f .env ]; then
        print_warning ".env file already exists, skipping"
        return
    fi

    print_info "Creating .env file with default settings..."

    cat > .env <<'EOF'
# roji Configuration
ROJI_NETWORK=roji
ROJI_DOMAIN=dev.localhost
ROJI_CERTS_DIR=/certs
ROJI_AUTO_CERT=true
ROJI_DASHBOARD=dev.localhost
ROJI_LOG_LEVEL=info
EOF

    print_success ".env file created"
}

# Start roji
start_roji() {
    print_info "Starting roji..."

    # Start the containers (root user will handle permissions)
    docker compose up -d

    print_success "roji started"

    # Give roji a moment to generate certificates
    print_info "Initializing certificates..."
    sleep 2
}


# Detect OS for CA installation instructions
detect_os() {
    case "$(uname -s)" in
        Darwin*)
            echo "macos"
            ;;
        Linux*)
            echo "linux"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            echo "windows"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# Show CA installation instructions
show_ca_instructions() {
    local os=$(detect_os)

    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  🎉 ${BLUE}roji${NC} has been successfully installed!                  ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${CYAN}Dashboard:${NC}"
    echo -e "  https://${DASHBOARD_HOST}"
    echo ""

    echo -e "${CYAN}Installation directory:${NC}"
    echo -e "  ${INSTALL_DIR}"
    echo ""

    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}IMPORTANT: Trust the CA certificate to enable HTTPS${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}CA Certificate location:${NC}"

    case "$os" in
        macos)
            echo -e "  ${INSTALL_DIR}/certs/ca.pem"
            echo ""
            echo -e "${CYAN}Installation (macOS):${NC}"
            echo ""
            echo "  Option 1 - Using terminal (requires password):"
            echo -e "    ${GREEN}sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${INSTALL_DIR}/certs/ca.pem${NC}"
            echo ""
            echo "  Option 2 - Using Keychain Access:"
            echo "    1. Double-click: ${INSTALL_DIR}/certs/ca.pem"
            echo "    2. Select 'System' keychain"
            echo "    3. Right-click the certificate → 'Get Info'"
            echo "    4. Expand 'Trust' → Select 'Always Trust' for SSL"
            ;;

        linux)
            echo -e "  ${INSTALL_DIR}/certs/ca.pem"
            echo ""
            echo -e "${CYAN}Installation (Linux - Chrome/Chromium):${NC}"
            echo ""
            echo "  1. Install certutil if needed:"
            echo "     Debian/Ubuntu: sudo apt install libnss3-tools"
            echo "     Fedora: sudo dnf install nss-tools"
            echo ""
            echo "  2. Add to certificate store:"
            echo -e "     ${GREEN}certutil -d sql:\$HOME/.pki/nssdb -A -t \"C,,\" -n \"roji CA\" -i ${INSTALL_DIR}/certs/ca.pem${NC}"
            ;;

        windows)
            echo -e "  ${INSTALL_DIR}/certs/ca.crt"
            echo ""
            echo -e "${CYAN}Installation (Windows):${NC}"
            echo ""
            echo "  1. Double-click: ${INSTALL_DIR}\\certs\\ca.crt"
            echo "  2. Click 'Install Certificate'"
            echo "  3. Select 'Local Machine' (requires admin) or 'Current User'"
            echo "  4. Select 'Place all certificates in the following store'"
            echo "  5. Click 'Browse' → Select 'Trusted Root Certification Authorities'"
            echo "  6. Click 'Next' → 'Finish'"
            echo "  7. Restart your browser"
            ;;

        *)
            echo -e "  ${INSTALL_DIR}/certs/ca.pem"
            echo ""
            echo -e "${CYAN}Installation:${NC}"
            echo "  Please refer to the documentation:"
            echo "  https://github.com/kan/roji#tls-certificates"
            ;;
    esac

    echo ""
    echo -e "${CYAN}Firefox (all platforms):${NC}"
    echo "  1. Settings → Privacy & Security → Certificates → View Certificates"
    echo "  2. Authorities tab → Import"
    echo "  3. Select ${INSTALL_DIR}/certs/ca.pem"
    echo "  4. Check 'Trust this CA to identify websites'"
    echo ""

    echo -e "${YELLOW}After installing the certificate, restart your browser.${NC}"
    echo ""

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}Next steps:${NC}"
    echo ""
    echo "  1. Add your Docker Compose services to the 'roji' network:"
    echo ""
    echo "     services:"
    echo "       myapp:"
    echo "         image: your-app"
    echo "         networks:"
    echo "           - roji"
    echo ""
    echo "     networks:"
    echo "       roji:"
    echo "         external: true"
    echo ""
    echo "  2. Access your app at https://myapp.dev.localhost"
    echo ""

    echo -e "${CYAN}Useful commands:${NC}"
    echo ""
    echo "  View logs:       docker logs roji -f"
    echo "  Restart:         docker restart roji"
    echo "  Stop:            docker stop roji"
    echo "  Start:           docker start roji"
    echo "  Upgrade:         curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash"
    echo "  Uninstall:       cd ${INSTALL_DIR} && docker compose down && rm -rf ${INSTALL_DIR}"
    echo ""

    echo -e "${CYAN}Documentation:${NC}"
    echo "  https://github.com/kan/roji"
    echo ""
}

# Main installation flow
main() {
    print_banner

    check_docker
    check_docker_compose

    # Check for existing installation
    if check_existing_installation; then
        handle_existing_installation
    fi

    # Fresh installation
    create_install_dir
    create_network
    create_compose_file
    create_env_file
    start_roji
    show_ca_instructions
}

main "$@"
