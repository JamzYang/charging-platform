#!/bin/bash

# WebSocket功能测试脚本
# 用于快速验证WebSocket功能是否正常工作

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

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 切换到项目目录
cd "$PROJECT_DIR"

log_info "开始WebSocket功能测试..."

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        log_error "Go未安装"
        exit 1
    fi
    
    # 检查Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker未安装"
        exit 1
    fi
    
    # 检查Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose未安装"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 启动测试环境
start_test_environment() {
    log_info "启动测试环境..."
    
    # 启动Redis和Kafka
    docker-compose -f test/docker-compose.test.yml up -d redis-test kafka-test zookeeper-test
    
    # 等待服务启动
    log_info "等待服务启动..."
    sleep 15
    
    # 检查服务状态
    if ! docker-compose -f test/docker-compose.test.yml ps | grep -q "healthy"; then
        log_warning "服务可能还未完全启动，继续等待..."
        sleep 10
    fi
    
    log_success "测试环境启动完成"
}

# 编译网关
build_gateway() {
    log_info "编译网关..."
    
    go build -o bin/gateway ./cmd/gateway/
    
    if [ $? -eq 0 ]; then
        log_success "网关编译成功"
    else
        log_error "网关编译失败"
        exit 1
    fi
}

# 启动网关
start_gateway() {
    log_info "启动网关..."
    
    # 设置环境变量
    export REDIS_ADDR="localhost:6380"
    export KAFKA_BROKERS="localhost:9093"
    export LOG_LEVEL="debug"
    
    # 启动网关（后台运行）
    ./bin/gateway > gateway.log 2>&1 &
    GATEWAY_PID=$!
    
    # 等待网关启动
    sleep 5
    
    # 检查网关是否运行
    if kill -0 $GATEWAY_PID 2>/dev/null; then
        log_success "网关启动成功 (PID: $GATEWAY_PID)"
        echo $GATEWAY_PID > gateway.pid
    else
        log_error "网关启动失败"
        cat gateway.log
        exit 1
    fi
}

# 测试WebSocket连接
test_websocket_connection() {
    log_info "测试WebSocket连接..."
    
    # 使用wscat测试（如果可用）
    if command -v wscat &> /dev/null; then
        log_info "使用wscat测试WebSocket连接..."
        
        # 创建测试消息
        TEST_MESSAGE='[2,"test-001","Heartbeat",{}]'
        
        # 测试连接
        echo "$TEST_MESSAGE" | timeout 5 wscat -c ws://localhost:8080/ocpp/CP-TEST-001 || true
        
        log_success "WebSocket连接测试完成"
    else
        log_warning "wscat未安装，跳过WebSocket连接测试"
        log_info "可以手动测试: wscat -c ws://localhost:8080/ocpp/CP-TEST-001"
    fi
}

# 运行集成测试
run_integration_tests() {
    log_info "运行WebSocket集成测试..."
    
    # 设置测试环境变量
    export GATEWAY_URL="ws://localhost:8080/ocpp"
    
    # 运行WebSocket集成测试
    go test -v ./test/integration/websocket_integration_test.go -timeout 30s
    
    if [ $? -eq 0 ]; then
        log_success "WebSocket集成测试通过"
    else
        log_error "WebSocket集成测试失败"
        return 1
    fi
}

# 检查网关日志
check_gateway_logs() {
    log_info "检查网关日志..."
    
    if [ -f "gateway.log" ]; then
        echo "=== 网关日志 ==="
        tail -20 gateway.log
        echo "==============="
    fi
}

# 清理环境
cleanup() {
    log_info "清理环境..."
    
    # 停止网关
    if [ -f "gateway.pid" ]; then
        GATEWAY_PID=$(cat gateway.pid)
        if kill -0 $GATEWAY_PID 2>/dev/null; then
            log_info "停止网关 (PID: $GATEWAY_PID)"
            kill $GATEWAY_PID
            sleep 2
        fi
        rm -f gateway.pid
    fi
    
    # 停止Docker服务
    docker-compose -f test/docker-compose.test.yml down -v
    
    # 清理文件
    rm -f bin/gateway gateway.log
    
    log_success "环境清理完成"
}

# 主函数
main() {
    # 设置清理陷阱
    trap cleanup EXIT INT TERM
    
    log_info "WebSocket功能测试开始"
    
    # 执行测试步骤
    check_dependencies
    start_test_environment
    build_gateway
    start_gateway
    
    # 等待网关完全启动
    sleep 3
    
    # 运行测试
    test_websocket_connection
    
    # 检查日志
    check_gateway_logs
    
    # 运行集成测试
    if run_integration_tests; then
        log_success "所有WebSocket测试通过！"
        
        log_info "WebSocket服务信息:"
        echo "  WebSocket URL: ws://localhost:8080/ocpp/{charge_point_id}"
        echo "  健康检查: http://localhost:8080/health"
        echo "  连接状态: http://localhost:8080/connections"
        echo ""
        echo "测试命令示例:"
        echo "  wscat -c ws://localhost:8080/ocpp/CP-001"
        echo "  curl http://localhost:8080/health"
        echo "  curl http://localhost:8080/connections"
        
    else
        log_error "WebSocket测试失败"
        check_gateway_logs
        exit 1
    fi
}

# 显示帮助信息
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    cat << EOF
WebSocket功能测试脚本

用法: $0 [选项]

选项:
    -h, --help    显示此帮助信息

此脚本将:
1. 启动测试环境 (Redis, Kafka)
2. 编译并启动网关
3. 测试WebSocket连接
4. 运行集成测试
5. 清理环境

测试完成后，可以使用以下命令手动测试:
- wscat -c ws://localhost:8080/ocpp/CP-001
- curl http://localhost:8080/health
- curl http://localhost:8080/connections

EOF
    exit 0
fi

# 执行主函数
main
