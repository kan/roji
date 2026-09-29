#!/bin/bash
set -e

# roji installation script (Native Mode)
# Usage: curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash
#
# Options:
#   --upgrade          Force upgrade mode
#   --local            Install to ~/.local/bin (default)
#   --global           Install to /usr/local/bin
#   --no-service       Skip service installation
#   --version X.Y.Z    Install this version instead of the latest release
#
# Environment:
#   ROJI_VERSION       Same as --version (the flag wins when both are given)
#
# The downloaded archive is checked against the release's checksums.txt. When
# the GitHub CLI is installed and logged in, its build provenance is verified
# with `gh attestation verify` as well. Either failure aborts the install.
#
# Nothing below runs until the last line calls main, so a download cut off
# midway leaves a script that only defines variables and functions.

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
# Releases before this one carry no build provenance attestation, so they can
# only be checked against checksums.txt.
FIRST_ATTESTED_VERSION="1.2.1"

# Default options
INSTALL_MODE=""  # Will be set interactively or via flags
FORCE_UPGRADE=false
SKIP_SERVICE=false
REQUESTED_VERSION="${ROJI_VERSION:-}"  # Empty means the latest release
VERSION=""       # Resolved version to install, without the leading "v"
TMP_DIR=""

# Parse command line arguments
parse_args() {
    while [ $# -gt 0 ]; do
        case $1 in
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
            --version)
                if [ $# -lt 2 ]; then
                    print_error "$MSG_VERSION_NEEDS_VALUE"
                    exit 1
                fi
                REQUESTED_VERSION="$2"
                shift
                ;;
            --version=*)
                REQUESTED_VERSION="${1#--version=}"
                ;;
        esac
        shift
    done
}

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
        # Version resolution
        MSG_VERSION_NEEDS_VALUE="--version にはバージョンを指定してください (例: --version 1.2.0)"
        MSG_INVALID_VERSION="不正なバージョン指定: %s"
        MSG_RESOLVE_FAILED="roji の最新バージョンを取得できませんでした"
        MSG_RESOLVE_HINT="ネットワークを確認するか、ROJI_VERSION=x.y.z でバージョンを指定してください"
        # Verification
        MSG_CHECKSUM_TOOL_MISSING="ダウンロードの検証には sha256sum か shasum が必要です"
        MSG_CHECKSUM_NOT_LISTED="checksums.txt に %s がありません"
        MSG_CHECKSUM_FAILED="チェックサムが一致しません。ダウンロードが破損している可能性があります"
        MSG_CHECKSUM_OK="チェックサムを確認しました"
        MSG_VERIFYING_ATTESTATION="gh attestation でビルドの出所を検証中..."
        MSG_ATTESTATION_FAILED="ビルドの出所を検証できませんでした。インストールを中止します"
        MSG_ATTESTATION_OK="ビルドの出所を確認しました"
        MSG_ATTESTATION_NO_GH="gh が無い、ログインしていない、または gh attestation 非対応の版 (2.49 未満) のため、ビルドの出所の検証を省略しました (チェックサムのみ)"
        MSG_ATTESTATION_OLD="roji %s にはビルドの出所の証明が無いため、チェックサムのみで検証しました"
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
        MSG_REQUESTED_VERSION="指定バージョン:"
        MSG_LOCATION="場所:"
        MSG_UP_TO_DATE="roji は最新です"
        MSG_SERVICE_NOT_RUNNING="roji サービスが稼働していません"
        MSG_SERVICE_START_HINT="起動するには: sudo roji service start"
        MSG_UPGRADE_AVAILABLE="新しいバージョンが利用可能です!"
        MSG_UPGRADING="アップグレード中..."
        MSG_OPTIONS="オプション:"
        MSG_UPGRADE_TO="バージョン %s にアップグレード"
        MSG_SWITCH_TO="バージョン %s に切り替え"
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
        # Version resolution
        MSG_VERSION_NEEDS_VALUE="--version needs a version (e.g. --version 1.2.0)"
        MSG_INVALID_VERSION="Invalid version: %s"
        MSG_RESOLVE_FAILED="Could not determine the latest roji version"
        MSG_RESOLVE_HINT="Check your network, or pin a version with ROJI_VERSION=x.y.z"
        # Verification
        MSG_CHECKSUM_TOOL_MISSING="sha256sum or shasum is required to verify the download"
        MSG_CHECKSUM_NOT_LISTED="%s is not listed in checksums.txt"
        MSG_CHECKSUM_FAILED="Checksum mismatch; the download may be corrupted"
        MSG_CHECKSUM_OK="Checksum verified"
        MSG_VERIFYING_ATTESTATION="Verifying build provenance with gh attestation..."
        MSG_ATTESTATION_FAILED="Build provenance could not be verified; aborting installation"
        MSG_ATTESTATION_OK="Build provenance verified"
        MSG_ATTESTATION_NO_GH="gh is missing, not logged in, or older than 2.49 (no gh attestation); skipped the build provenance check (checksum only)"
        MSG_ATTESTATION_OLD="roji %s has no build provenance attestation; verified by checksum only"
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
        MSG_REQUESTED_VERSION="Requested version:"
        MSG_LOCATION="Location:"
        MSG_UP_TO_DATE="roji is already up to date"
        MSG_SERVICE_NOT_RUNNING="roji service is not running"
        MSG_SERVICE_START_HINT="Start with: sudo roji service start"
        MSG_UPGRADE_AVAILABLE="A newer version is available!"
        MSG_UPGRADING="Upgrading..."
        MSG_OPTIONS="Options:"
        MSG_UPGRADE_TO="Upgrade to %s"
        MSG_SWITCH_TO="Switch to %s"
        MSG_KEEP_CURRENT="Keep current version (%s)"
        MSG_KEEPING_CURRENT="Keeping current version"
        MSG_AUTO_UPGRADING="Auto-upgrading to %s..."
        MSG_UPGRADING_IN_PLACE="Upgrading in place:"
        MSG_STOPPING_SERVICE="Stopping roji service..."
    fi
}

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

