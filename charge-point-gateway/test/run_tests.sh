#!/bin/bash

# 充电桩网关测试运行脚本
# 用于启动测试环境并运行各种类型的测试

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

# 显示帮助信息
show_help() {
    echo "充电桩网关测试运行脚本"
    echo ""
    echo "用法: $0 [选项] [测试类型]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示此帮助信息"
    echo "  -v, --verbose  详细输出"
    echo "  -c, --clean    测试前清理环境"
    echo "  --no-build     跳过构建步骤"
    echo "  --keep-env     测试后保持环境运行"
    echo "  --local-only   仅运行本地测试（不启动Docker服务）"
    echo ""
    echo "测试类型:"
    echo "  unit           运行单元测试"
    echo "  integration    运行集成测试"
    echo "  e2e            运行端到端测试"
    echo "  performance    运行性能测试"
    echo "  all            运行所有测试 (默认)"
    echo ""
    echo "示例:"
    echo "  $0 integration              # 运行集成测试"
    echo "  $0 -v -c e2e               # 详细输出，清理环境，运行E2E测试"
    echo "  $0 --local-only unit       # 仅运行本地单元测试"
    echo "  $0 --no-build --keep-env   # 跳过构建，保持环境，运行所有测试"
}

# 默认参数
VERBOSE=false
CLEAN=false
NO_BUILD=false
KEEP_ENV=false
LOCAL_ONLY=false
TEST_TYPE="all"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        --no-build)
            NO_BUILD=true
            shift
            ;;
        --keep-env)
            KEEP_ENV=true
            shift
            ;;
        --local-only)
            LOCAL_ONLY=true
            shift
            ;;
        unit|integration|e2e|performance|all)
            TEST_TYPE=$1
            shift
            ;;
        *)
            log_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查Docker和Docker Compose
