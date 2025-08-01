# QMP Controller

A powerful Go-based CLI tool for managing QEMU virtual machines via QMP (QEMU Machine Protocol) with advanced automation, OCR integration, and scripting capabilities.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

## 🚀 Features

### Core VM Management
- **QMP Protocol Integration**: Direct control of QEMU VMs via QMP sockets
- **Multi-VM Support**: Manage multiple VMs simultaneously with connection pooling
- **Socket Forwarding**: SSH-based remote VM access with automatic setup
- **Real-time Status**: Live VM monitoring and resource tracking

### Advanced Automation
- **Enhanced Script2 System**: Comprehensive automation with bash-style variables, functions, and control flow
- **OCR Integration**: Screen-based automation using optical character recognition
- **Flexible Argument Ordering**: Smart file type detection allows arguments in any order
- **Interactive Live Mode**: Real-time VM interaction with command history and status display
- **USB Device Management**: Dynamic USB device attachment and control

### Developer Tools
- **Configuration Management**: Complete config system with profiles, validation, and environment variables
- **VSCode Extension**: Syntax highlighting for script2 files with automated grammar generation
- **Environment Variable Support**: 23 comprehensive `QMP_*` environment variables with priority system
- **Profile System**: Named configurations for dev/staging/prod environments
- **Structured Logging**: Performance monitoring and contextual debugging
- **Resource Management**: Automatic cleanup and connection pooling
- **Training Data Generation**: OCR model training and optimization

## 📦 Installation

### From Source
```bash
git clone https://github.com/jeeftor/qmp-controller.git
cd qmp-controller
make build
```

