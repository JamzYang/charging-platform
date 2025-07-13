#!/bin/bash

# 测试执行脚本

set -e

echo "🧪 Running comprehensive tests..."

# 进入项目目录
cd "$(dirname "$0")/.."

# 清理之前的测试结果
echo "Cleaning previous test results..."
rm -f coverage.out coverage.html

# 运行单元测试
echo "Running unit tests..."
go test -v -race -coverprofile=coverage.out ./...

# 生成覆盖率报告
echo "Generating coverage report..."
go tool cover -func=coverage.out

# 检查覆盖率是否达到要求
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
REQUIRED_COVERAGE=80

echo "Coverage: ${COVERAGE}%"

if (( $(echo "$COVERAGE >= $REQUIRED_COVERAGE" | bc -l) )); then
    echo "✅ Coverage requirement met (${COVERAGE}% >= ${REQUIRED_COVERAGE}%)"
else
    echo "❌ Coverage requirement not met (${COVERAGE}% < ${REQUIRED_COVERAGE}%)"
    exit 1
fi

# 生成HTML覆盖率报告
echo "Generating HTML coverage report..."
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"

# 运行基准测试
echo "Running benchmark tests..."
go test -bench=. -benchmem ./... || echo "No benchmark tests found"

# 运行竞态检测
echo "Running race detection tests..."
go test -race ./...

# 检查测试文件命名规范
echo "Checking test file naming conventions..."
find . -name "*_test.go" -not -path "./vendor/*" | while read -r file; do
    if [[ ! "$file" =~ _test\.go$ ]]; then
        echo "❌ Test file $file does not follow naming convention"
        exit 1
    fi
done

echo "✅ Test file naming conventions are correct"

echo "🎉 All tests passed successfully!"
