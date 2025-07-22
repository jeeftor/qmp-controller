# OCR Training Scripts

This directory contains scripts for training the QMP Controller's OCR system with console fonts.

## Scripts Overview

### Core Training Scripts

- **`training_pipeline.sh`** - Complete automated training pipeline
- **`structured_ocr_training.sh`** - Display structured character sets for manual training
- **`generate_ocr_training_chars.sh`** - Generate character sequences for training

### Utility Scripts

- **`send_ansi.sh`** - Send ANSI escape sequences via QMP
- **`clear_screen.sh`** - Advanced screen clearing options
- **`generate_font_training_data.sh`** - Generate comprehensive font training displays

### Testing Scripts

- **`test_ocr_compatibility.sh`** - Test compatibility between scripts and OCR pipeline

## Quick Start

1. **Build the QMP controller** (in parent directory):
   ```bash
   go build
   ```

2. **Run the complete training pipeline**:
   ```bash
   cd scripts
   ./training_pipeline.sh --vmid 108
   ```

3. **Or clear screen and display training data manually**:
   ```bash
   ./send_ansi.sh --vmid 108 --sequence clear-full
   ./structured_ocr_training.sh
   ```

## Usage Examples

### Complete Automated Training
```bash
# Full pipeline with eurlatgr font
./training_pipeline.sh --vmid 108 --font eurlatgr

# Interactive training mode
./training_pipeline.sh --vmid 108 --mode interactive

# Batch training mode
./training_pipeline.sh --vmid 108 --mode batch
```

### Manual Training Workflow
```bash
# 1. Clear screen completely
./send_ansi.sh --vmid 108 --sequence clear-complete

# 2. Display structured training data
./structured_ocr_training.sh > training_display.txt

# 3. Send training data to VM
../qmp-controller keyboard type 108 "$(cat training_display.txt)"

# 4. Take screenshot and train
../qmp-controller screenshot 108 training.ppm
../qmp-controller train-ocr training.ppm data.json --res 160x50 --interactive
```

### ANSI Screen Control
```bash
# Clear screen and reset cursor
./send_ansi.sh --vmid 108 --sequence clear-full

# Complete terminal reset
./send_ansi.sh --vmid 108 --sequence reset-terminal

# Clear with scrollback buffer
./send_ansi.sh --vmid 108 --sequence clear-complete
```

## Notes

- All scripts expect the `qmp-controller` binary to be in the parent directory
- Scripts are designed for console font training (eurlatgr, lat0-16, etc.)
- Interactive training mode is recommended for new fonts
- Use batch mode when you have a known character sequence

## Training Workflow

1. **Set Font**: `setfont eurlatgr`
2. **Clear Screen**: Use ANSI sequences for clean display
3. **Display Characters**: Structured output with known positions
4. **Screenshot**: Capture the training data
5. **Train OCR**: Use interactive or batch mode
6. **Test**: Verify training data works correctly
