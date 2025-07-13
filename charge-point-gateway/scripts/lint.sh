#!/bin/bash

# 代码检查脚本

set -e

echo "🔍 Running code quality checks..."

# 进入项目目录
cd "$(dirname "$0")/.."

# 检查 golangci-lint 是否安装
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

# 运行代码检查
echo "Running golangci-lint..."
golangci-lint run ./...

# 检查代码格式
echo "Checking code format..."
if [ "$(gofmt -l . | wc -l)" -gt 0 ]; then
    echo "❌ Code is not formatted. Run 'make fmt' to fix."
    gofmt -l .
    exit 1
else
    echo "✅ Code is properly formatted"
fi

# 检查导入排序
echo "Checking import order..."
if ! command -v goimports &> /dev/null; then
    echo "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi

if [ "$(goimports -l . | wc -l)" -gt 0 ]; then
    echo "❌ Imports are not properly sorted. Run 'make fmt' to fix."
    goimports -l .
    exit 1
else
    echo "✅ Imports are properly sorted"
fi

# 检查模块整洁性
echo "Checking module tidiness..."
go mod tidy
if [ "$(git diff --name-only | grep -E '(go.mod|go.sum)' | wc -l)" -gt 0 ]; then
    echo "❌ go.mod or go.sum is not tidy. Run 'go mod tidy'."
    exit 1
else
    echo "✅ Modules are tidy"
fi

# 检查安全问题
echo "Running security checks..."
if ! command -v gosec &> /dev/null; then
    echo "Installing gosec..."
    go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
fi

gosec -quiet ./...

echo "🎉 All code quality checks passed!"
