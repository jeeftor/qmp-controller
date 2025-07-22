#!/bin/bash

# Advanced Screen Clearing Script for OCR Training
# Provides various methods to completely clear and reset the terminal screen

# Parse command line arguments
CLEAR_METHOD="full"
VMID=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --vmid)
            VMID="$2"
            shift 2
            ;;
        --method)
            CLEAR_METHOD="$2"
            shift 2
            ;;
        --help)
            cat << 'EOF'
Advanced Screen Clearing for OCR Training

Usage: ./clear_screen.sh [options]

Options:
  --vmid VMID         Send clear commands to VM via QMP (optional)
  --method METHOD     Clear method: full, complete, reset, or all (default: full)
  --help              Show this help

Clear Methods:
  full      - Clear entire screen and reset cursor (ESC[2J + ESC[H)
  complete  - Full clear plus clear scrollback buffer (ESC[3J)
  reset     - Complete terminal reset (ESC c)
  all       - All methods combined

Examples:
  # Clear local terminal completely
  ./clear_screen.sh --method complete

  # Clear VM screen via QMP
  ./clear_screen.sh --vmid 108 --method full

  # Nuclear option - complete reset
  ./clear_screen.sh --vmid 108 --method reset
EOF
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Function to send clear commands via QMP
send_via_qmp() {
    local sequence="$1"
    local description="$2"

    if [[ -z "$VMID" ]]; then
        echo "Error: --vmid required for QMP commands"
        exit 1
    fi

    # Check if qmp-controller exists
    QMP_BIN="./qmp-controller"
    if [[ ! -f "$QMP_BIN" ]] && [[ -f "../qmp" ]]; then
        QMP_BIN="./qmp"
    elif [[ ! -f "$QMP_BIN" ]]; then
        echo "Error: qmp-controller binary not found in parent directory"
        exit 1
    fi

    echo "Sending $description to VM $VMID..."
    $QMP_BIN keyboard type "$VMID" "$sequence"
    sleep 0.5
}

# Function to send clear commands locally
send_locally() {
    local sequence="$1"
    local description="$2"

    echo "Executing $description locally..."
    printf "$sequence"
}

# Function to execute clear command
execute_clear() {
    local sequence="$1"
    local description="$2"

    if [[ -n "$VMID" ]]; then
        send_via_qmp "$sequence" "$description"
    else
        send_locally "$sequence" "$description"
    fi
}

echo "Advanced Screen Clearing"
echo "======================="
echo "Method: $CLEAR_METHOD"
if [[ -n "$VMID" ]]; then
    echo "Target: VM $VMID (via QMP)"
else
    echo "Target: Local terminal"
fi
echo ""

case "$CLEAR_METHOD" in
    "full")
        # ESC[2J - Clear entire screen
        # ESC[H - Move cursor to home position (1,1)
        execute_clear $'\033[2J\033[H' "full screen clear with cursor reset"
        ;;
    "complete")
        # ESC[2J - Clear entire screen
        # ESC[3J - Clear scrollback buffer
        # ESC[H - Move cursor to home position
        execute_clear $'\033[2J\033[3J\033[H' "complete screen and scrollback clear"
        ;;
    "reset")
        # ESC c - Complete terminal reset (hard reset)
        execute_clear $'\033c' "complete terminal reset"
        ;;
    "all")
        echo "Executing all clear methods..."
        execute_clear $'\033[2J\033[H' "full screen clear"
        sleep 0.5
        execute_clear $'\033[3J' "scrollback buffer clear"
        sleep 0.5
        execute_clear $'\033c' "terminal reset"
        ;;
    *)
        echo "Error: Unknown clear method: $CLEAR_METHOD"
        echo "Valid methods: full, complete, reset, all"
        exit 1
        ;;
esac

echo ""
echo "Screen clearing completed."

# Additional ANSI codes reference
cat << 'EOF'

=== ANSI Clear Code Reference ===
ESC[2J    - Clear entire screen
ESC[3J    - Clear scrollback buffer
ESC[H     - Move cursor to home (1,1)
ESC[1J    - Clear from cursor to beginning
ESC[0J    - Clear from cursor to end
ESC c     - Reset terminal completely
ESC[r     - Reset scroll region

=== Usage in QMP Commands ===
# Basic clear (recommended for OCR training):
./qmp-controller keyboard type VMID $'\033[2J\033[H'

# Nuclear clear (if screen has artifacts):
./qmp-controller keyboard type VMID $'\033c'

# Complete clear including scrollback:
./qmp-controller keyboard type VMID $'\033[2J\033[3J\033[H'
EOF