check_dependencies() {
    log_info "检查依赖..."
    
    if [ "$LOCAL_ONLY" = false ]; then
        if ! command -v docker &> /dev/null; then
            log_error "Docker 未安装或不在PATH中"
            exit 1
        fi
        
        if ! command -v docker-compose &> /dev/null; then
            log_error "Docker Compose 未安装或不在PATH中"
            exit 1
        fi
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装或不在PATH中"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 清理环境
cleanup_environment() {
    if [ "$CLEAN" = true ] && [ "$LOCAL_ONLY" = false ]; then
        log_info "清理测试环境..."
        docker-compose -f test/docker-compose.test.yml down -v --remove-orphans 2>/dev/null || true
        docker system prune -f 2>/dev/null || true
        log_success "环境清理完成"
    fi
}

# 构建应用
build_application() {
    if [ "$NO_BUILD" = false ]; then
        log_info "构建应用..."
        
        # 构建Go应用
        go mod tidy
        
        if [ "$LOCAL_ONLY" = false ]; then
            go build -o bin/gateway ./cmd/gateway
            
            # 构建Docker镜像
            docker build -t charge-point-gateway:test .
        fi
        
        log_success "应用构建完成"
    else
        log_info "跳过构建步骤"
    fi
}

# 启动测试环境
start_test_environment() {
    if [ "$LOCAL_ONLY" = true ]; then
        log_info "本地测试模式，跳过Docker环境启动"
        return 0
    fi
    
    log_info "启动测试环境..."
    
    # 启动基础服务 (Redis, Kafka)
    docker-compose -f test/docker-compose.test.yml up -d redis-test zookeeper-test kafka-test
    
    # 等待服务健康检查通过
    log_info "等待服务启动..."
    
    # 等待Redis
    timeout=60
    while [ $timeout -gt 0 ]; do
        if docker-compose -f test/docker-compose.test.yml exec -T redis-test redis-cli ping &> /dev/null; then
            break
        fi
        sleep 1
        ((timeout--))
    done
    
    if [ $timeout -eq 0 ]; then
        log_error "Redis 启动超时"
        exit 1
    fi
    
    # 等待Kafka
    timeout=120
    while [ $timeout -gt 0 ]; do
        if docker-compose -f test/docker-compose.test.yml exec -T kafka-test kafka-topics --bootstrap-server kafka-test:9092 --list &> /dev/null; then
            break
        fi
        sleep 1
        ((timeout--))
    done
    
    if [ $timeout -eq 0 ]; then
        log_error "Kafka 启动超时"
        exit 1
    fi
    
    log_success "测试环境启动完成"
}

# 创建Kafka主题
create_kafka_topics() {
    if [ "$LOCAL_ONLY" = true ]; then
        return 0
    fi
    
    log_info "创建Kafka主题..."
    
    # 创建测试主题
    docker-compose -f test/docker-compose.test.yml exec -T kafka-test kafka-topics \
        --bootstrap-server kafka-test:9092 \
        --create --if-not-exists \
        --topic ocpp-events-up-test \
        --partitions 3 \
        --replication-factor 1
    
    docker-compose -f test/docker-compose.test.yml exec -T kafka-test kafka-topics \
        --bootstrap-server kafka-test:9092 \
        --create --if-not-exists \
        --topic commands-down-test \
        --partitions 3 \
        --replication-factor 1
    
    log_success "Kafka主题创建完成"
}

# 运行单元测试
run_unit_tests() {
    log_info "运行单元测试..."
    
    if [ "$VERBOSE" = true ]; then
        go test -v ./internal/... -timeout 30s
    else
        go test ./internal/... -timeout 30s
    fi
    
    if [ $? -eq 0 ]; then
        log_success "单元测试通过"
    else
        log_error "单元测试失败"
        return 1
    fi
}

# 运行集成测试
run_integration_tests() {
    if [ "$LOCAL_ONLY" = true ]; then
        log_warning "本地模式跳过集成测试（需要Docker环境）"
        return 0
    fi
    
    log_info "运行集成测试..."
    
    if [ "$VERBOSE" = true ]; then
        go test -v ./test/integration/... -timeout 60s
    else
        go test ./test/integration/... -timeout 60s
    fi
    
    if [ $? -eq 0 ]; then
        log_success "集成测试通过"
    else
        log_error "集成测试失败"
        return 1
    fi
}

# 运行E2E测试
run_e2e_tests() {
    if [ "$LOCAL_ONLY" = true ]; then
        log_warning "本地模式跳过E2E测试（需要Docker环境）"
        return 0
    fi
    
    log_info "启动网关服务进行E2E测试..."
    
    # 启动网关服务
    docker-compose -f test/docker-compose.test.yml up -d gateway-test
    
    # 等待网关服务启动
    timeout=60
    while [ $timeout -gt 0 ]; do
        if curl -f http://localhost:8081/health &> /dev/null; then
            break
        fi
        sleep 1
        ((timeout--))
    done
    
    if [ $timeout -eq 0 ]; then
        log_error "网关服务启动超时"
        return 1
    fi
    
    log_info "运行E2E测试..."
    
    if [ "$VERBOSE" = true ]; then
        go test -v ./test/e2e/... -timeout 120s
    else
        go test ./test/e2e/... -timeout 120s
    fi
    
    if [ $? -eq 0 ]; then
        log_success "E2E测试通过"
    else
        log_error "E2E测试失败"
        return 1
    fi
}

# 主函数
main() {
    log_info "开始运行充电桩网关测试 (类型: $TEST_TYPE)"
    
    # 检查依赖
    check_dependencies
    
    # 清理环境
    cleanup_environment
    
    # 构建应用
    build_application
    
    # 启动测试环境
    start_test_environment
    
    # 创建Kafka主题
    create_kafka_topics
    
    # 运行测试
    test_failed=false
    
    case $TEST_TYPE in
        unit)
            run_unit_tests || test_failed=true
            ;;
        integration)
            run_integration_tests || test_failed=true
            ;;
        e2e)
            run_e2e_tests || test_failed=true
            ;;
        all)
            run_unit_tests || test_failed=true
            if [ "$LOCAL_ONLY" = false ]; then
                run_integration_tests || test_failed=true
                run_e2e_tests || test_failed=true
            fi
            ;;
    esac
    
    # 停止测试环境
    if [ "$LOCAL_ONLY" = false ] && [ "$KEEP_ENV" = false ]; then
        log_info "停止测试环境..."
        docker-compose -f test/docker-compose.test.yml down
        log_success "测试环境已停止"
    elif [ "$LOCAL_ONLY" = false ] && [ "$KEEP_ENV" = true ]; then
        log_info "保持测试环境运行"
        log_info "可以通过以下命令停止: docker-compose -f test/docker-compose.test.yml down"
    fi
    
    # 输出结果
    if [ "$test_failed" = true ]; then
        log_error "测试失败"
        exit 1
    else
        log_success "所有测试通过"
    fi
}

# 捕获中断信号，确保清理
trap 'log_warning "收到中断信号，清理环境..."; docker-compose -f test/docker-compose.test.yml down 2>/dev/null || true; exit 1' INT TERM

# 运行主函数
main
