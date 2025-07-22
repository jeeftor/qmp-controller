#!/bin/bash

# Simple screen clearing using standard commands
# More reliable than ANSI escape sequences sent via individual keypresses

VMID=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --vmid)
            VMID="$2"
            shift 2
            ;;
        --help)
            echo "Simple Screen Clear"
            echo "Usage: $0 --vmid VMID"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ -z "$VMID" ]]; then
    echo "Error: --vmid required"
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

echo "Clearing screen for VM $VMID using reliable methods..."

# Method 1: Use the clear command
echo "Sending 'clear' command..."
$QMP_BIN keyboard type "$VMID" "clear"
$QMP_BIN keyboard send "$VMID" "enter"
sleep 1

# Method 2: Use printf with ANSI codes (more reliable as a command)
echo "Sending printf with ANSI escape codes..."
$QMP_BIN keyboard type "$VMID" 'printf "\033[2J\033[H"'
$QMP_BIN keyboard send "$VMID" "enter"
sleep 1

# Method 3: Use tput for maximum compatibility
echo "Sending tput clear..."
$QMP_BIN keyboard type "$VMID" "tput clear"
$QMP_BIN keyboard send "$VMID" "enter"
sleep 1

echo "Screen clearing commands sent. The screen should now be clear."
echo ""
echo "Alternative methods you can try manually:"
echo '  ./qmp-controller keyboard type 108 "clear && printf \"\\033[2J\\033[H\""'
echo '  ./qmp-controller keyboard type 108 "reset"'
echo '  ./qmp-controller keyboard type 108 "tput clear"'
