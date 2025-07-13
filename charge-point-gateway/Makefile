# Charge Point Gateway Makefile

# 变量定义
APP_NAME := charge-point-gateway
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')

# 构建标志
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)"

# 默认目标
.PHONY: all
all: clean check test build

# 清理
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf bin/ dist/ coverage.out

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# 快速代码检查 (开发阶段)
.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# 全面代码检查 (提交前)
.PHONY: lint
lint:
	@echo "Running comprehensive linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, running go vet instead..."; \
		go vet ./...; \
	fi

# 基础检查 (快速)
.PHONY: check
check: vet fmt
	@echo "Basic checks completed"

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

# 测试覆盖率
.PHONY: coverage
coverage: test
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 构建
.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(APP_NAME) cmd/gateway/main.go

# 交叉编译
.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 cmd/gateway/main.go

# 运行
.PHONY: run
run:
	@echo "Running $(APP_NAME)..."
	@go run cmd/gateway/main.go

# 开发模式运行
.PHONY: dev
dev:
	@echo "Running in development mode..."
	@air -c .air.toml

# Docker构建
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) .
	@docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

# Docker运行
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	@docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-p 9090:9090 \
		-v $(PWD)/configs:/app/configs \
		$(APP_NAME):latest

# 生成模拟代码
.PHONY: generate
generate:
	@echo "Generating mocks..."
	@go generate ./...

# 安装开发工具
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/cosmtrek/air@latest

# 初始化项目
.PHONY: init
init: install-tools deps
	@echo "Project initialized successfully!"

# 帮助
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, lint, test and build"
	@echo "  clean        - Remove build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  lint         - Run code linter"
	@echo "  fmt          - Format code"
	@echo "  test         - Run tests"
	@echo "  coverage     - Generate test coverage report"
	@echo "  build        - Build binary"
	@echo "  build-linux  - Cross compile for Linux"
	@echo "  run          - Run application"
	@echo "  dev          - Run in development mode with hot reload"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  generate     - Generate mocks and other code"
	@echo "  install-tools- Install development tools"
	@echo "  init         - Initialize project"
	@echo "  help         - Show this help"
