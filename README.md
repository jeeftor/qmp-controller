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
- **Interactive Live Mode**: Real-time VM interaction with command history and status display
- **USB Device Management**: Dynamic USB device attachment and control

### Developer Tools
- **VSCode Extension**: Syntax highlighting for script2 files with automated grammar generation
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

### Basic VM Control
```bash
# Check VM status
qmp status 106

# Take screenshot
qmp screenshot 106 output.png

# Interactive keyboard mode
qmp keyboard live 106

# USB device management
qmp usb add 106 /dev/sdb
```

### OCR and Screen Automation
```bash
# Generate OCR training data
qmp train_ocr 106 training.json

# Search for text on screen
qmp ocr find "login:" training.json vm 106

# Regex search
qmp ocr re "\\berror\\b" training.json vm 106 --ignore-case
```

### Script Automation
```bash
# Execute enhanced script with variables
qmp script2 106 automation.sc2 training.json --var USER=admin

# Sample script generation
qmp script2 sample > my-script.sc2
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

### Socket Setup
```bash
# Simple TCP forwarding (recommended)
make socket-simple

# Direct socket forwarding
make socket-setup

# Test connections
make socket-test
```

### Environment Variables
- `QMP_DEBUG=1`: Enable debug logging
- `QMP_TRAINING_DATA`: Default OCR training data path
- `QMP_SOCKET_TIMEOUT`: Connection timeout (default: 30s)

### Configuration Files
- `.qmp.yaml`: Main configuration
- `~/.qmp_training_data.json`: OCR training data (persistent)

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

## 📋 Makefile Targets

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
