#!/bin/bash
set -e

# roji installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash

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
DOMAIN="localhost"
DASHBOARD_HOST="roji.localhost"

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

    # Create certs directory
    mkdir -p "${INSTALL_DIR}/certs"

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
    environment:
      - ROJI_NETWORK=roji
      - ROJI_DOMAIN=localhost
      - ROJI_CERTS_DIR=/certs
      - ROJI_AUTO_CERT=true
      - ROJI_DASHBOARD=roji.localhost
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
ROJI_DOMAIN=localhost
ROJI_CERTS_DIR=/certs
ROJI_AUTO_CERT=true
ROJI_DASHBOARD=roji.localhost
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
}

# Wait for certificates to be generated
wait_for_certs() {
    print_info "Waiting for certificates to be generated..."

    local max_wait=30
    local waited=0

    # Check if certificates exist (now directly mounted)
    while [ ! -f "${INSTALL_DIR}/certs/ca.pem" ] && [ $waited -lt $max_wait ]; do
        sleep 1
        waited=$((waited + 1))
    done

    if [ -f "${INSTALL_DIR}/certs/ca.pem" ]; then
        print_success "Certificates generated"
        # Create Windows-compatible .crt file
        if [ -f "${INSTALL_DIR}/certs/ca.pem" ]; then
            cp "${INSTALL_DIR}/certs/ca.pem" "${INSTALL_DIR}/certs/ca.crt"
        fi
    else
        print_warning "Certificate generation is taking longer than expected"
        print_warning "You can check the logs with: docker logs roji"
    fi
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
    echo "  2. Access your app at https://myapp.localhost"
    echo ""

    echo -e "${CYAN}Useful commands:${NC}"
    echo ""
    echo "  View logs:       docker logs roji -f"
    echo "  Restart:         docker restart roji"
    echo "  Stop:            docker stop roji"
    echo "  Start:           docker start roji"
    echo "  Uninstall:       docker compose down && rm -rf ${INSTALL_DIR}"
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
    create_install_dir
    create_network
    create_compose_file
    create_env_file
    start_roji
    wait_for_certs
    show_ca_instructions
}

main "$@"
