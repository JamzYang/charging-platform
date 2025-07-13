#!/bin/bash

# 配置模块测试脚本

set -e

echo "🧪 Testing configuration module..."

# 进入项目目录
cd "$(dirname "$0")/.."

# 运行配置模块测试
echo "Running config package tests..."
go test -v -race -coverprofile=coverage-config.out ./internal/config/

# 生成覆盖率报告
echo "Generating coverage report..."
go tool cover -func=coverage-config.out

# 检查覆盖率是否达到要求 (80%)
COVERAGE=$(go tool cover -func=coverage-config.out | grep total | awk '{print $3}' | sed 's/%//')
REQUIRED_COVERAGE=80

echo "Coverage: ${COVERAGE}%"

if (( $(echo "$COVERAGE >= $REQUIRED_COVERAGE" | bc -l) )); then
    echo "✅ Coverage requirement met (${COVERAGE}% >= ${REQUIRED_COVERAGE}%)"
else
    echo "❌ Coverage requirement not met (${COVERAGE}% < ${REQUIRED_COVERAGE}%)"
    exit 1
fi

# 测试配置文件加载
echo "Testing config file loading..."
if go run cmd/gateway/main.go --help > /dev/null 2>&1; then
    echo "✅ Application starts successfully"
else
    echo "❌ Application failed to start"
    exit 1
fi

echo "🎉 All configuration tests passed!"
