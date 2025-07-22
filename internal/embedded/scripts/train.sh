#!/bin/bash

# Set default VMID
DEFAULT_VMID=108

# Check for --vmid flag
while [ $# -gt 0 ]; do
    case "$1" in
        --vmid)
            shift
            VMID="$1"
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
    shift
done

# If VMID is not set, prompt the user
if [ -z "$VMID" ]; then
    read -p "Enter VMID (default: $DEFAULT_VMID): " VMID
    # Use default if no input is provided
    VMID=${VMID:-$DEFAULT_VMID}
fi

# Validate that VMID is a number
if ! [[ "$VMID" =~ ^[0-9]+$ ]]; then
    echo "Error: VMID must be a number"
    exit 1
fi

# Output the VMID
echo "Using VMID: $VMID"

./qmp-controller script $VMID ./scripts/training_data_script.txt  --debug
./qmp-controller screenshot $VMID training.ppm
