#!/bin/bash

# 启动监控栈脚本

set -e

# 进入脚本所在目录
cd "$(dirname "$0")"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
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

function start_monitoring() {
    log_info "启动监控栈..."
    
    # 检查Docker是否运行
    if ! docker version >/dev/null 2>&1; then
        log_error "Docker未运行，请先启动Docker"
        exit 1
    fi
    
    # 启动监控服务
    log_info "启动Prometheus、Grafana和相关监控服务..."
    docker-compose -f docker-compose.monitoring.yml up -d
    
    if [ $? -eq 0 ]; then
        log_success "监控栈启动成功！"
        log_info "服务访问地址："
        echo -e "${YELLOW}  📊 Grafana:      http://localhost:3000 (admin/admin123)${NC}"
        echo -e "${YELLOW}  📈 Prometheus:   http://localhost:9090${NC}"
        echo -e "${YELLOW}  🚨 AlertManager: http://localhost:9093${NC}"
        echo -e "${YELLOW}  💻 Node Exporter: http://localhost:9100${NC}"
        echo -e "${YELLOW}  🐳 cAdvisor:     http://localhost:8080${NC}"
        echo -e "${YELLOW}  🔴 Redis Metrics: http://localhost:9121${NC}"
        echo -e "${YELLOW}  📨 Kafka Metrics: http://localhost:9308${NC}"
        log_info "等待服务完全启动..."
        sleep 10
        show_status
    else
        log_error "监控栈启动失败"
        exit 1
    fi
}

function stop_monitoring() {
    log_info "停止监控栈..."
    docker-compose -f docker-compose.monitoring.yml down
    
    if [ $? -eq 0 ]; then
        log_success "监控栈已停止"
    else
        log_error "停止监控栈失败"
        exit 1
    fi
}

function restart_monitoring() {
    log_info "重启监控栈..."
    stop_monitoring
    sleep 5
    start_monitoring
}

function show_status() {
    log_info "检查监控服务状态..."
    docker-compose -f docker-compose.monitoring.yml ps
    
    log_info "检查服务健康状态..."
    
    # 检查Grafana
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/api/health | grep -q "200"; then
        log_success "Grafana 服务正常"
    else
        log_warning "Grafana 服务无法访问"
    fi
    
    # 检查Prometheus
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/-/healthy | grep -q "200"; then
        log_success "Prometheus 服务正常"
    else
        log_warning "Prometheus 服务无法访问"
    fi
    
    # 检查AlertManager
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:9093/-/healthy | grep -q "200"; then
        log_success "AlertManager 服务正常"
    else
        log_warning "AlertManager 服务无法访问"
    fi
}

function show_logs() {
    log_info "显示监控服务日志..."
    docker-compose -f docker-compose.monitoring.yml logs -f
}

function show_help() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  start     启动监控栈 (默认)"
    echo "  stop      停止监控栈"
    echo "  restart   重启监控栈"
    echo "  status    显示服务状态"
    echo "  logs      显示服务日志"
    echo "  help      显示此帮助信息"
    echo ""
}

# 主逻辑
case "${1:-start}" in
    start)
        start_monitoring
        ;;
    stop)
        stop_monitoring
        ;;
    restart)
        restart_monitoring
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    help)
        show_help
        ;;
    *)
        log_error "未知选项: $1"
        show_help
        exit 1
        ;;
esac

log_info "脚本执行完成"
