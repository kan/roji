#!/bin/bash
set -e

# roji installation script (Native Mode)
# Usage: curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash
#
# Options:
#   --upgrade       Force upgrade mode
#   --local         Install to ~/.local/bin (default)
#   --global        Install to /usr/local/bin
#   --no-service    Skip service installation
#   --migrate       Migrate from Docker Mode

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="kan/roji"
LOCAL_BIN="$HOME/.local/bin"
GLOBAL_BIN="/usr/local/bin"
DOCKER_INSTALL_DIR="$HOME/.roji"

# Default options
INSTALL_MODE=""  # Will be set interactively or via flags
FORCE_UPGRADE=false
SKIP_SERVICE=false
MIGRATE_DOCKER=false

# Parse command line arguments
for arg in "$@"; do
    case $arg in
        --upgrade)
            FORCE_UPGRADE=true
            ;;
        --local)
            INSTALL_MODE="local"
            ;;
        --global)
            INSTALL_MODE="global"
            ;;
        --no-service)
            SKIP_SERVICE=true
            ;;
        --migrate)
            MIGRATE_DOCKER=true
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
    echo -e "${CYAN}║${NC}      Native Mode Installation                               ${CYAN}║${NC}"
    echo -e "${CYAN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Detect OS and architecture
# Returns format matching GoReleaser output: Linux_x86_64, Darwin_arm64, etc.
detect_platform() {
    local os=""
    local arch=""

    case "$(uname -s)" in
        Darwin*)
            os="Darwin"
            ;;
        Linux*)
            os="Linux"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            os="Windows"
            ;;
        *)
            print_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="x86_64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

# Check if running in WSL
is_wsl() {
    if grep -qEi "(Microsoft|WSL)" /proc/version 2>/dev/null; then
        return 0
    fi
    return 1
}

# Check if Docker is installed
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        echo ""
        echo "roji requires Docker to discover containers."
        echo "Please install Docker first: https://docs.docker.com/get-docker/"
        echo ""
        exit 1
    fi

    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running"
        echo ""
        echo "Please start Docker and try again."
        echo ""
        exit 1
    fi

    print_success "Docker is available"
}

# Check for existing Docker Mode installation
check_docker_mode() {
    if [ -f "${DOCKER_INSTALL_DIR}/docker-compose.yml" ]; then
        return 0
    fi
    if docker ps --filter "name=roji" --format "{{.ID}}" 2>/dev/null | grep -q .; then
        return 0
    fi
    return 1
}

# Check for existing Native Mode installation
check_native_mode() {
    if command -v roji &> /dev/null; then
        return 0
    fi
    if [ -f "${LOCAL_BIN}/roji" ] || [ -f "${GLOBAL_BIN}/roji" ]; then
        return 0
    fi
    return 1
}

# Get current native version
get_native_version() {
    if command -v roji &> /dev/null; then
        roji version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown"
    else
        echo "not installed"
    fi
}

# Get latest version from GitHub
get_latest_version() {
    local version=""
    if command -v curl &> /dev/null; then
        version=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep -oE '"tag_name":\s*"v?[0-9]+\.[0-9]+\.[0-9]+"' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
    fi
    if [ -z "$version" ]; then
        version="latest"
    fi
    echo "$version"
}

