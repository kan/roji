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
    esac
done

# Detect language from environment variables
detect_lang() {
    local lang_val=""
    for env in LC_ALL LC_MESSAGES LANG; do
        lang_val=$(eval echo \$$env)
        if [ -n "$lang_val" ] && [ "$lang_val" != "C" ] && [ "$lang_val" != "POSIX" ]; then
            break
        fi
        lang_val=""
    done
    case "$lang_val" in
        ja*) echo "ja" ;;
        *) echo "en" ;;
    esac
}

# Set up i18n messages based on detected language
setup_messages() {
    local lang="$1"
    if [ "$lang" = "ja" ]; then
        # Banner
        MSG_BANNER_DESC="ローカル開発用リバースプロキシ"
        MSG_BANNER_MODE="Native Mode インストール"
        # Platform detection
        MSG_UNSUPPORTED_OS="サポートされていないOS:"
        MSG_UNSUPPORTED_ARCH="サポートされていないアーキテクチャ:"
        # Docker checks
        MSG_DOCKER_NOT_INSTALLED="Docker がインストールされていません"
        MSG_DOCKER_REQUIRED="roji はコンテナを検出するために Docker が必要です。"
        MSG_DOCKER_INSTALL_HINT="先に Docker をインストールしてください: https://docs.docker.com/get-docker/"
        MSG_DOCKER_NOT_RUNNING="Docker デーモンが起動していません"
        MSG_DOCKER_START_HINT="Docker を起動してから再度お試しください。"
        MSG_DOCKER_AVAILABLE="Docker が利用可能です"
        # Docker Mode warning
        MSG_DOCKER_MODE_REMOVED="Docker Mode はサポート終了しました (v1.0.0 で削除)"
        MSG_NATIVE_ONLY="roji v1.0.0 は Native Mode (単体バイナリ) のみサポートしています。"
        MSG_DOCKER_DEPRECATED="Docker Mode は v0.9.0 で非推奨となり、削除されました。"
        MSG_MIGRATE_MANUAL="手動で移行するには:"
        MSG_MIGRATE_STEP1="Docker コンテナを停止:"
        MSG_MIGRATE_STEP2="証明書をバックアップ (任意):"
        MSG_MIGRATE_STEP3="Docker インストールを削除:"
        MSG_MIGRATE_STEP4="インストーラーを再実行:"
        # Installation directory
        MSG_INSTALL_LOCATION="インストール先:"
        MSG_INSTALL_LOCAL="~/.local/bin (推奨、sudo 不要)"
        MSG_INSTALL_GLOBAL="/usr/local/bin (システム全体、sudo 必要)"
        MSG_CHOOSE_OPTION="オプションを選択 [1]: "
        MSG_INSTALLING_TO="インストール先:"
        # Download and install
        MSG_DOWNLOADING="roji %s (%s) をダウンロード中..."
        MSG_DOWNLOAD_FAILED="roji のダウンロードに失敗しました"
        MSG_INSTALLED_TO="roji を %s にインストールしました"
        MSG_NOT_IN_PATH="%s が PATH に含まれていません"
        MSG_ADD_TO_PATH="シェル設定に追加してください:"
        # Doctor
        MSG_RUNNING_DIAGNOSTICS="診断とセットアップを実行中..."
        MSG_DOCTOR_PARTIAL="一部の問題を自動修復できませんでした"
        MSG_DOCTOR_DETAILS="詳細は 'sudo roji doctor' を実行してください"
        MSG_ENV_CONFIGURED="環境を設定しました"
        # CA certificate
        MSG_INSTALLING_CA="CA 証明書をシステム信頼ストアにインストール中..."
        MSG_WSL_DETECTED="WSL を検出 - Linux と Windows の両方にインストールします"
        MSG_CA_MANUAL="CA 証明書のインストールに手動操作が必要な場合があります"
        MSG_CA_RETRY="再試行するには 'sudo roji ca install' を実行"
        MSG_CA_STATUS="現在の状態は 'roji ca status' で確認"
        MSG_CA_INSTALLED="CA 証明書をインストールしました"
        # Service
        MSG_SKIP_SERVICE="サービスのインストールをスキップ (--no-service)"
        MSG_INSTALLING_SERVICE="roji サービスをインストール・起動中..."
        MSG_SERVICE_INSTALL_FAILED="サービスのインストールに失敗しました"
        MSG_SERVICE_MANUAL_START="手動で起動するには: sudo roji"
        MSG_SERVICE_START_FAILED="サービスの起動に失敗しました"
        MSG_SERVICE_CHECK_STATUS="状態確認: sudo roji service status"
        MSG_SERVICE_RUNNING="roji サービスが稼働中です"
        # Completion
        MSG_INSTALL_SUCCESS="roji のインストールが完了しました!"
        MSG_LABEL_VERSION="バージョン:"
        MSG_LABEL_BINARY="バイナリ:"
        MSG_LABEL_CONFIG="設定:"
        MSG_LABEL_DASHBOARD="ダッシュボード:"
        MSG_QUICK_START="クイックスタート:"
        MSG_QS_STEP1="Docker Compose サービスを 'roji' ネットワークに追加:"
        MSG_QS_STEP2="アプリに https://myapp.dev.localhost でアクセス"
        MSG_USEFUL_COMMANDS="便利なコマンド:"
        MSG_CMD_START="サーバー起動 (フォアグラウンド)"
        MSG_CMD_STATUS="サービス状態確認"
        MSG_CMD_RESTART="サービス再起動"
        MSG_CMD_DOCTOR="診断実行"
        MSG_CMD_CONFIG="現在の設定を表示"
        MSG_CMD_ROUTES="アクティブなルート一覧"
        MSG_DOCUMENTATION="ドキュメント:"
        # Upgrade
        MSG_EXISTING_DETECTED="既存の roji インストールを検出"
        MSG_CURRENT_VERSION="現在のバージョン:"
        MSG_LATEST_VERSION="最新バージョン:"
        MSG_LOCATION="場所:"
        MSG_UP_TO_DATE="roji は最新です"
        MSG_SERVICE_NOT_RUNNING="roji サービスが稼働していません"
        MSG_SERVICE_START_HINT="起動するには: sudo roji service start"
        MSG_UPGRADE_AVAILABLE="新しいバージョンが利用可能です!"
        MSG_UPGRADING="アップグレード中..."
        MSG_OPTIONS="オプション:"
        MSG_UPGRADE_TO="バージョン %s にアップグレード"
        MSG_KEEP_CURRENT="現在のバージョンを維持 (%s)"
        MSG_KEEPING_CURRENT="現在のバージョンを維持します"
        MSG_AUTO_UPGRADING="バージョン %s に自動アップグレード中..."
        MSG_UPGRADING_IN_PLACE="既存の場所でアップグレード:"
        MSG_STOPPING_SERVICE="roji サービスを停止中..."
    else
        # English (default)
        # Banner
        MSG_BANNER_DESC="Reverse proxy for local development"
        MSG_BANNER_MODE="Native Mode Installation"
        # Platform detection
        MSG_UNSUPPORTED_OS="Unsupported operating system:"
        MSG_UNSUPPORTED_ARCH="Unsupported architecture:"
        # Docker checks
        MSG_DOCKER_NOT_INSTALLED="Docker is not installed"
        MSG_DOCKER_REQUIRED="roji requires Docker to discover containers."
        MSG_DOCKER_INSTALL_HINT="Please install Docker first: https://docs.docker.com/get-docker/"
        MSG_DOCKER_NOT_RUNNING="Docker daemon is not running"
        MSG_DOCKER_START_HINT="Please start Docker and try again."
        MSG_DOCKER_AVAILABLE="Docker is available"
        # Docker Mode warning
        MSG_DOCKER_MODE_REMOVED="Docker Mode is no longer supported (removed in v1.0.0)"
        MSG_NATIVE_ONLY="roji v1.0.0 only supports Native Mode (standalone binary)."
        MSG_DOCKER_DEPRECATED="Docker Mode was deprecated in v0.9.0 and has been removed."
        MSG_MIGRATE_MANUAL="To migrate manually:"
        MSG_MIGRATE_STEP1="Stop the Docker container:"
        MSG_MIGRATE_STEP2="Back up your certificates (optional):"
        MSG_MIGRATE_STEP3="Remove the Docker installation:"
        MSG_MIGRATE_STEP4="Re-run this installer:"
        # Installation directory
        MSG_INSTALL_LOCATION="Installation location:"
        MSG_INSTALL_LOCAL="~/.local/bin (recommended, no sudo for install)"
        MSG_INSTALL_GLOBAL="/usr/local/bin (system-wide, requires sudo)"
        MSG_CHOOSE_OPTION="Choose an option [1]: "
        MSG_INSTALLING_TO="Installing to:"
        # Download and install
        MSG_DOWNLOADING="Downloading roji %s for %s..."
        MSG_DOWNLOAD_FAILED="Failed to download roji"
        MSG_INSTALLED_TO="roji installed to %s"
        MSG_NOT_IN_PATH="%s is not in your PATH"
        MSG_ADD_TO_PATH="Add it to your shell configuration:"
        # Doctor
        MSG_RUNNING_DIAGNOSTICS="Running diagnostics and setup..."
        MSG_DOCTOR_PARTIAL="Some issues could not be auto-fixed"
        MSG_DOCTOR_DETAILS="Run 'sudo roji doctor' to see details"
        MSG_ENV_CONFIGURED="Environment configured"
        # CA certificate
        MSG_INSTALLING_CA="Installing CA certificate to system trust store..."
        MSG_WSL_DETECTED="WSL detected - installing to both Linux and Windows"
        MSG_CA_MANUAL="CA certificate installation may require manual steps"
        MSG_CA_RETRY="Run 'sudo roji ca install' to retry"
        MSG_CA_STATUS="Or see 'roji ca status' for current state"
        MSG_CA_INSTALLED="CA certificate installed"
        # Service
        MSG_SKIP_SERVICE="Skipping service installation (--no-service)"
        MSG_INSTALLING_SERVICE="Installing and starting roji service..."
        MSG_SERVICE_INSTALL_FAILED="Service installation failed"
        MSG_SERVICE_MANUAL_START="You can start roji manually with: sudo roji"
        MSG_SERVICE_START_FAILED="Service failed to start"
        MSG_SERVICE_CHECK_STATUS="Check status with: sudo roji service status"
        MSG_SERVICE_RUNNING="roji service is running"
        # Completion
        MSG_INSTALL_SUCCESS="roji has been successfully installed!"
        MSG_LABEL_VERSION="Version:"
        MSG_LABEL_BINARY="Binary:"
        MSG_LABEL_CONFIG="Config:"
        MSG_LABEL_DASHBOARD="Dashboard:"
        MSG_QUICK_START="Quick Start:"
        MSG_QS_STEP1="Add your Docker Compose services to the 'roji' network:"
        MSG_QS_STEP2="Access your app at https://myapp.dev.localhost"
        MSG_USEFUL_COMMANDS="Useful commands:"
        MSG_CMD_START="Start server (foreground)"
        MSG_CMD_STATUS="Check service status"
        MSG_CMD_RESTART="Restart service"
        MSG_CMD_DOCTOR="Run diagnostics"
        MSG_CMD_CONFIG="Show current configuration"
        MSG_CMD_ROUTES="List active routes"
        MSG_DOCUMENTATION="Documentation:"
        # Upgrade
        MSG_EXISTING_DETECTED="Existing roji installation detected"
        MSG_CURRENT_VERSION="Current version:"
        MSG_LATEST_VERSION="Latest version:"
        MSG_LOCATION="Location:"
        MSG_UP_TO_DATE="roji is already up to date"
        MSG_SERVICE_NOT_RUNNING="roji service is not running"
        MSG_SERVICE_START_HINT="Start with: sudo roji service start"
        MSG_UPGRADE_AVAILABLE="A newer version is available!"
        MSG_UPGRADING="Upgrading..."
        MSG_OPTIONS="Options:"
        MSG_UPGRADE_TO="Upgrade to %s"
        MSG_KEEP_CURRENT="Keep current version (%s)"
        MSG_KEEPING_CURRENT="Keeping current version"
        MSG_AUTO_UPGRADING="Auto-upgrading to %s..."
        MSG_UPGRADING_IN_PLACE="Upgrading in place:"
        MSG_STOPPING_SERVICE="Stopping roji service..."
    fi
}