### Pre-built Binaries
Download from the [releases page](https://github.com/jeeftor/qmp-controller/releases) or build with:
```bash
make build-amd    # AMD64 Linux
make build-arm    # ARM64 Linux
make build-mac-arm # ARM64 macOS
```

## 🛠️ Quick Start

### 1. Configuration Setup
```bash
# Generate a comprehensive configuration file
qmp config init                               # Creates ~/.qmp.yaml with examples

# Set up common environment variables
export QMP_VM_ID=106                          # Default VM for all operations
export QMP_TRAINING_DATA=~/training.json      # OCR training data

# View current configuration
qmp config show                               # Shows settings and sources
qmp env                                       # Shows all 23 environment variables
```

### 2. Basic VM Control (with environment variables)
```bash
# Check VM status (uses QMP_VM_ID if set)
qmp status 106
qmp status                                    # Uses QMP_VM_ID=106

# Take screenshot (with environment variables)
qmp screenshot 106 output.png
qmp screenshot output.png                     # Uses QMP_VM_ID=106

# Interactive keyboard mode
qmp keyboard live 106
qmp keyboard live                             # Uses QMP_VM_ID=106

# USB device management
qmp usb add 106 /dev/sdb
qmp usb list                                  # Uses QMP_VM_ID=106
```

### 3. Configuration Profiles
```bash
# Use profiles for different environments
qmp --profile dev status                      # Development VM settings
qmp --profile prod ocr vm "login:"            # Production VM + settings
qmp --profile test keyboard live              # Test environment

# Profiles can include VM ID, socket paths, OCR settings, etc.
```

### 4. OCR and Screen Automation
```bash
# Generate OCR training data
qmp train_ocr vm 106 training.json
qmp train_ocr vm training.json                # Uses QMP_VM_ID=106

# Search for text (flexible argument order + env vars)
qmp ocr find vm 106 training.json "login:"
qmp ocr find vm "login:"                      # Uses QMP_VM_ID + QMP_TRAINING_DATA

# Regex search
qmp ocr re vm 106 training.json "\\berror\\b" --ignore-case
qmp ocr re vm "\\berror\\b" --ignore-case     # Uses environment variables
```

### 5. Script Automation
```bash
# Execute enhanced scripts (flexible argument order)
qmp script2 106 automation.sc2 training.json --var USER=admin
qmp script2 automation.sc2 --var USER=admin  # Uses QMP_VM_ID + QMP_TRAINING_DATA

# Generate sample scripts
qmp script2 sample > my-script.sc2            # Enhanced scripting examples
qmp script sample > basic-script.txt         # Original script format
```

## 📝 Script2 Language

The enhanced Script2 system provides a powerful automation language with modern programming constructs:

### Variables and Expansion
```bash
# Variable definitions
USER=${USER:-admin}
PASSWORD=${PASSWORD:-secret}
TARGET=${TARGET_HOST:-server.local}

# Usage in commands
ssh $USER@$TARGET
echo "Connecting as: ${USER}"
```

### Control Flow
```bash
# Conditional execution
<if-found "login:" 10s>
echo "Login prompt detected"
$USER
<else>
echo "No login prompt found"
<exit 1>

# Loops
<retry 3>
ping -c 1 $TARGET

<while-not-found "$ " 60s poll 2s>
echo "Waiting for shell prompt..."
```

### Functions
```bash
# Function definition
<function connect_ssh>
ssh $1@$2
<watch "password:" 10s>
$3
<enter>
<end-function>

# Function calls
<call connect_ssh $USER $TARGET $PASSWORD>
```

### Advanced Features
```bash
# Screenshot capture
<screenshot "login-{timestamp}.png">

# Script composition
<include "common-functions.sc2">

# Console switching
<console 2>

# Timing control
<wait 5s>
<watch "ready" 30s>
```

### Enhanced TUI Debugging
```bash
# Interactive debugging with dual-pane view and syntax highlighting
qmp script2 106 automation.sc2 --debug-tui

# Console debugging (SSH/remote friendly)
qmp script2 106 automation.sc2 --debug-console

# Step-by-step debugging
qmp script2 106 automation.sc2 --debug

# Set breakpoints on specific lines
qmp script2 106 automation.sc2 --debug-tui --breakpoint 10,25,50
```

**TUI Debugging Features:**
- **Dual-Pane Mode**: Press `t` to toggle side-by-side views (Script + OCR/Variables/etc.)
- **VSCode-Style Syntax Highlighting**: Rainbow colors matching the VSCode extension
- **Smart Navigation**: Number keys (1-7) switch views, left/right arrows navigate panes
- **State Persistence**: Dual-pane mode and view selections persist across debug sessions
- **Live OCR Integration**: Real-time screen monitoring with search capabilities
- **Multiple Views**: Script, Variables, Breakpoints, OCR Preview, Watch Progress, Performance, Help

## 🏗️ Architecture

### Core Packages

#### `internal/qmp`
QMP protocol client implementation for VM control with connection pooling and error handling.

#### `internal/script2`
Enhanced scripting engine with:
- **Parser** (`parser.go`): Syntax analysis and AST generation
- **Executor** (`executor.go`): Runtime execution with OCR integration
- **Variables** (`variables.go`): Bash-compatible variable expansion
- **Types** (`types.go`): Core data structures and interfaces

#### `internal/ocr`
OCR engine for screen-based automation:
- **OCR** (`ocr.go`): Character recognition and training
- **Search** (`search.go`): Pattern matching and text search

#### `internal/logging`
Structured logging with contextual loggers, performance monitoring, and user-facing output formatting.

#### `internal/resource`
Resource management system:
- **Manager** (`manager.go`): Connection pooling and cleanup
- **Context** (`context.go`): Application-wide context management
- **Screenshot** (`screenshot.go`): Batch screenshot operations

### Command Structure
Built with Cobra framework for consistent CLI experience:

```
cmd/
├── root.go          # Base command and configuration
├── env.go           # Environment variable display
├── status.go        # VM status monitoring
├── screenshot.go    # Screenshot capture
├── keyboard.go      # Interactive keyboard control
├── ocr.go          # OCR operations and search
├── script.go       # Original script execution
├── script2.go      # Enhanced script system
├── train_ocr.go    # OCR training data generation
├── usb.go          # USB device management
└── version.go      # Version information
```

## 🎨 VSCode Extension

The project includes an automated VSCode extension for Script2 syntax highlighting:

### Features
- **Granular Syntax Highlighting**: Each component gets distinct colors
  - `<watch "text" 5s>`: brackets (gray), keyword (blue), string (green), timeout (orange)
- **Automated Grammar Generation**: Extracts patterns from actual parser code
- **Git Version Sync**: Automatically uses git tags/commits for versioning
- **Multi-Extension Support**: `.sc2`, `.script2`, `.qmp2` files

### Installation
```bash
# Build and install extension
make vscode-install

# Clean reinstall
make vscode-reinstall

# Update grammar when parser changes
make vscode-extension
```

## 🔧 Configuration

QMP Controller features a comprehensive configuration management system supporting configuration files, environment variables, and named profiles for different environments.

### Configuration Management Commands

```bash
# Generate a comprehensive configuration file
qmp config init                    # Creates ~/.qmp.yaml
qmp config init project.yaml       # Creates custom config file

# View current configuration and sources
qmp config show                    # Shows all settings and their sources

# List available profiles
qmp config profiles                 # Shows dev/staging/prod profiles

# Validate configuration
qmp config validate                # Validates current config
qmp config validate config.yaml    # Validates specific file

# Show configuration search paths
qmp config path                    # Shows where configs are loaded from
```

### Configuration Profiles

Profiles allow you to define named collections of settings for different environments:

```bash
# Use a profile (applies all profile settings)
qmp --profile dev status           # Uses dev VM and settings
qmp --profile prod ocr vm          # Uses production configuration
qmp --profile test usb list        # Uses test environment settings

# Environment variables still override profiles
QMP_VM_ID=999 qmp --profile dev status  # Uses VM 999 instead of profile's VM
```

### Configuration Files

Configuration files are searched in priority order:
1. `~/.qmp.yaml` (user configuration)
2. `./.qmp.yaml` (project configuration)
3. `/etc/qmp/.qmp.yaml` (system configuration)

Sample configuration with profiles:
```yaml
# Core settings
log_level: info
vm_id: 106

# OCR defaults
columns: 160
rows: 50
training_data: ~/training.json

# Profiles for different environments
profiles:
  dev:
    vm_id: 106
    socket: /var/run/qemu-server/106.qmp
    training_data: ~/dev-training.json

  prod:
    vm_id: 200
    socket: /tmp/prod-qmp-tunnel
    screenshot_format: ppm

  test:
    vm_id: 999
    columns: 80
    rows: 25
    ansi_mode: true
    color_mode: true
```

### Environment Variables (23 Variables Supported)

QMP Controller supports comprehensive environment variable configuration with the `QMP_` prefix. All settings can be overridden by environment variables:

#### Core Configuration
- `QMP_LOG_LEVEL`: Logging level (debug, info, warn, error) - default: info
- `QMP_SOCKET`: Custom socket path for SSH tunneling
- `QMP_VM_ID`: Default VM ID for operations

#### OCR Configuration
- `QMP_COLUMNS`: Screen width in characters - default: 160
- `QMP_ROWS`: Screen height in characters - default: 50
- `QMP_TRAINING_DATA`: Path to OCR training data file
- `QMP_IMAGE_FILE`: Default image file for OCR operations
- `QMP_OUTPUT_FILE`: Default output file for results

#### Timing & Format
- `QMP_KEY_DELAY`: Keyboard input delay (e.g., 50ms, 100ms) - default: 50ms
- `QMP_SCRIPT_DELAY`: Script execution delay between commands - default: 50ms
- `QMP_SCREENSHOT_FORMAT`: Default screenshot format (ppm, png) - default: png
- `QMP_COMMENT_CHAR`: Script comment character - default: #
- `QMP_CONTROL_CHAR`: Script control command prefix - default: <#

#### OCR Processing Options
- `QMP_ANSI_MODE`: Enable ANSI bitmap output (true/false)
- `QMP_COLOR_MODE`: Enable color output (true/false)
- `QMP_FILTER_BLANK_LINES`: Filter blank lines from output (true/false)
- `QMP_SHOW_LINE_NUMBERS`: Show line numbers with output (true/false)
- `QMP_RENDER_SPECIAL_CHARS`: Render special characters visually (true/false)

#### Advanced OCR Options
- `QMP_SINGLE_CHAR`: Single character extraction mode (true/false)
- `QMP_CHAR_ROW`: Row for single character extraction (0-based)
- `QMP_CHAR_COL`: Column for single character extraction (0-based)
- `QMP_CROP_ROWS`: Crop rows range (format: "start:end")
- `QMP_CROP_COLS`: Crop columns range (format: "start:end")

**View all variables**: Use `qmp env` to see current values, sources, and descriptions.

### Configuration Priority

Settings are resolved in this priority order (highest to lowest):
1. **Command-line flags** (highest priority)
2. **Environment variables** (`QMP_*`)
3. **Profile settings** (`--profile name`)
4. **Configuration file values**
5. **Built-in defaults** (lowest priority)

### Socket Setup
```bash
# Simple TCP forwarding (recommended)
make socket-simple

# Direct socket forwarding
make socket-setup

# Test connections
make socket-test
```

## 🧪 Development

### Building
```bash
# Development build
go build -o qmp-controller

# Production builds
make build              # All platforms
make build-with-vscode  # Include VSCode extension
```

### Testing
```bash
# Socket connectivity
make socket-test

# OCR functionality
qmp train_ocr 106 test-training.json
qmp ocr re "test" test-training.json vm 106

# Script validation
qmp script2 106 test.sc2 --dry-run
```

### Code Quality
- **Structured Logging**: All operations use contextual loggers
- **Resource Management**: Automatic cleanup and connection pooling
- **Error Handling**: Graceful degradation and user-friendly messages
- **Performance Monitoring**: Built-in timing and metrics collection

## 📋 Common Commands

### Configuration Management (✅ Complete System)
```bash
# Configuration setup
qmp config init                                # Generate comprehensive config
qmp config profiles                            # List available profiles
qmp config show                               # View current configuration

# Environment variables (✅ 23 Variables Supported)
qmp env                                       # View all environment variables

# Set common defaults
export QMP_VM_ID=106                          # Default VM for all operations
export QMP_COLUMNS=160                        # Screen width
export QMP_ROWS=50                            # Screen height
export QMP_TRAINING_DATA=~/my-training.json   # OCR training data
export QMP_OUTPUT_FILE=output.txt             # Default output file
export QMP_LOG_LEVEL=debug                    # Logging level
```

### Using Profiles and Environment Variables
```bash
# Profile-based workflows
qmp --profile dev status                      # Use dev environment settings
qmp --profile prod ocr vm                     # Use production configuration
qmp --profile test keyboard live              # Use test environment

# All commands support QMP_VM_ID and environment variables:
qmp status                                    # Uses QMP_VM_ID if set
qmp screenshot                                # Uses QMP_VM_ID + QMP_OUTPUT_FILE
qmp ocr vm                                    # Uses env vars for flexible operation
qmp usb list                                  # Uses QMP_VM_ID if set
qmp keyboard live                             # Uses QMP_VM_ID if set
qmp script automation.txt                     # Uses QMP_VM_ID + QMP_TRAINING_DATA
qmp script2 automation.sc2                   # Uses QMP_VM_ID + QMP_TRAINING_DATA
```

### Flexible Argument Ordering (✅ All Commands)
```bash
# All these commands are equivalent (smart file type detection):
qmp ocr vm 106 training.json output.txt
qmp ocr vm training.json 106 output.txt
qmp ocr vm output.txt training.json 106

# Script commands with flexible ordering:
qmp script2 106 automation.sc2 training.json
qmp script2 automation.sc2 106 training.json
qmp script2 training.json automation.sc2 106

# With environment variables (even more flexible):
export QMP_VM_ID=106
export QMP_TRAINING_DATA=training.json
qmp ocr vm output.txt                         # VM ID and training data from env
qmp script2 automation.sc2                   # All parameters from env vars
```

### Build Targets
- `make build` - Build all platform binaries
- `make build-amd` - AMD64 Linux binary
- `make build-arm` - ARM64 Linux binary
- `make build-mac-arm` - ARM64 macOS binary

### Socket Management
- `make socket-simple` - Set up TCP-based QMP forwarding
- `make socket-test` - Test socket connections
- `make socket-cleanup` - Clean up socket forwards

### VSCode Extension
- `make vscode-extension` - Generate and build extension
- `make vscode-install` - Install extension in VSCode
- `make vscode-reinstall` - Clean reinstall extension

### Deployment
- `make scp` - Deploy to remote servers
- `make clean` - Clean build artifacts

## 🔍 Examples

### Complete Automation Script
```bash
#!/usr/bin/env qmp script2
# automation.sc2 - Complete VM automation example

# Configuration
VM_ID=106
USER=${USER:-admin}
PASSWORD=${PASSWORD:-secure123}
TARGET=${TARGET:-production.server.com}

# Take initial screenshot
<screenshot "start-{timestamp}.png">

# SSH connection with retry logic
<retry 3>
ssh $USER@$TARGET
<watch "password:" 10s>
$PASSWORD
<enter>

# Wait for shell and capture state
<watch "$ " 15s>
<screenshot "connected-{timestamp}.png">

# System checks with conditional logic
<if-found "$ " 5s>
echo "=== System Status Check ==="
uptime
df -h
<else>
echo "Shell not available, exiting"
<exit 1>

# Function for service checks
<function check_service>
systemctl status $1
<if-found "active (running)" 5s>
echo "✅ $1 is running"
<else>
echo "❌ $1 is not running"
<screenshot "service-error-$1-{timestamp}.png">

# Check critical services
<call check_service nginx>
<call check_service postgresql>
<call check_service redis>

# Final status
echo "=== Automation Complete ==="
<screenshot "final-{timestamp}.png">
<exit 0>
```

### OCR Training Workflow
```bash
# 1. Generate training data from VM console
qmp train_ocr 106 my-training.json

# 2. Test recognition accuracy
qmp ocr find "login:" my-training.json vm 106 --debug

# 3. Use in automation scripts
qmp script2 106 login-automation.sc2 my-training.json
```

## 📚 Advanced Features

### Resource Management
- **Connection Pooling**: Reuse QMP connections across operations
- **Automatic Cleanup**: Temporary files and connections cleaned on exit
- **Signal Handling**: Graceful shutdown on SIGINT/SIGTERM
- **Context Cancellation**: Proper timeout and cancellation support

### Performance Monitoring
- **Structured Logging**: Operation timing and metrics
- **Debug Mode**: Detailed execution traces
- **Resource Tracking**: Memory and connection usage
- **Error Analytics**: Comprehensive error reporting

### Security Considerations
- **No Credential Storage**: Variables support environment/file loading
- **Socket Security**: SSH-based secure tunneling
- **Audit Trail**: Complete operation logging
- **Privilege Separation**: Minimal required permissions

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make changes and test thoroughly
4. Update documentation and examples
5. Submit a pull request

### Development Guidelines
- Use structured logging (`internal/logging`)
- Follow Go conventions and `gofmt`
- Add tests for new functionality
- Update CLAUDE.md for AI context
- Regenerate VSCode extension if parser changes

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **QEMU Team**: For the robust QMP protocol
- **Go Community**: For excellent tooling and libraries
- **TextMate**: For the grammar specification used in VSCode extension
- **Lipgloss**: For beautiful terminal UI styling

---

**Built with ❤️ for VM automation and management**