# Migrate from Docker Mode
migrate_from_docker() {
    echo ""
    echo -e "${YELLOW}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║${NC}  Docker Mode installation detected                           ${YELLOW}║${NC}"
    echo -e "${YELLOW}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "roji now runs as a native binary instead of a Docker container."
    echo "This provides better performance and easier management."
    echo ""

    if [ -t 0 ] && [ "$MIGRATE_DOCKER" != true ]; then
        echo -e "${CYAN}Options:${NC}"
        echo "  1. Migrate to Native Mode (recommended)"
        echo "  2. Keep Docker Mode (exit)"
        echo ""
        read -p "Choose an option [1]: " choice
        choice=${choice:-1}

        if [ "$choice" != "1" ]; then
            echo ""
            print_info "Keeping Docker Mode. No changes made."
            echo ""
            echo "To manage your Docker Mode installation:"
            echo "  cd ${DOCKER_INSTALL_DIR}"
            echo "  docker compose up -d"
            echo ""
            exit 0
        fi
    fi

    print_info "Migrating from Docker Mode to Native Mode..."

    # Stop Docker container
    print_info "Stopping Docker container..."
    if [ -f "${DOCKER_INSTALL_DIR}/docker-compose.yml" ]; then
        cd "${DOCKER_INSTALL_DIR}"
        docker compose down 2>/dev/null || true
    else
        docker stop roji 2>/dev/null || true
        docker rm roji 2>/dev/null || true
    fi
    print_success "Docker container stopped"

    # Backup certs if they exist
    if [ -d "${DOCKER_INSTALL_DIR}/certs" ]; then
        print_info "Backing up certificates..."
        mkdir -p "$HOME/.local/share/roji"
        cp -r "${DOCKER_INSTALL_DIR}/certs" "$HOME/.local/share/roji/" 2>/dev/null || true
        print_success "Certificates backed up to ~/.local/share/roji/certs"
    fi

    # Rename old directory as backup
    if [ -d "${DOCKER_INSTALL_DIR}" ]; then
        local backup_dir="${DOCKER_INSTALL_DIR}.backup.$(date +%Y%m%d_%H%M%S)"
        print_info "Moving old installation to ${backup_dir}..."
        mv "${DOCKER_INSTALL_DIR}" "${backup_dir}"
        print_success "Old installation backed up"
        echo ""
        echo "  You can delete it later with: rm -rf ${backup_dir}"
    fi

    echo ""
    print_success "Migration preparation complete"
    echo ""
}

# Select installation directory
select_install_dir() {
    # Already set (upgrade or flag)
    if [ -n "$INSTALL_DIR" ]; then
        return
    fi

    if [ -n "$INSTALL_MODE" ]; then
        # Set via flag
        if [ "$INSTALL_MODE" = "global" ]; then
            INSTALL_DIR="$GLOBAL_BIN"
        else
            INSTALL_DIR="$LOCAL_BIN"
        fi
        return
    fi

    # Interactive selection
    if [ -t 0 ]; then
        echo ""
        echo -e "${CYAN}Installation location:${NC}"
        echo ""
        echo "  1. ~/.local/bin (recommended, no sudo for install)"
        echo "  2. /usr/local/bin (system-wide, requires sudo)"
        echo ""
        read -p "Choose an option [1]: " choice
        choice=${choice:-1}

        case $choice in
            2)
                INSTALL_DIR="$GLOBAL_BIN"
                INSTALL_MODE="global"
                ;;
            *)
                INSTALL_DIR="$LOCAL_BIN"
                INSTALL_MODE="local"
                ;;
        esac
    else
        # Non-interactive: default to local
        INSTALL_DIR="$LOCAL_BIN"
        INSTALL_MODE="local"
    fi

    echo ""
    print_info "Installing to: ${INSTALL_DIR}"
}

