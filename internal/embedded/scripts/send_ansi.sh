#!/bin/bash

# Send ANSI escape sequences via QMP
# Breaks down ANSI codes into individual key presses

VMID=""
SEQUENCE=""
DESCRIPTION=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --vmid)
            VMID="$2"
            shift 2
            ;;
        --seq|--sequence)
            SEQUENCE="$2"
            shift 2
            ;;
        --desc|--description)
            DESCRIPTION="$2"
            shift 2
            ;;
        --help)
            cat << 'EOF'
Send ANSI Escape Sequences via QMP

Usage: ./send_ansi.sh --vmid VMID --sequence SEQUENCE [--description DESC]

Options:
  --vmid VMID         VM ID for QMP connection
  --sequence SEQ      ANSI sequence to send (e.g., "2J", "H", "3J")
  --description DESC  Description for logging
  --help              Show this help

Predefined sequences:
  clear-screen        ESC[2J - Clear entire screen
  cursor-home         ESC[H - Move cursor to home (1,1)
  clear-scrollback    ESC[3J - Clear scrollback buffer
  reset-terminal      ESC c - Complete terminal reset
  clear-full          ESC[2J + ESC[H - Clear screen and reset cursor
  clear-complete      ESC[2J + ESC[3J + ESC[H - Complete clear

Examples:
  # Clear screen
  ./send_ansi.sh --vmid 108 --sequence clear-screen

  # Clear screen and reset cursor
  ./send_ansi.sh --vmid 108 --sequence clear-full

  # Send custom sequence
  ./send_ansi.sh --vmid 108 --sequence "2J" --desc "Clear screen"
EOF
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ -z "$VMID" ]] || [[ -z "$SEQUENCE" ]]; then
    echo "Error: --vmid and --sequence are required"
    echo "Use --help for usage information"
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

# Function to send individual keys with delay
send_keys() {
    local keys=("$@")
    for key in "${keys[@]}"; do
        echo "Sending key: $key"
        if ! $QMP_BIN keyboard send "$VMID" "$key"; then
            echo "Error sending key: $key"
            return 1
        fi
        sleep 0.1
    done
}

# Function to send ESC sequence
send_esc_sequence() {
    local seq="$1"
    echo "Sending ESC[$seq"

    # Send ESC key first
    if ! $QMP_BIN keyboard send "$VMID" "esc"; then
        echo "Error sending ESC key"
        return 1
    fi
    sleep 0.1

    # Send opening bracket if needed
    if [[ "$seq" != "c" ]]; then
        if ! $QMP_BIN keyboard send "$VMID" "["; then
            echo "Error sending [ key"
            return 1
        fi
        sleep 0.1
    fi

    # Send sequence characters
    for ((i=0; i<${#seq}; i++)); do
        char="${seq:$i:1}"
        if [[ "$char" != "c" ]] || [[ "$seq" == "c" ]]; then
            echo "Sending: $char"
            if ! $QMP_BIN keyboard send "$VMID" "$char"; then
                echo "Error sending character: $char"
                return 1
            fi
            sleep 0.1
        fi
    done
}

echo "Sending ANSI sequence to VM $VMID"
echo "Sequence: $SEQUENCE"
[[ -n "$DESCRIPTION" ]] && echo "Description: $DESCRIPTION"
echo ""

# Handle predefined sequences
case "$SEQUENCE" in
    "clear-screen")
        send_esc_sequence "2J"
        ;;
    "cursor-home")
        send_esc_sequence "H"
        ;;
    "clear-scrollback")
        send_esc_sequence "3J"
        ;;
    "reset-terminal")
        send_esc_sequence "c"
        ;;
    "clear-full")
        echo "Sending clear screen + cursor home..."
        send_esc_sequence "2J"
        sleep 0.2
        send_esc_sequence "H"
        ;;
    "clear-complete")
        echo "Sending complete clear sequence..."
        send_esc_sequence "2J"
        sleep 0.2
        send_esc_sequence "3J"
        sleep 0.2
        send_esc_sequence "H"
        ;;
    *)
        # Custom sequence
        send_esc_sequence "$SEQUENCE"
        ;;
esac

echo ""
echo "ANSI sequence sent successfully!"
