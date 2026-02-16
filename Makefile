# GoAgent Linux Automation
# -----------------------------------------------------------------------------
BIN_DIR = ./bin
GW_BIN     = $(BIN_DIR)/goagent-gw
CLI_BIN    = $(BIN_DIR)/goagent-cli
REPORT_BIN = $(BIN_DIR)/goagent-report
TOKEN   = "supersecret123" # Replace with your GOAGENT_TOKEN env var

.PHONY: all build run-gw test test-integration clean update-go

all: build

# 1. Update Go Toolchain (Linux x64)
# Use this if you see 'toolchain not available' errors.
update-go:
	@echo "🌐 Downloading Go 1.26.0 for Linux..."
	@wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
	@sudo rm -rf /usr/local/go
	@sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
	@rm go1.26.0.linux-amd64.tar.gz
	@echo "✅ Go 1.26 installed. Please restart your terminal."

# 2. Build binaries
build:
	@echo "🔨 Building GoAgent binaries..."
	@mkdir -p ./bin
	# GOTOOLCHAIN=local tells Go to use the version we just installed, not try to download 1.23
	GOTOOLCHAIN=local go build -o ./bin/goagent-gw ./cmd/gateway
	GOTOOLCHAIN=local go build -o ./bin/goagent-cli ./cmd/cli
	GOTOOLCHAIN=local go build -o ./bin/goagent-report ./cmd/report
	@echo "✅ Binaries ready in ./bin"

# 3. Run tests (mock only — no API key needed)
test:
	@echo "🧪 Running tests..."
	GOTOOLCHAIN=local go test ./... -run 'Test[^I]' -v
	@echo "✅ Tests passed"

# 4. Run tests including real LLM integration (needs .env with LLM_API_KEY)
test-integration:
	@echo "🧪 Running all tests (mock + integration)..."
	@set -a && . ./.env && set +a && GOTOOLCHAIN=local go test ./... -v
	@echo "✅ All tests passed"

# 5. Run Gateway
run-gw: build
	@echo "🚀 Starting Gateway..."
	@$(GW_BIN) --token=$(TOKEN) --data="~/.goagent"

# 6. Cleanup
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf $(BIN_DIR)
	@rm -rf ~/.goagent/*.tmp
