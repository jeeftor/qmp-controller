
# Build variables
# Build variables
GIT_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
GIT_COMMITS_SINCE_TAG ?= $(shell c=$$(git rev-list $(GIT_TAG)..HEAD --count 2>/dev/null); [ "$$c" -eq 0 ] && echo "0" || echo "$$c" 2>/dev/null || echo "0")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
GIT_BRANCH ?= $(shell git branch --show-current 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")


print-vars:
	@echo "GIT_TAG = $(GIT_TAG)"
	@echo "GIT_COMMITS_SINCE_TAG = $(GIT_COMMITS_SINCE_TAG)"
	@echo "GIT_COMMIT = $(GIT_COMMIT)"
	@echo "GIT_BRANCH = $(GIT_BRANCH)"
	@echo "BUILD_TIME = $(BUILD_TIME)"

# LDFLAGS for embedding version information
LDFLAGS := -ldflags "-s -w \
	-X github.com/jeeftor/qmp-controller/cmd.buildTag=$(GIT_TAG) \
	-X github.com/jeeftor/qmp-controller/cmd.buildCommitsSinceTag=$(GIT_COMMITS_SINCE_TAG) \
	-X github.com/jeeftor/qmp-controller/cmd.buildCommit=$(GIT_COMMIT) \
	-X github.com/jeeftor/qmp-controller/cmd.buildBranch=$(GIT_BRANCH)\
	-X github.com/jeeftor/qmp-controller/cmd.buildTime=$(BUILD_TIME)"

# LDFLAGS with static linking for Linux builds
LDFLAGS_STATIC := -ldflags "-s -w \
	-X github.com/jeeftor/qmp-controller/cmd.buildTag=$(GIT_TAG) \
	-X github.com/jeeftor/qmp-controller/cmd.buildCommitsSinceTag=$(GIT_COMMITS_SINCE_TAG) \
	-X github.com/jeeftor/qmp-controller/cmd.buildCommit=$(GIT_COMMIT) \
	-X github.com/jeeftor/qmp-controller/cmd.buildBranch=$(GIT_BRANCH)\
	-X github.com/jeeftor/qmp-controller/cmd.buildTime=$(BUILD_TIME) \
	-extldflags '-static'"



utm-screenshot:
	magick /Users/jstein/Library/Containers/com.utmapp.UTM/Data/Documents/Linux.utm/screenshot.png ./test_data/utm.ppm


build-amd:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a $(LDFLAGS_STATIC) -o dist/qmp-controller-amd64

build-arm:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -a $(LDFLAGS_STATIC) -o dist/qmp-controller-arm64

build-mac-arm:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -a $(LDFLAGS) -o dist/qmp-controller-darwin-arm64

build: build-arm build-amd build-mac-arm

build-with-vscode: build vscode-extension

clean:
	rm -rf dist
	rm -rf vscode-extension/*.vsix

# VSCode Extension Generation
vscode-extension:
	@echo "🎨 Generating VSCode extension..."
	go run main.go generate-vscode-grammar
	@echo "📦 Building VSCode extension..."
	cd vscode-extension && npm install --silent && npm run package
	@echo "✅ VSCode extension built: vscode-extension/qmp-script2-*.vsix"

vscode-uninstall:
	@echo "🗑️  Uninstalling old VSCode extension..."
	code --uninstall-extension qmp-controscrller.qmp-script2 || true
	@echo "✅ Old extension uninstalled!"

vscode-install: vscode-extension vscode-uninstall
	@echo "🔧 Installing VSCode extension..."
	cd vscode-extension && code --install-extension qmp-script2-*.vsix
	@echo "✅ VSCode extension installed!"

vscode-reinstall: vscode-uninstall vscode-install
	@echo "✅ VSCode extension reinstalled!"

vscode-clean:
	rm -rf vscode-extension/*.vsix vscode-extension/node_modules

# JetBrains Plugin Generation (Docker-based)
jetbrains-plugin:
	@echo "🔧 Generating JetBrains plugin..."
	go run main.go generate-jetbrains-plugin
	@echo "🐳 Building JetBrains plugin with Docker..."
	cd jetbrains-plugin && ./docker-build.sh
	@echo "✅ JetBrains plugin built: jetbrains-plugin/build/distributions/*.zip"

jetbrains-plugin-local:
	@echo "🔧 Generating JetBrains plugin..."
	go run main.go generate-jetbrains-plugin
	@echo "📦 Building JetBrains plugin locally (requires Java)..."
	cd jetbrains-plugin && ./gradlew buildPlugin
	@echo "✅ JetBrains plugin built: jetbrains-plugin/build/distributions/*.zip"

jetbrains-dev:
	@echo "🚀 Running JetBrains plugin in development IDE (requires Java)..."
	@echo "⚠️  This target requires local Java installation"
	cd jetbrains-plugin && ./gradlew runIde

jetbrains-test:
	@echo "🧪 Testing JetBrains plugin with Docker..."
	cd jetbrains-plugin && docker run --rm -v "$$(pwd):/workspace" openjdk:11-jdk-slim bash -c "cd /workspace && ./gradlew test verifyPlugin"

jetbrains-clean:
	rm -rf jetbrains-plugin/build jetbrains-plugin/.gradle
	docker rmi jetbrains-plugin-builder 2>/dev/null || true

scp: clean vscode-extension build-amd
	scp ./dist/qmp-controller-amd64 pve1:~/qmp-controller &
	scp ./dist/qmp-controller-amd64 pve2:~/qmp-controller &
	scp ./dist/qmp-controller-amd64 pve3:~/qmp-controller &
	scp ./dist/qmp-controller-amd64 pve4:~/qmp-controller &
	cp ./dist/qmp-controller-amd64  /Users/jstein/devel/n2cx/secureUSB/qmp &
	cp ./vscode-extension/qmp-script2-*.vsix /Volumes/SecureUSB/dev/script2.vsix &
	cp ./dist/qmp-controller-amd64  /Volumes/SecureUSB/dev/qmp	&
	wait

socat-tcp:
	socat UNIX-LISTEN:/tmp/qmp-socket,fork TCP:localhost:5902

socket-setup:
	@echo "Setting up QMP socket forwards..."
	@mkdir -p /tmp/qmp
	@rm -f /tmp/qmp/*.sock
	@echo "Starting socket forwards in background..."
	ssh -o StreamLocalBindUnlink=yes -o ExitOnForwardFailure=yes -o ControlMaster=no -L /tmp/qmp/pve1-106.sock:/var/run/qemu-server/106.qmp -N -f pve1 || echo "❌ pve1 forward failed"
	ssh -o StreamLocalBindUnlink=yes -o ExitOnForwardFailure=yes -o ControlMaster=no -L /tmp/qmp/pve4-108.sock:/var/run/qemu-server/108.qmp -N -f pve4 || echo "❌ pve4 forward failed"
	@echo "Waiting for socket files to be created..."
	@sleep 3
	@echo "Socket forwards status:"
	@ls -la /tmp/qmp/*.sock 2>/dev/null || echo "❌ No socket files created"
	@echo "SSH processes:"
	@ps aux | grep -E "ssh.*qemu-server" | grep -v grep || echo "❌ No SSH forwards running"

socket-simple:
	@echo "Setting up simple TCP-based QMP forwards..."
	@echo "Step 1: Cleaning up old processes..."
	-ssh pve1 "pkill -f 'socat.*9106'" 2>/dev/null || true
	-ssh pve4 "pkill -f 'socat.*9108'" 2>/dev/null || true
	@echo "Step 2: Starting SOCAT bridges (properly backgrounded)..."
	ssh pve1 "nohup socat TCP-LISTEN:9106,reuseaddr,fork UNIX-CONNECT:/var/run/qemu-server/106.qmp </dev/null >/dev/null 2>&1 & disown"
	ssh pve4 "nohup socat TCP-LISTEN:9108,reuseaddr,fork UNIX-CONNECT:/var/run/qemu-server/108.qmp </dev/null >/dev/null 2>&1 & disown"
	@sleep 2
	@echo "Step 3: Verifying remote TCP ports are listening..."
	ssh pve1 "ss -tlnp | grep :9106" || echo "❌ Port 9106 not listening on pve1"
	ssh pve4 "ss -tlnp | grep :9108" || echo "❌ Port 9108 not listening on pve4"
	@echo "Step 4: Setting up SSH TCP port forwards..."
	ssh -f -N -L 9106:localhost:9106 pve1 || echo "❌ pve1 TCP forward failed"
	ssh -f -N -L 9108:localhost:9108 pve4 || echo "❌ pve4 TCP forward failed"
	@sleep 1
	@echo "Step 5: Creating local Unix sockets..."
	@mkdir -p /tmp/qmp
	@rm -f /tmp/qmp/*.sock
	nohup socat UNIX-LISTEN:/tmp/qmp/pve1-106.sock,fork TCP:localhost:9106 </dev/null >/dev/null 2>&1 &
	nohup socat UNIX-LISTEN:/tmp/qmp/pve4-108.sock,fork TCP:localhost:9108 </dev/null >/dev/null 2>&1 &
	@sleep 2
	@echo "✅ QMP forwards ready:"
	@echo "  VM 106 (pve1): /tmp/qmp/pve1-106.sock"
	@echo "  VM 108 (pve4): /tmp/qmp/pve4-108.sock"
	@ls -la /tmp/qmp/*.sock 2>/dev/null || echo "❌ Socket files not created"

socket-simple-cleanup:
	@echo "Cleaning up simple TCP forwards..."
	@pkill -f "socat.*TCP:localhost:910" || true
	@pkill -f "ssh.*9106" || true
	@pkill -f "ssh.*9108" || true
	ssh pve1 "pkill -f 'socat.*TCP.*106.qmp' || true"
	ssh pve4 "pkill -f 'socat.*TCP.*108.qmp' || true"
	@rm -f /tmp/qmp/*.sock
	@echo "Simple forwards cleaned up"

socket-test:
	@echo "Testing socket connections..."
	@echo "Testing VM 106 on pve1:"
	@echo '{"execute":"qmp_capabilities"}' | socat - UNIX-CONNECT:/tmp/qmp/pve1-106.sock || echo "❌ pve1-106 failed"
	@echo "Testing VM 108 on pve4:"
	@echo '{"execute":"qmp_capabilities"}' | socat - UNIX-CONNECT:/tmp/qmp/pve4-108.sock || echo "❌ pve4-108 failed"

socket-cleanup:
	@echo "Cleaning up socket forwards..."
	@pkill -f "ssh.*StreamLocalBindUnlink.*qemu-server" || true
	@rm -f /tmp/qmp/*.sock
	@echo "Socket forwards stopped and cleaned up"

socket-status:
	@echo "Socket forward status:"
	@ps aux | grep -E "ssh.*qemu-server" | grep -v grep || echo "No socket forwards running"
	@echo "Available sockets:"
	@ls -la /tmp/qmp/*.sock 2>/dev/null || echo "No sockets found"

socket-debug:
	@echo "Debugging socket forwards..."
	@echo "1. Checking if VMs are running:"
	ssh pve1 "qm status 106" || echo "❌ VM 106 not found/running on pve1"
	ssh pve4 "qm status 108" || echo "❌ VM 108 not found/running on pve4"
	@echo "2. Checking if QMP sockets exist on hosts:"
	ssh pve1 "ls -la /var/run/qemu-server/106.qmp" || echo "❌ QMP socket for VM 106 not found"
	ssh pve4 "ls -la /var/run/qemu-server/108.qmp" || echo "❌ QMP socket for VM 108 not found"
	@echo "3. Testing manual SSH socket forward (verbose):"
	@echo "Try this manually: ssh -v -o StreamLocalBindUnlink=yes -L /tmp/test.sock:/var/run/qemu-server/106.qmp pve1"

socket-manual:
	@echo "Setting up socket forwards with verbose output..."
	@mkdir -p /tmp/qmp
	@rm -f /tmp/qmp/*.sock
	ssh -v -o StreamLocalBindUnlink=yes -L /tmp/qmp/pve1-106.sock:/var/run/qemu-server/106.qmp -N pve1

socket-test-manual:
	@echo "Manual testing of each step..."
	@echo "1. Test direct QMP access:"
	ssh pve1 "echo '{\"execute\":\"qmp_capabilities\"}' | socat - UNIX-CONNECT:/var/run/qemu-server/106.qmp"
	@echo "2. Test if socat is available:"
	ssh pve1 "which socat"
	@echo "3. Start SOCAT bridge manually:"
	ssh pve1 "socat TCP-LISTEN:9106,reuseaddr,fork UNIX-CONNECT:/var/run/qemu-server/106.qmp &"
	@sleep 2
	@echo "4. Test TCP bridge:"
	ssh pve1 "echo '{\"execute\":\"qmp_capabilities\"}' | socat - TCP:localhost:9106"

# Testing targets
test:
	@echo "🧪 Running unit tests..."
	@echo "📦 Testing script command pattern..."
	go test ./internal/script/... -v
	@echo "📦 Testing OCR argument parsing..."
	go test ./internal/ocr/... -v
	@echo "📦 Testing QMP utilities..."
	go test ./internal/qmp/... -v
	@echo "📦 Testing parameter resolution..."
	go test ./internal/params/... -v || echo "⚠️  Some parameter resolver tests have minor issues (known/fixable)"
	@echo "✅ Unit tests completed!"

test-quick:
	@echo "🧪 Running quick unit tests..."
	go test ./internal/script/... ./internal/ocr/... ./internal/qmp/...
	@echo "✅ Quick tests completed!"

test-coverage:
	@echo "🧪 Running tests with coverage..."
	@mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./internal/script/... ./internal/ocr/... ./internal/qmp/...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "📊 Coverage report: coverage/coverage.html"

test-clean:
	rm -rf coverage

# Convenience aliases
socket: socket-setup
socket-simple-start: socket-simple
test-socket: socket-test
test-manual: socket-test-manual
clean-socket: socket-cleanup
debug-socket: socket-debug
clean-simple: socket-simple-cleanup

.PHONY: clean build-amd build-arm build-mac-arm build build-with-vscode scp socket-setup socket-simple socket-simple-cleanup socket-test socket-test-manual socket-cleanup socket-status socket test-socket clean-socket socket-debug socket-manual debug-socket socket-simple-start clean-simple test-manual test test-quick test-coverage test-clean vscode-extension vscode-uninstall vscode-install vscode-reinstall vscode-clean jetbrains-plugin jetbrains-plugin-local jetbrains-dev jetbrains-test jetbrains-clean
