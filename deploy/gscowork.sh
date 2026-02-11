#!/bin/bash
#
# GSCoWork Debian Service Management Script
# Usage: ./gscowork.sh {install|uninstall|start|stop|restart|status|logs}
#

set -e

SERVICE_NAME="gscowork"
INSTALL_DIR="/opt/gscowork"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This operation requires root privileges. Please run with sudo."
        exit 1
    fi
}

install_service() {
    check_root

    print_info "Installing GSCoWork service..."

    # Create installation directory
    mkdir -p "$INSTALL_DIR"

    # Copy binary (look in parent directory or current directory)
    if [[ -f "${SCRIPT_DIR}/../gscowork" ]]; then
        cp "${SCRIPT_DIR}/../gscowork" "$INSTALL_DIR/"
    elif [[ -f "${SCRIPT_DIR}/gscowork" ]]; then
        cp "${SCRIPT_DIR}/gscowork" "$INSTALL_DIR/"
    else
        print_error "gscowork binary not found. Please build it first: go build -o gscowork ."
        exit 1
    fi

    chmod +x "$INSTALL_DIR/gscowork"

    # Copy service file
    cp "${SCRIPT_DIR}/gscowork.service" "$SERVICE_FILE"

    # Set ownership
    chown -R www-data:www-data "$INSTALL_DIR"

    # Reload systemd
    systemctl daemon-reload

    # Enable service
    systemctl enable "$SERVICE_NAME"

    print_info "Service installed successfully!"
    print_info "Start with: sudo systemctl start $SERVICE_NAME"
    print_info "Or run: sudo $0 start"
}

uninstall_service() {
    check_root

    print_info "Uninstalling GSCoWork service..."

    # Stop service if running
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true

    # Disable service
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true

    # Remove service file
    rm -f "$SERVICE_FILE"

    # Reload systemd
    systemctl daemon-reload

    print_warn "Service uninstalled. Data directory $INSTALL_DIR was kept."
    print_info "To completely remove, run: sudo rm -rf $INSTALL_DIR"
}

start_service() {
    check_root
    print_info "Starting GSCoWork service..."
    systemctl start "$SERVICE_NAME"
    print_info "Service started."
}

stop_service() {
    check_root
    print_info "Stopping GSCoWork service..."
    systemctl stop "$SERVICE_NAME"
    print_info "Service stopped."
}

restart_service() {
    check_root
    print_info "Restarting GSCoWork service..."
    systemctl restart "$SERVICE_NAME"
    print_info "Service restarted."
}

reload_service() {
    check_root
    print_info "Reloading systemd daemon..."
    systemctl daemon-reload
    print_info "Reloading GSCoWork service..."
    systemctl reload-or-restart "$SERVICE_NAME"
    print_info "Service reloaded."
}

status_service() {
    systemctl status "$SERVICE_NAME" --no-pager || true
}

show_logs() {
    journalctl -u "$SERVICE_NAME" -f
}

show_recent_logs() {
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager
}

update_binary() {
    check_root

    print_info "Updating GSCoWork binary..."

    # Copy new binary
    if [[ -f "${SCRIPT_DIR}/../gscowork" ]]; then
        cp "${SCRIPT_DIR}/../gscowork" "$INSTALL_DIR/"
    elif [[ -f "${SCRIPT_DIR}/gscowork" ]]; then
        cp "${SCRIPT_DIR}/gscowork" "$INSTALL_DIR/"
    else
        print_error "gscowork binary not found. Please build it first: go build -o gscowork ."
        exit 1
    fi

    chmod +x "$INSTALL_DIR/gscowork"
    chown www-data:www-data "$INSTALL_DIR/gscowork"

    # Restart service
    systemctl restart "$SERVICE_NAME"

    print_info "Binary updated and service restarted."
}

show_usage() {
    echo "GSCoWork Service Management"
    echo ""
    echo "Usage: $0 {command}"
    echo ""
    echo "Commands:"
    echo "  install     Install and enable the systemd service"
    echo "  uninstall   Remove the systemd service (keeps data)"
    echo "  start       Start the service"
    echo "  stop        Stop the service"
    echo "  restart     Restart the service"
    echo "  reload      Reload systemd and restart service"
    echo "  status      Show service status"
    echo "  logs        Follow service logs (Ctrl+C to exit)"
    echo "  logs-recent Show last 50 log entries"
    echo "  update      Update binary and restart service"
    echo ""
    echo "Examples:"
    echo "  sudo $0 install    # First time setup"
    echo "  sudo $0 restart    # Restart after config change"
    echo "  $0 status          # Check if running"
    echo "  $0 logs            # View live logs"
}

case "$1" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    reload)
        reload_service
        ;;
    status)
        status_service
        ;;
    logs)
        show_logs
        ;;
    logs-recent)
        show_recent_logs
        ;;
    update)
        update_binary
        ;;
    *)
        show_usage
        exit 1
        ;;
esac

exit 0
