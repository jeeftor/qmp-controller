#!/bin/bash

# Script to generate training data for eurlatgr console font
# This script outputs all supported characters in a systematic way
# suitable for OCR training data generation

# Set the font to eurlatgr
setfont eurlatgr 2>/dev/null || echo "Warning: eurlatgr font not available, using default"

# Function to print a character range with labels
print_range() {
    local start=$1
    local end=$2
    local label="$3"

    echo "=== $label ==="
    for ((i=start; i<=end; i++)); do
        # Convert decimal to hex for display
        hex=$(printf "%02X" $i)
        # Use printf to output the character
        char=$(printf "\\$(printf "%03o" $i)")

        # Skip control characters and non-printable chars
        if [[ $i -ge 32 && $i -le 126 ]] || [[ $i -ge 160 && $i -le 255 ]]; then
            echo "U+$hex: $char"
        fi
    done
    echo ""
}

# Function to print characters in a grid format (useful for screenshots)
print_grid() {
    local start=$1
    local end=$2
    local cols=$3
    local label="$4"

    echo "=== $label (Grid Format) ==="
    local count=0
    for ((i=start; i<=end; i++)); do
        # Skip control characters and non-printable chars
        if [[ $i -ge 32 && $i -le 126 ]] || [[ $i -ge 160 && $i -le 255 ]]; then
            char=$(printf "\\$(printf "%03o" $i)")
            printf "%s " "$char"
            ((count++))
            if ((count % cols == 0)); then
                echo ""
            fi
        fi
    done
    echo -e "\n"
}

# Function to print all characters on single lines (good for OCR line detection)
print_single_chars() {
    local start=$1
    local end=$2
    local label="$3"

    echo "=== $label (Single Characters) ==="
    for ((i=start; i<=end; i++)); do
        if [[ $i -ge 32 && $i -le 126 ]] || [[ $i -ge 160 && $i -le 255 ]]; then
            char=$(printf "\\$(printf "%03o" $i)")
            echo "$char"
        fi
    done
    echo ""
}

# Main execution
echo "Font Training Data Generator for eurlatgr"
echo "========================================"
echo ""

# Basic ASCII printable characters (32-126)
print_range 32 126 "Basic ASCII Printable Characters (32-126)"
print_grid 32 126 16 "ASCII Grid"
print_single_chars 32 126 "ASCII Single Lines"

# Latin-1 Supplement (160-255)
print_range 160 255 "Latin-1 Supplement (160-255)"
print_grid 160 255 16 "Latin-1 Grid"
print_single_chars 160 255 "Latin-1 Single Lines"

# Print some common combinations and words
echo "=== Common Character Combinations ==="
echo "The quick brown fox jumps over the lazy dog"
echo "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
echo "abcdefghijklmnopqrstuvwxyz"
echo "0123456789"
echo "!@#$%^&*()_+-=[]{}|;':\",./<>?"
echo ""

# Print numbers in different contexts
echo "=== Numbers in Context ==="
for i in {0..9}; do
    echo "Number $i: $i$i$i"
done
echo ""

# Print alphabet in different cases
echo "=== Alphabet Variations ==="
echo "Uppercase: ABCDEFGHIJKLMNOPQRSTUVWXYZ"
echo "Lowercase: abcdefghijklmnopqrstuvwxyz"
echo "Mixed: AbCdEfGhIjKlMnOpQrStUvWxYz"
echo ""

# Print common punctuation combinations
echo "=== Punctuation Combinations ==="
echo "Periods: ... ."
echo "Commas: , ,, ,,,"
echo "Quotes: \"Hello\" 'World'"
echo "Brackets: [test] {test} (test)"
echo "Math: 2+2=4, 10-5=5, 3*3=9, 8/2=4"
echo ""

# European characters if available
echo "=== European Characters (if supported) ==="
# French
echo "Français: àáâãäåæçèéêëìíîï"
# German
echo "Deutsch: äöüßÄÖÜ"
# Spanish
echo "Español: ñáéíóúü¿¡"
# Scandinavian
echo "Skandinavisk: æøåÆØÅ"
echo ""

echo "Training data generation complete!"
echo "Tip: Redirect output to a file: $0 > training_output.txt"
echo "Or take screenshots while running: $0 | less"