# Get current native version ("unknown" when it cannot be read)
get_native_version() {
    local version=""
    if command -v roji &> /dev/null; then
        version=$(roji version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
    fi
    echo "${version:-unknown}"
}

# Check that a string is a release version such as 1.2.0 or 1.3.0-rc.1
is_version() {
    [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]
}

# curl restricted to HTTPS with TLS 1.2 or later
curl_https() {
    curl --proto '=https' --tlsv1.2 "$@"
}

# Get the latest release version from the redirect of /releases/latest.
# The REST API is avoided on purpose: unauthenticated calls are limited to 60
# an hour, and running out used to be mistaken for "already up to date".
get_latest_version() {
    local location=""
    location=$(curl_https -fsSI -o /dev/null -w '%{redirect_url}' \
        "https://github.com/${GITHUB_REPO}/releases/latest") || return 1
    local version="${location##*/}"
    version="${version#v}"
    is_version "$version" || return 1
    echo "$version"
}

# Set VERSION from --version / ROJI_VERSION, or from the latest release
resolve_version() {
    if [ -n "$REQUESTED_VERSION" ]; then
        VERSION="${REQUESTED_VERSION#v}"
        if ! is_version "$VERSION"; then
            # shellcheck disable=SC2059
            print_error "$(printf "$MSG_INVALID_VERSION" "$REQUESTED_VERSION")"
            exit 1
        fi
        return
    fi

    if ! VERSION=$(get_latest_version); then
        print_error "$MSG_RESOLVE_FAILED"
        echo ""
        echo "  ${MSG_RESOLVE_HINT}"
        echo ""
        exit 1
    fi
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
    echo "     curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/kan/roji/main/install.sh | bash"
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

# Download a release asset into TMP_DIR, exiting on failure
download_asset() {
    local name="$1"
    local url="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${name}"

    if ! curl_https -fsSL "$url" -o "${TMP_DIR}/${name}"; then
        print_error "$MSG_DOWNLOAD_FAILED"
        echo ""
        echo "URL: ${url}"
        echo ""
        exit 1
    fi
}

# Check the archive against checksums.txt from the same release.
# This catches corruption in transit; it cannot catch tampering, since both
# files come from the same place. verify_attestation covers that.
verify_checksum() {
    local archive="$1"
    local expected=""
    local actual=""

    download_asset "checksums.txt"
    expected=$(awk -v name="$archive" '$2 == name { print $1 }' "${TMP_DIR}/checksums.txt")
    if [ -z "$expected" ]; then
        # shellcheck disable=SC2059
        print_error "$(printf "$MSG_CHECKSUM_NOT_LISTED" "$archive")"
        exit 1
    fi

    if command -v sha256sum &> /dev/null; then
        actual=$(sha256sum "${TMP_DIR}/${archive}" | awk '{ print $1 }')
    elif command -v shasum &> /dev/null; then
        actual=$(shasum -a 256 "${TMP_DIR}/${archive}" | awk '{ print $1 }')
    else
        print_error "$MSG_CHECKSUM_TOOL_MISSING"
        exit 1
    fi

    if [ "$actual" != "$expected" ]; then
        print_error "$MSG_CHECKSUM_FAILED"
        echo ""
        echo "  expected: ${expected}"
        echo "  actual:   ${actual}"
        echo ""
        exit 1
    fi
    print_success "$MSG_CHECKSUM_OK"
}

# Verify that the archive was built by this repository's release workflow
verify_attestation() {
    local archive="$1"

    if version_lt "$VERSION" "$FIRST_ATTESTED_VERSION"; then
        # shellcheck disable=SC2059
        print_warning "$(printf "$MSG_ATTESTATION_OLD" "$VERSION")"
        return
    fi
    # gh attestation verify calls the GitHub API, so it needs a login too.
    # The attestation command itself arrived in gh 2.49.
    if ! command -v gh &> /dev/null || ! gh attestation verify --help &> /dev/null ||
        ! gh auth status &> /dev/null; then
        print_warning "$MSG_ATTESTATION_NO_GH"
        return
    fi

    print_info "$MSG_VERIFYING_ATTESTATION"
    if ! gh attestation verify "${TMP_DIR}/${archive}" --repo "$GITHUB_REPO" > /dev/null; then
        print_error "$MSG_ATTESTATION_FAILED"
        exit 1
    fi
    print_success "$MSG_ATTESTATION_OK"
}

# Download and install binary
install_binary() {
    local platform=""
    platform=$(detect_platform)
    local archive_ext="tar.gz"

    # Windows uses zip format
    if [[ "$platform" == Windows* ]]; then
        archive_ext="zip"
    fi
    local archive="roji_${platform}.${archive_ext}"

    # shellcheck disable=SC2059
    print_info "$(printf "$MSG_DOWNLOADING" "$VERSION" "$platform")"

    TMP_DIR=$(mktemp -d)
    download_asset "$archive"
    verify_checksum "$archive"
    verify_attestation "$archive"

    # Extract
    if [ "$archive_ext" = "zip" ]; then
        unzip -q "${TMP_DIR}/${archive}" -d "$TMP_DIR"
    else
        tar -xzf "${TMP_DIR}/${archive}" -C "$TMP_DIR"
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
        sudo mv "${TMP_DIR}/roji" "${INSTALL_DIR}/roji"
        sudo chmod +x "${INSTALL_DIR}/roji"
    else
        mv "${TMP_DIR}/roji" "${INSTALL_DIR}/roji"
        chmod +x "${INSTALL_DIR}/roji"
    fi

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
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}  🎉 ${BLUE}roji${NC} ${MSG_INSTALL_SUCCESS}                  ${GREEN}║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${CYAN}${MSG_LABEL_VERSION}${NC}    ${VERSION}"
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

# Compare versions (returns 0 if v1 < v2); both must pass is_version
version_lt() {
    local v1="$1"
    local v2="$2"

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
    local current=""
    current=$(get_native_version)
    local existing_dir=""
    existing_dir=$(detect_existing_install_dir)
    local target_label="$MSG_LATEST_VERSION"
    if [ -n "$REQUESTED_VERSION" ]; then
        target_label="$MSG_REQUESTED_VERSION"
    fi

    echo ""
    echo -e "${CYAN}${MSG_EXISTING_DETECTED}${NC}"
    echo ""
    echo -e "  ${MSG_CURRENT_VERSION} ${YELLOW}${current}${NC}"
    echo -e "  ${target_label}  ${GREEN}${VERSION}${NC}"
    if [ -n "$existing_dir" ]; then
        echo -e "  ${MSG_LOCATION}        ${existing_dir}/roji"
    fi
    echo ""

    # With a readable current version that is not older than the target,
    # there is nothing to do when it matches, or when no version was requested
    # (it is ahead of the latest release). An unreadable one always proceeds.
    local is_upgrade=false
    local up_to_date=false
    if [ "$current" != "unknown" ]; then
        if version_lt "$current" "$VERSION"; then
            is_upgrade=true
        elif [ "$current" = "$VERSION" ] || [ -z "$REQUESTED_VERSION" ]; then
            up_to_date=true
        fi
    fi
    if [ "$up_to_date" = true ]; then
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

    # Upgrade (or switch to the requested version)
    local install_choice="$MSG_SWITCH_TO"
    if [ "$is_upgrade" = true ]; then
        echo "$MSG_UPGRADE_AVAILABLE"
        echo ""
        install_choice="$MSG_UPGRADE_TO"
    fi

    if [ "$FORCE_UPGRADE" = true ]; then
        print_info "$MSG_UPGRADING"
    elif [ -t 0 ]; then
        echo -e "${CYAN}${MSG_OPTIONS}${NC}"
        # shellcheck disable=SC2059
        echo "  1. $(printf "$install_choice" "$VERSION")"
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
        print_info "$(printf "$MSG_AUTO_UPGRADING" "$VERSION")"
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
    setup_messages "$(detect_lang)"
    parse_args "$@"
    trap 'rm -rf "$TMP_DIR"' EXIT

    print_banner

    # Check Docker first
    check_docker

    # Docker Mode exits here, before any network access
    if check_docker_mode; then
        warn_docker_mode
    fi

    # Decide which version to install before comparing with an existing one
    resolve_version

    if check_native_mode; then
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
