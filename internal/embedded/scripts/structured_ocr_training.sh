#!/bin/bash

# Structured OCR Training Script
# Displays known character sets at specific line positions for controlled training
# Compatible with qmp-controller OCR training pipeline

# Configuration
LINES_PER_SET=1
CLEAR_SCREEN=true
FONT_NAME="eurlatgr"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --font)
            FONT_NAME="$2"
            shift 2
            ;;
        --lines)
            LINES_PER_SET="$2"
            shift 2
            ;;
        --no-clear)
            CLEAR_SCREEN=false
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --font FONT     Set console font (default: eurlatgr)"
            echo "  --lines N       Characters per line (default: 1)"
            echo "  --no-clear      Don't clear screen before output"
            echo "  --help          Show this help"
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

# Clear screen completely using ANSI codes if requested
if [[ "$CLEAR_SCREEN" == "true" ]]; then
    # Use ANSI escape sequences for complete screen clear
    # ESC[2J clears entire screen, ESC[H moves cursor to home (1,1)
    printf '\033[2J\033[H'
fi

# Function to output character set with known positions
output_structured_training() {
    echo "=== Structured OCR Training Data ==="
    echo "Font: $FONT_NAME"
    echo "Format: Each line contains known characters for training"
    echo ""

    # Line 1: Basic lowercase letters
    echo "abcdefghijklmnopqrstuvwxyz"

    # Line 2: Basic uppercase letters
    echo "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

    # Line 3: Numbers
    echo "0123456789"

    # Line 4: Basic punctuation
    echo "!@#$%^&*()-_=+[]{}|;':\",./<>?"

    # Line 5: More punctuation and symbols
    echo "\`~"

    # Line 6: Common European characters (if supported by font)
    echo "àáâãäåæçèéêëìíîïñòóôõöøùúûüýÿ"

    # Line 7: More European characters
    echo "ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÑÒÓÔÕÖØÙÚÛÜÝŸ"

    # Line 8: Additional symbols and characters
    echo "¡¢£¤¥¦§¨©ª«¬®¯°±²³´µ¶·¸¹º»¼½¾¿"
}

# Function to generate training command
generate_training_command() {
    cat << 'EOF'

=== Training Commands ===
# After displaying this output and taking a screenshot:

# Method 1: Train specific lines with known character sets
./qmp-controller train-ocr screenshot.ppm training-data.json --res 160x50 --interactive

# Method 2: For batch training, create character strings for each line:
LINE1="abcdefghijklmnopqrstuvwxyz"
LINE2="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
LINE3="0123456789"
LINE4="!@#$%^&*()-_=+[]{}|;':\",./<>?"
LINE5="\`~"
LINE6="àáâãäåæçèéêëìíîïñòóôõöøùúûüýÿ"
LINE7="ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÑÒÓÔÕÖØÙÚÛÜÝŸ"
LINE8="¡¢£¤¥¦§¨©ª«¬®¯°±²³´µ¶·¸¹º»¼½¾¿"

# Combine all lines for comprehensive training
ALL_CHARS="$LINE1$LINE2$LINE3$LINE4$LINE5$LINE6$LINE7$LINE8"
./qmp-controller train-ocr screenshot.ppm training-data.json --res 160x50 --train-chars "$ALL_CHARS"

EOF
}

# Main execution
output_structured_training
generate_training_command

# Output character mapping for reference
cat << 'EOF'

=== Line-by-Line Character Reference ===
Line 1 (lowercase): abcdefghijklmnopqrstuvwxyz
Line 2 (uppercase): ABCDEFGHIJKLMNOPQRSTUVWXYZ
Line 3 (numbers): 0123456789
Line 4 (punctuation): !@#$%^&*()-_=+[]{}|;':",./<>?
Line 5 (symbols): `~
Line 6 (european_lower): àáâãäåæçèéêëìíîïñòóôõöøùúûüýÿ
Line 7 (european_upper): ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÑÒÓÔÕÖØÙÚÛÜÝŸ
Line 8 (extended): ¡¢£¤¥¦§¨©ª«¬®¯°±²³´µ¶·¸¹º»¼½¾¿

=== Training Workflow ===
1. Run this script to display structured characters
2. Take screenshot: ./qmp-controller screenshot [vmid] training.ppm
3. Train with known positions:
   ./qmp-controller train-ocr training.ppm data.json --res 160x50 --interactive

   Or specify the exact character sequence:
   CHARS="abcd...xyz..." (concatenate all lines above)
   ./qmp-controller train-ocr training.ppm data.json --res 160x50 --train-chars "$CHARS"

EOF