# Detect language and set up messages
DETECTED_LANG=$(detect_lang)
setup_messages "$DETECTED_LANG"

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
    echo -e "${CYAN}║${NC}  🛤️  ${BLUE}roji${NC} - ${MSG_BANNER_DESC}                    ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}      ${MSG_BANNER_MODE}                               ${CYAN}║${NC}"
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
            print_error "${MSG_UNSUPPORTED_OS} $(uname -s)"
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
            print_error "${MSG_UNSUPPORTED_ARCH} $(uname -m)"
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
        print_error "$MSG_DOCKER_NOT_INSTALLED"
        echo ""
        echo "$MSG_DOCKER_REQUIRED"
        echo "$MSG_DOCKER_INSTALL_HINT"
        echo ""
        exit 1
    fi

    if ! docker info &> /dev/null; then
        print_error "$MSG_DOCKER_NOT_RUNNING"
        echo ""
        echo "$MSG_DOCKER_START_HINT"
        echo ""
        exit 1
    fi

    print_success "$MSG_DOCKER_AVAILABLE"
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

# Warn about Docker Mode (no longer supported in v1.0.0)
warn_docker_mode() {
    echo ""
    echo -e "${RED}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║${NC}  ${MSG_DOCKER_MODE_REMOVED}      ${RED}║${NC}"
    echo -e "${RED}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "$MSG_NATIVE_ONLY"
    echo "$MSG_DOCKER_DEPRECATED"
    echo ""
    echo -e "${CYAN}${MSG_MIGRATE_MANUAL}${NC}"
    echo ""
    echo "  1. ${MSG_MIGRATE_STEP1}"
    echo "     cd ${DOCKER_INSTALL_DIR} && docker compose down"
    echo ""
    echo "  2. ${MSG_MIGRATE_STEP2}"
    echo "     cp -r ${DOCKER_INSTALL_DIR}/certs ~/.local/share/roji/"
    echo ""
    echo "  3. ${MSG_MIGRATE_STEP3}"
    echo "     rm -rf ${DOCKER_INSTALL_DIR}"
    echo ""
    echo "  4. ${MSG_MIGRATE_STEP4}"
    echo "     curl -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash"
    echo ""
    exit 1
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
        echo -e "${CYAN}${MSG_INSTALL_LOCATION}${NC}"
        echo ""
        echo "  1. ${MSG_INSTALL_LOCAL}"
        echo "  2. ${MSG_INSTALL_GLOBAL}"
        echo ""
        read -p "${MSG_CHOOSE_OPTION}" choice
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
    print_info "${MSG_INSTALLING_TO} ${INSTALL_DIR}"
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

    # shellcheck disable=SC2059
    print_info "$(printf "$MSG_DOWNLOADING" "$version" "$platform")"

    # Construct download URL
    if [ "$version" = "latest" ]; then
        download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/roji_${platform}.${archive_ext}"
    else
        download_url="https://github.com/${GITHUB_REPO}/releases/download/v${version}/roji_${platform}.${archive_ext}"
    fi

    # Download
    if ! curl -fsSL "$download_url" -o "${tmp_dir}/roji.${archive_ext}"; then
        print_error "$MSG_DOWNLOAD_FAILED"
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

    # shellcheck disable=SC2059
    print_success "$(printf "$MSG_INSTALLED_TO" "${INSTALL_DIR}/roji")"

    # Check if INSTALL_DIR is in PATH
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        echo ""
        # shellcheck disable=SC2059
        print_warning "$(printf "$MSG_NOT_IN_PATH" "${INSTALL_DIR}")"
        echo ""
        echo "  ${MSG_ADD_TO_PATH}"
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
    print_info "$MSG_RUNNING_DIAGNOSTICS"
    echo ""

    if ! sudo "${INSTALL_DIR}/roji" doctor --fix; then
        print_warning "$MSG_DOCTOR_PARTIAL"
        echo ""
        echo "  ${MSG_DOCTOR_DETAILS}"
        echo ""
    else
        print_success "$MSG_ENV_CONFIGURED"
    fi
}

