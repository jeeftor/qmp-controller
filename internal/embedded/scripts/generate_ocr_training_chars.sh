#!/bin/bash

# OCR Training Character Generator for Console Fonts
# Outputs characters in formats optimized for OCR bitmap training
# Compatible with qmp-controller OCR training pipeline

# Configuration
FONT_NAME="eurlatgr"
OUTPUT_MODE="individual"  # individual, grid, or mixed

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --font)
            FONT_NAME="$2"
            shift 2
            ;;
        --mode)
            OUTPUT_MODE="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --font FONT    Set console font (default: eurlatgr)"
            echo "  --mode MODE    Output mode: individual, grid, mixed (default: individual)"
            echo "  --help         Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Set font if available
if command -v setfont >/dev/null 2>&1; then
    setfont "$FONT_NAME" 2>/dev/null || echo "# Warning: $FONT_NAME font not available, using default" >&2
fi

# Function to output individual characters (best for OCR training)
output_individual() {
    local start=$1
    local end=$2

    for ((i=start; i<=end; i++)); do
        # Skip control characters
        if [[ $i -ge 32 && $i -le 126 ]] || [[ $i -ge 160 && $i -le 255 ]]; then
            char=$(printf "\\$(printf "%03o" $i)")
            # Output just the character on its own line
            echo "$char"
        fi
    done
}

# Function to output character grids (useful for context training)
output_grid() {
    local start=$1
    local end=$2
    local cols=$3

    local count=0
    for ((i=start; i<=end; i++)); do
        if [[ $i -ge 32 && $i -le 126 ]] || [[ $i -ge 160 && $i -le 255 ]]; then
            char=$(printf "\\$(printf "%03o" $i)")
            printf "%s" "$char"
            ((count++))
            if ((count % cols == 0)); then
                echo ""
            else
                printf " "
            fi
        fi
    done
    [[ $((count % cols)) -ne 0 ]] && echo ""
}

# Function to output mixed format
output_mixed() {
    echo "# Single characters for individual training"
    output_individual 32 126
    output_individual 160 255

    echo ""
    echo "# Character grids for context training"
    output_grid 65 90 13  # A-Z
    echo ""
    output_grid 97 122 13 # a-z
    echo ""
    output_grid 48 57 10  # 0-9
    echo ""
    output_grid 32 47 16  # punctuation
    echo ""
    output_grid 160 255 16 # extended characters
}

# Main execution based on mode
case $OUTPUT_MODE in
    "individual")
        # Output each character on its own line - best for OCR training
        output_individual 32 126    # ASCII printable
        output_individual 160 255   # Latin-1 supplement
        ;;
    "grid")
        # Output characters in grids
        echo "# ASCII Letters"
        output_grid 65 90 13   # A-Z
        echo ""
        output_grid 97 122 13  # a-z
        echo ""
        echo "# Numbers"
        output_grid 48 57 10   # 0-9
        echo ""
        echo "# Punctuation"
        output_grid 33 47 15   # !"#$%&'()*+,-./
        output_grid 58 64 7    # :;<=>?@
        output_grid 91 96 6    # [\]^_`
        output_grid 123 126 4  # {|}~
        echo ""
        echo "# Extended Characters"
        output_grid 160 255 16
        ;;
    "mixed")
        output_mixed
        ;;
    *)
        echo "Error: Unknown output mode: $OUTPUT_MODE" >&2
        exit 1
        ;;
esac
