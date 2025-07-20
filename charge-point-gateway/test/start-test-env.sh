#!/bin/bash
# 启动测试环境脚本 (Bash)
# 支持可选的监控服务

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 参数解析
WITH_MONITORING=false
STOP=false
RESTART=false
STATUS=false
LOGS=false
BUILD=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --with-monitoring)
            WITH_MONITORING=true
            shift
            ;;
        --stop)
            STOP=true
            shift
            ;;
        --restart)
            RESTART=true
            shift
            ;;
        --status)
            STATUS=true
            shift
            ;;
        --logs)
            LOGS=true
            shift
            ;;
        --build)
            BUILD=true
            shift
            ;;
        -h|--help)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --with-monitoring  启动监控服务"
            echo "  --stop            停止环境"
            echo "  --restart         重启环境"
            echo "  --status          查看状态"
            echo "  --logs            查看日志"
            echo "  --build           重新构建"
            echo "  -h, --help        显示帮助"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            echo "使用 -h 或 --help 查看帮助"
            exit 1
            ;;
    esac
done

# 进入脚本所在目录
cd "$(dirname "$0")"

function log_info() {
    echo -e "${CYAN}ℹ️  $1${NC}"
}

function log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

function log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

function log_error() {
    echo -e "${RED}❌ $1${NC}"
}

function start_test_environment() {
    local include_monitoring=$1
    
    log_info "启动测试环境..."
    
    # 检查Docker是否运行
    if ! docker version >/dev/null 2>&1; then
        log_error "Docker未运行，请先启动Docker"
        exit 1
    fi
    
    # 构建启动命令
    local compose_cmd="docker-compose -f docker-compose.test.yml"
    
    if [ "$BUILD" = true ]; then
        log_info "重新构建镜像..."
        if [ "$include_monitoring" = true ]; then
            $compose_cmd --profile monitoring build
        else
            $compose_cmd build
        fi
    fi
    
    # 启动服务
    if [ "$include_monitoring" = true ]; then
        log_info "启动测试环境 + 监控服务..."
        $compose_cmd --profile monitoring up -d
        
        if [ $? -eq 0 ]; then
            log_success "测试环境和监控服务启动成功！"
            log_info "服务访问地址："
            echo -e "${YELLOW}  🌐 网关WebSocket:  ws://localhost:8081/ocpp/{charge_point_id}${NC}"
            echo -e "${YELLOW}  🏥 网关健康检查:   http://localhost:8081/health${NC}"
            echo -e "${YELLOW}  📊 网关Metrics:    http://localhost:9091/metrics${NC}"
            echo -e "${YELLOW}  📈 Grafana:        http://localhost:3000 (admin/admin123)${NC}"
            echo -e "${YELLOW}  🔍 Prometheus:     http://localhost:9090${NC}"
            echo -e "${YELLOW}  🚨 AlertManager:   http://localhost:9093${NC}"
            echo -e "${YELLOW}  🔴 Redis:          localhost:6379${NC}"
            echo -e "${YELLOW}  📨 Kafka:          localhost:9092${NC}"
        fi
    else
        log_info "启动测试环境（不包含监控）..."
        $compose_cmd up -d
        
        if [ $? -eq 0 ]; then
            log_success "测试环境启动成功！"
            log_info "服务访问地址："
            echo -e "${YELLOW}  🌐 网关WebSocket:  ws://localhost:8081/ocpp/{charge_point_id}${NC}"
            echo -e "${YELLOW}  🏥 网关健康检查:   http://localhost:8081/health${NC}"
            echo -e "${YELLOW}  📊 网关Metrics:    http://localhost:9091/metrics${NC}"
            echo -e "${YELLOW}  🔴 Redis:          localhost:6379${NC}"
            echo -e "${YELLOW}  📨 Kafka:          localhost:9092${NC}"
            log_warning "监控服务未启动。使用 --with-monitoring 参数启动监控。"
        fi
    fi
    
    if [ $? -eq 0 ]; then
        log_info "等待服务完全启动..."
        sleep 10
        show_status
    else
        log_error "环境启动失败"
        exit 1
    fi
}

function stop_test_environment() {
    log_info "停止测试环境..."
    docker-compose -f docker-compose.test.yml --profile monitoring down
    
    if [ $? -eq 0 ]; then
        log_success "测试环境已停止"
    else
        log_error "停止测试环境失败"
        exit 1
    fi
}

function restart_test_environment() {
    local include_monitoring=$1
    log_info "重启测试环境..."
    stop_test_environment
    sleep 5
    start_test_environment $include_monitoring
}

function show_status() {
    log_info "检查服务状态..."
    docker-compose -f docker-compose.test.yml --profile monitoring ps
    
    log_info "检查服务健康状态..."
    
    # 检查网关服务
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/health | grep -q "200"; then
        log_success "网关健康检查 服务正常"
    else
        log_warning "网关健康检查 服务无法访问"
    fi
    
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:9091/metrics | grep -q "200"; then
        log_success "网关Metrics 服务正常"
    else
        log_warning "网关Metrics 服务无法访问"
    fi
    
    # 如果监控服务在运行，也检查监控服务
    if docker ps --filter name=prometheus-test --format '{{.Names}}' | grep -q prometheus-test; then
        if curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/api/health | grep -q "200"; then
            log_success "Grafana 服务正常"
        else
            log_warning "Grafana 服务无法访问"
        fi
        
        if curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/-/healthy | grep -q "200"; then
            log_success "Prometheus 服务正常"
        else
            log_warning "Prometheus 服务无法访问"
        fi
        
        if curl -s -o /dev/null -w "%{http_code}" http://localhost:9093/-/healthy | grep -q "200"; then
            log_success "AlertManager 服务正常"
        else
            log_warning "AlertManager 服务无法访问"
        fi
    fi
}

function show_logs() {
    log_info "显示服务日志..."
    docker-compose -f docker-compose.test.yml --profile monitoring logs -f
}

# 主逻辑
if [ "$STOP" = true ]; then
    stop_test_environment
elif [ "$RESTART" = true ]; then
    restart_test_environment $WITH_MONITORING
elif [ "$STATUS" = true ]; then
    show_status
elif [ "$LOGS" = true ]; then
    show_logs
else
    start_test_environment $WITH_MONITORING
fi

log_info "脚本执行完成"
