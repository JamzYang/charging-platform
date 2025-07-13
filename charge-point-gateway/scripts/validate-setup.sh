#!/bin/bash

# 验证开发环境设置脚本

set -e

echo "🔧 Validating development environment setup..."

# 进入项目目录
cd "$(dirname "$0")/.."

# 检查Go版本
echo "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Go version: $GO_VERSION"

# 检查项目结构
echo "Checking project structure..."
REQUIRED_DIRS=("cmd/gateway" "internal/config" "configs" "scripts")
for dir in "${REQUIRED_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        echo "✅ Directory $dir exists"
    else
        echo "❌ Directory $dir missing"
        exit 1
    fi
done

# 检查必要文件
echo "Checking required files..."
REQUIRED_FILES=("go.mod" ".golangci.yml" ".editorconfig" "Makefile" "README.md")
for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ File $file exists"
    else
        echo "❌ File $file missing"
        exit 1
    fi
done

# 检查依赖
echo "Checking dependencies..."
if go mod verify; then
    echo "✅ Dependencies verified"
else
    echo "❌ Dependencies verification failed"
    exit 1
fi

# 检查代码编译
echo "Checking code compilation..."
if go build ./...; then
    echo "✅ Code compiles successfully"
else
    echo "❌ Code compilation failed"
    exit 1
fi

# 运行测试
echo "Running tests..."
if go test ./...; then
    echo "✅ All tests pass"
else
    echo "❌ Tests failed"
    exit 1
fi

# 检查代码格式
echo "Checking code format..."
if [ "$(gofmt -l . | wc -l)" -eq 0 ]; then
    echo "✅ Code is properly formatted"
else
    echo "❌ Code formatting issues found:"
    gofmt -l .
    exit 1
fi

echo "🎉 Development environment setup validation completed successfully!"
echo ""
echo "Next steps:"
echo "1. Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
echo "2. Run linter: golangci-lint run ./..."
echo "3. Start development: make dev"