# Install CA certificate
install_ca() {
    print_info "$MSG_INSTALLING_CA"

    local ca_args=""
    if is_wsl; then
        ca_args="--windows"
        print_info "$MSG_WSL_DETECTED"
    fi

    if ! sudo "${INSTALL_DIR}/roji" ca install $ca_args; then
        print_warning "$MSG_CA_MANUAL"
        echo ""
        echo "  ${MSG_CA_RETRY}"
        echo "  ${MSG_CA_STATUS}"
        echo ""
    else
        print_success "$MSG_CA_INSTALLED"
    fi
}

# Install and start service
install_service() {
    if [ "$SKIP_SERVICE" = true ]; then
        print_info "$MSG_SKIP_SERVICE"
        return
    fi

    print_info "$MSG_INSTALLING_SERVICE"

    if ! sudo "${INSTALL_DIR}/roji" service install; then
        print_warning "$MSG_SERVICE_INSTALL_FAILED"
        echo ""
        echo "  ${MSG_SERVICE_MANUAL_START}"
        echo ""
        return
    fi

    if ! sudo "${INSTALL_DIR}/roji" service start; then
        print_warning "$MSG_SERVICE_START_FAILED"
        echo ""
        echo "  ${MSG_SERVICE_CHECK_STATUS}"
        echo ""
        return
    fi

    print_success "$MSG_SERVICE_RUNNING"
}