# Download and install binary
install_binary() {
    local platform=$(detect_platform)
    local version=$(get_latest_version)
    local download_url=""
    local archive_ext="tar.gz"
    local tmp_dir=$(mktemp -d)

    # Windows uses zip format
    if [[ "$platform" == Windows* ]]; then
        archive_ext="zip"
    fi

    print_info "Downloading roji ${version} for ${platform}..."

    # Construct download URL
    if [ "$version" = "latest" ]; then
        download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/roji_${platform}.${archive_ext}"
    else
        download_url="https://github.com/${GITHUB_REPO}/releases/download/v${version}/roji_${platform}.${archive_ext}"
    fi

    # Download
    if ! curl -fsSL "$download_url" -o "${tmp_dir}/roji.${archive_ext}"; then
        print_error "Failed to download roji"
        echo ""
        echo "URL: ${download_url}"
        echo ""
        rm -rf "$tmp_dir"
        exit 1
    fi

    # Extract
    if [ "$archive_ext" = "zip" ]; then
        unzip -q "${tmp_dir}/roji.${archive_ext}" -d "$tmp_dir"
    else
        tar -xzf "${tmp_dir}/roji.${archive_ext}" -C "$tmp_dir"
    fi

    # Create install directory if needed
    if [ ! -d "$INSTALL_DIR" ]; then
        if [ "$INSTALL_MODE" = "global" ]; then
            sudo mkdir -p "$INSTALL_DIR"
        else
            mkdir -p "$INSTALL_DIR"
        fi
    fi

    # Install binary
    if [ "$INSTALL_MODE" = "global" ]; then
        sudo mv "${tmp_dir}/roji" "${INSTALL_DIR}/roji"
        sudo chmod +x "${INSTALL_DIR}/roji"
    else
        mv "${tmp_dir}/roji" "${INSTALL_DIR}/roji"
        chmod +x "${INSTALL_DIR}/roji"
    fi

    rm -rf "$tmp_dir"

    print_success "roji installed to ${INSTALL_DIR}/roji"

    # Check if INSTALL_DIR is in PATH
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        echo ""
        print_warning "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "  Add it to your shell configuration:"
        echo ""
        if [ -f "$HOME/.zshrc" ]; then
            echo "    echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc"
            echo "    source ~/.zshrc"
        elif [ -f "$HOME/.bashrc" ]; then
            echo "    echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.bashrc"
            echo "    source ~/.bashrc"
        else
            echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        fi
        echo ""

        # Add to PATH for this session
        export PATH="${INSTALL_DIR}:$PATH"
    fi
}

# Run doctor to set up environment
run_doctor() {
    print_info "Running diagnostics and setup..."
    echo ""

    if ! sudo "${INSTALL_DIR}/roji" doctor --fix; then
        print_warning "Some issues could not be auto-fixed"
        echo ""
        echo "  Run 'sudo roji doctor' to see details"
        echo ""
    else
        print_success "Environment configured"
    fi
}

# Install CA certificate
install_ca() {
    print_info "Installing CA certificate to system trust store..."

    local ca_args=""
    if is_wsl; then
        ca_args="--windows"
        print_info "WSL detected - installing to both Linux and Windows"
    fi

    if ! sudo "${INSTALL_DIR}/roji" ca install $ca_args; then
        print_warning "CA certificate installation may require manual steps"
        echo ""
        echo "  Run 'sudo roji ca install' to retry"
        echo "  Or see 'roji ca status' for current state"
        echo ""
    else
        print_success "CA certificate installed"
    fi
}

# Install and start service
install_service() {
    if [ "$SKIP_SERVICE" = true ]; then
        print_info "Skipping service installation (--no-service)"
        return
    fi

    print_info "Installing and starting roji service..."

    if ! sudo "${INSTALL_DIR}/roji" service install; then
        print_warning "Service installation failed"
        echo ""
        echo "  You can start roji manually with: sudo roji"
        echo ""
        return
    fi

    if ! sudo "${INSTALL_DIR}/roji" service start; then
        print_warning "Service failed to start"
        echo ""
        echo "  Check status with: sudo roji service status"
        echo ""
        return
    fi

    print_success "roji service is running"
}

# Show completion message
show_completion() {
    local version=$(get_latest_version)

    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  🎉 ${BLUE}roji${NC} has been successfully installed!                  ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${CYAN}Version:${NC}    ${version}"
    echo -e "${CYAN}Binary:${NC}     ${INSTALL_DIR}/roji"
    echo -e "${CYAN}Config:${NC}     ~/.config/roji/config.yaml"
    echo -e "${CYAN}Dashboard:${NC}  https://roji.dev.localhost"
    echo ""

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}Quick Start:${NC}"
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

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}Useful commands:${NC}"
    echo ""
    echo "  sudo roji                  Start server (foreground)"
    echo "  sudo roji service status   Check service status"
    echo "  sudo roji service restart  Restart service"
    echo "  sudo roji doctor           Run diagnostics"
    echo "  roji config show           Show current configuration"
    echo "  roji routes                List active routes"
    echo ""

    echo -e "${CYAN}Documentation:${NC}"
    echo "  https://github.com/kan/roji"
    echo ""
}

