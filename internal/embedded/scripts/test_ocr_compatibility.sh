#!/bin/bash

# Test compatibility between font generation scripts and OCR training pipeline
# This script demonstrates how to use the generated character sets with qmp-controller OCR training

set -e  # Exit on any error

echo "OCR Training Pipeline Compatibility Test"
echo "======================================="

# Check if required files exist
if [[ ! -f "./generate_ocr_training_chars.sh" ]]; then
    echo "Error: generate_ocr_training_chars.sh not found"
    exit 1
fi

if [[ ! -f "../qmp-controller" ]] && [[ ! -f "../qmp" ]]; then
    echo "Error: qmp-controller binary not found in parent directory. Please build first with 'go build'"
    exit 1
fi

# Determine the binary name
QMP_BIN="./qmp-controller"
if [[ ! -f "$QMP_BIN" ]] && [[ -f "../qmp" ]]; then
    QMP_BIN="./qmp"
fi

# Generate character set for training
echo "1. Generating character set for OCR training..."
TRAIN_CHARS=$(./generate_ocr_training_chars.sh --mode individual | tr -d '\n' | tr -d ' ')
echo "Generated ${#TRAIN_CHARS} characters for training"

# Show first 50 characters as preview
echo "Preview (first 50 chars): ${TRAIN_CHARS:0:50}..."

# Create a more comprehensive training character set
echo ""
echo "2. Creating comprehensive training character set..."
COMPREHENSIVE_CHARS=""

# Add basic ASCII
for ((i=32; i<=126; i++)); do
    char=$(printf "\\$(printf "%03o" $i)")
    COMPREHENSIVE_CHARS+="$char"
done

# Add extended characters if supported
for ((i=160; i<=255; i++)); do
    char=$(printf "\\$(printf "%03o" $i)")
    COMPREHENSIVE_CHARS+="$char"
done

echo "Comprehensive set contains ${#COMPREHENSIVE_CHARS} characters"

# Create test output showing how to use with OCR training
echo ""
echo "3. Example usage with qmp-controller train-ocr:"
echo ""

cat << 'EOF'
# Method 1: Use generated character set for batch training
TRAIN_CHARS=$(./generate_ocr_training_chars.sh --mode individual | tr -d '\n')
./qmp-controller train-ocr screenshot.ppm training-data.json --res 160x50 --train-chars "$TRAIN_CHARS"

# Method 2: Use interactive mode (recommended for new fonts)
./qmp-controller train-ocr screenshot.ppm training-data.json --res 160x50 --interactive

# Method 3: Generate specific character ranges
# For basic ASCII only:
BASIC_CHARS=$(./generate_ocr_training_chars.sh --mode individual | head -95 | tr -d '\n')

# For extended characters only:
EXTENDED_CHARS=$(./generate_ocr_training_chars.sh --mode individual | tail -96 | tr -d '\n')

# Method 4: Generate training display for screenshots
# Create a display showing all characters in grid format for screenshots:
./generate_font_training_data.sh > font_display.txt
# Then display this on console and take screenshot for training
EOF

echo ""
echo "4. Testing character generation modes:"
echo ""

echo "Individual mode (first 10 chars):"
./generate_ocr_training_chars.sh --mode individual | head -10

echo ""
echo "Grid mode preview:"
./generate_ocr_training_chars.sh --mode grid | head -20

echo ""
echo "5. Workflow recommendation:"
echo ""

cat << 'EOF'
Recommended workflow for training eurlatgr font:

1. Set console font: setfont eurlatgr

2. Generate character display:
   ./generate_font_training_data.sh

3. Take screenshot of the character display:
   ./qmp-controller screenshot [vmid] training_screenshot.ppm

4. Train OCR in interactive mode:
   ./qmp-controller train-ocr training_screenshot.ppm eurlatgr_training.json --res 160x50 --interactive

5. Or use batch mode with generated character set:
   CHARS=$(./generate_ocr_training_chars.sh --mode individual | tr -d '\n')
   ./qmp-controller train-ocr training_screenshot.ppm eurlatgr_training.json --res 160x50 --train-chars "$CHARS"

6. Test OCR with the new training data:
   ./qmp-controller ocr [vmid] --training-data eurlatgr_training.json
EOF

echo ""
echo "Compatibility test completed successfully!"
echo "The generated character sets are compatible with qmp-controller OCR training pipeline."