# Show completion message
show_completion() {
    local version=$(get_latest_version)

    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  🎉 ${BLUE}roji${NC} ${MSG_INSTALL_SUCCESS}                  ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${CYAN}${MSG_LABEL_VERSION}${NC}    ${version}"
    echo -e "${CYAN}${MSG_LABEL_BINARY}${NC}     ${INSTALL_DIR}/roji"
    echo -e "${CYAN}${MSG_LABEL_CONFIG}${NC}     ~/.config/roji/config.yaml"
    echo -e "${CYAN}${MSG_LABEL_DASHBOARD}${NC}  https://roji.dev.localhost"
    echo ""

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}${MSG_QUICK_START}${NC}"
    echo ""
    echo "  1. ${MSG_QS_STEP1}"
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
    echo "  2. ${MSG_QS_STEP2}"
    echo ""

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    echo -e "${CYAN}${MSG_USEFUL_COMMANDS}${NC}"
    echo ""
    echo "  sudo roji                  ${MSG_CMD_START}"
    echo "  sudo roji service status   ${MSG_CMD_STATUS}"
    echo "  sudo roji service restart  ${MSG_CMD_RESTART}"
    echo "  sudo roji doctor           ${MSG_CMD_DOCTOR}"
    echo "  roji config show           ${MSG_CMD_CONFIG}"
    echo "  roji routes                ${MSG_CMD_ROUTES}"
    echo ""

    echo -e "${CYAN}${MSG_DOCUMENTATION}${NC}"
    echo "  https://roji-proxy.dev"
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
    echo -e "${CYAN}${MSG_EXISTING_DETECTED}${NC}"
    echo ""
    echo -e "  ${MSG_CURRENT_VERSION} ${YELLOW}${current}${NC}"
    echo -e "  ${MSG_LATEST_VERSION}  ${GREEN}${latest}${NC}"
    if [ -n "$existing_dir" ]; then
        echo -e "  ${MSG_LOCATION}        ${existing_dir}/roji"
    fi
    echo ""

    # Check if already up to date
    if ! version_lt "$current" "$latest"; then
        print_success "${MSG_UP_TO_DATE} (${current})"
        echo ""

        # Check service status
        if sudo roji service status &>/dev/null; then
            print_success "$MSG_SERVICE_RUNNING"
        else
            print_warning "$MSG_SERVICE_NOT_RUNNING"
            echo ""
            echo "  ${MSG_SERVICE_START_HINT}"
        fi
        echo ""
        exit 0
    fi

    # Upgrade available
    echo "$MSG_UPGRADE_AVAILABLE"
    echo ""

    if [ "$FORCE_UPGRADE" = true ]; then
        print_info "$MSG_UPGRADING"
    elif [ -t 0 ]; then
        echo -e "${CYAN}${MSG_OPTIONS}${NC}"
        # shellcheck disable=SC2059
        echo "  1. $(printf "$MSG_UPGRADE_TO" "$latest")"
        # shellcheck disable=SC2059
        echo "  2. $(printf "$MSG_KEEP_CURRENT" "$current")"
        echo ""
        read -p "${MSG_CHOOSE_OPTION}" choice
        choice=${choice:-1}

        if [ "$choice" != "1" ]; then
            echo ""
            print_success "$MSG_KEEPING_CURRENT"
            echo ""
            exit 0
        fi
    else
        # Non-interactive mode: auto-upgrade
        # shellcheck disable=SC2059
        print_info "$(printf "$MSG_AUTO_UPGRADING" "$latest")"
    fi

    # Use existing install directory
    if [ -n "$existing_dir" ]; then
        INSTALL_DIR="$existing_dir"
        if [ "$existing_dir" = "$GLOBAL_BIN" ]; then
            INSTALL_MODE="global"
        else
            INSTALL_MODE="local"
        fi
        print_info "${MSG_UPGRADING_IN_PLACE} ${INSTALL_DIR}"
    fi

    # Stop service before upgrade
    print_info "$MSG_STOPPING_SERVICE"
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
        warn_docker_mode
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
