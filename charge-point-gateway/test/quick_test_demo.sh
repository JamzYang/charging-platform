#!/bin/bash

# 充电桩网关测试演示脚本
# 用于快速验证不同层次的测试

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "🚀 充电桩网关测试演示"
echo "====================================="
echo ""

log_info "📋 测试层次结构:"
echo "├── 单元测试 (Unit Tests) - 仅需Go环境"
echo "├── 集成测试 (Integration Tests) - 使用TestContainers"
echo "└── E2E测试 (End-to-End Tests) - 使用Docker Compose"
echo ""

log_info "🎯 1. 运行单元测试 (快速验证)"
echo "====================================="
log_info "运行OCPP消息创建测试..."
if go test -v ./test -run "TestOCPPMessageCreation" -timeout 10s; then
    log_success "OCPP消息测试通过"
else
    log_error "OCPP消息测试失败"
    exit 1
fi
echo ""

log_info "🎯 2. 运行工具函数测试"
echo "====================================="
log_info "运行断言和数据加载测试..."
if go test -v ./test -run "TestLoadTestData|TestAssertionHelpers" -timeout 10s; then
    log_success "工具函数测试通过"
else
    log_error "工具函数测试失败"
    exit 1
fi
echo ""

log_info "🎯 3. 运行WebSocket客户端测试"
echo "====================================="
log_info "运行WebSocket客户端创建测试..."
if go test -v ./test -run "TestWebSocketClientCreation" -timeout 10s; then
    log_success "WebSocket客户端测试通过"
else
    log_error "WebSocket客户端测试失败"
    exit 1
fi
echo ""

log_info "🎯 4. 验证修复工具"
echo "====================================="
log_info "编译并运行verify_fixes..."
if go test -c -o test_verify_fixes github.com/charging-platform/charge-point-gateway/test; then
    log_success "verify_fixes编译成功"
    log_info "运行verify_fixes (注意: 可能会有连接失败，这是正常的)"
    # 运行但不检查退出码，因为连接失败是预期的
    ./test_verify_fixes || true
    rm -f test_verify_fixes
else
    log_error "verify_fixes编译失败"
fi
echo ""

log_info "🎯 5. 检查Docker环境状态"
echo "====================================="
log_info "检查Docker Compose服务状态..."
if command -v docker-compose &> /dev/null; then
    docker-compose -f test/docker-compose.test.yml ps || log_warning "Docker Compose环境未启动"
else
    log_warning "Docker Compose未安装"
fi
echo ""

log_info "📊 测试总结"
echo "====================================="
log_success "✅ 单元测试: 全部通过 (无需Docker)"
log_info "🔧 集成测试: 使用TestContainers (自动管理容器)"
log_info "🐳 E2E测试: 使用Docker Compose环境 (需要预启动)"
echo ""
log_info "💡 下一步:"
echo "1. 运行集成测试: go test -v ./test/integration/... -timeout 120s"
echo "2. 启动完整环境: docker-compose -f test/docker-compose.test.yml up -d"
echo "3. 运行E2E测试: go test -v ./test/e2e/... -timeout 300s"
echo ""

log_success "🎉 测试演示完成!"