# Compare versions (returns 0 if v1 < v2)
version_lt() {
    local v1="$1"
    local v2="$2"

    # Handle special cases
    if [ "$v1" = "latest" ] || [ "$v2" = "latest" ] || [ "$v1" = "unknown" ] || [ "$v1" = "not installed" ]; then
        return 1  # Can't compare
    fi
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

# Detect existing roji binary location
detect_existing_install_dir() {
    # Check command location first
    if command -v roji &> /dev/null; then
        local roji_path=$(command -v roji)
        dirname "$roji_path"
        return
    fi
    # Check common locations
    if [ -f "${LOCAL_BIN}/roji" ]; then
        echo "$LOCAL_BIN"
        return
    fi
    if [ -f "${GLOBAL_BIN}/roji" ]; then
        echo "$GLOBAL_BIN"
        return
    fi
    echo ""
}

# Handle existing native installation (upgrade)
handle_existing_native() {
    local current=$(get_native_version)
    local latest=$(get_latest_version)
    local existing_dir=$(detect_existing_install_dir)

    echo ""
    echo -e "${CYAN}Existing roji installation detected${NC}"
    echo ""
    echo -e "  Current version: ${YELLOW}${current}${NC}"
    echo -e "  Latest version:  ${GREEN}${latest}${NC}"
    if [ -n "$existing_dir" ]; then
        echo -e "  Location:        ${existing_dir}/roji"
    fi
    echo ""

    # Check if already up to date
    if ! version_lt "$current" "$latest"; then
        print_success "roji is already up to date (${current})"
        echo ""

        # Check service status
        if sudo roji service status &>/dev/null; then
            print_success "roji service is running"
        else
            print_warning "roji service is not running"
            echo ""
            echo "  Start with: sudo roji service start"
        fi
        echo ""
        exit 0
    fi

    # Upgrade available
    echo "A newer version is available!"
    echo ""

    if [ "$FORCE_UPGRADE" = true ]; then
        print_info "Upgrading..."
    elif [ -t 0 ]; then
        echo -e "${CYAN}Options:${NC}"
        echo "  1. Upgrade to ${latest}"
        echo "  2. Keep current version (${current})"
        echo ""
        read -p "Choose an option [1]: " choice
        choice=${choice:-1}

        if [ "$choice" != "1" ]; then
            echo ""
            print_success "Keeping current version"
            echo ""
            exit 0
        fi
    else
        # Non-interactive mode: auto-upgrade
        print_info "Auto-upgrading to ${latest}..."
    fi

    # Use existing install directory
    if [ -n "$existing_dir" ]; then
        INSTALL_DIR="$existing_dir"
        if [ "$existing_dir" = "$GLOBAL_BIN" ]; then
            INSTALL_MODE="global"
        else
            INSTALL_MODE="local"
        fi
        print_info "Upgrading in place: ${INSTALL_DIR}"
    fi

    # Stop service before upgrade
    print_info "Stopping roji service..."
    sudo roji service stop 2>/dev/null || true

    return 0  # Proceed with installation
}

# Main installation flow
main() {
    print_banner

    # Check Docker first
    check_docker

    # Check for existing installations
    if check_docker_mode; then
        migrate_from_docker
    elif check_native_mode; then
        handle_existing_native
    fi

    # Select installation directory
    select_install_dir

    # Download and install binary
    install_binary

    # Run doctor to set up environment
    run_doctor

    # Install CA certificate
    install_ca

    # Install and start service
    install_service

    # Show completion message
    show_completion
}

main "$@"
