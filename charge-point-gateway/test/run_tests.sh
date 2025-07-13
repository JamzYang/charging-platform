#!/bin/bash

# 测试运行脚本
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
    cat << EOF
Usage: $0 [OPTIONS] [TEST_TYPE]

测试运行脚本，用于启动测试环境并运行各种类型的测试。

TEST_TYPE:
    unit            运行单元测试
    integration     运行集成测试
    e2e             运行端到端测试
    performance     运行性能测试
    all             运行所有测试（默认）

OPTIONS:
    -h, --help      显示此帮助信息
    -v, --verbose   详细输出
    -c, --clean     测试前清理环境
    -s, --setup     只启动测试环境，不运行测试
    -d, --down      停止并清理测试环境
    --no-docker     不使用Docker环境（仅运行单元测试）
    --debug         启用调试工具（Kafka UI, Redis Commander）

Examples:
    $0                          # 运行所有测试
    $0 integration              # 只运行集成测试
    $0 -c all                   # 清理环境后运行所有测试
    $0 -s                       # 只启动测试环境
    $0 -d                       # 停止测试环境
    $0 --no-docker unit         # 不使用Docker运行单元测试
    $0 --debug e2e              # 启用调试工具运行E2E测试

EOF
}

# 默认参数
TEST_TYPE="all"
VERBOSE=false
CLEAN=false
SETUP_ONLY=false
DOWN_ONLY=false
NO_DOCKER=false
DEBUG=false

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
        -s|--setup)
            SETUP_ONLY=true
            shift
            ;;
        -d|--down)
            DOWN_ONLY=true
            shift
            ;;
        --no-docker)
            NO_DOCKER=true
            shift
            ;;
        --debug)
            DEBUG=true
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

# 获取脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 切换到项目目录
cd "$PROJECT_DIR"

# 停止并清理环境
cleanup_environment() {
    log_info "停止测试环境..."
    
    if [ -f "test/docker-compose.test.yml" ]; then
        docker-compose -f test/docker-compose.test.yml down -v --remove-orphans
        log_success "测试环境已停止"
    else
        log_warning "Docker Compose文件不存在"
    fi
    
    # 清理测试生成的文件
    rm -f coverage.out coverage.html test-results.json
    log_info "清理完成"
}

# 如果只是要停止环境
if [ "$DOWN_ONLY" = true ]; then
    cleanup_environment
    exit 0
fi

# 清理环境（如果需要）
if [ "$CLEAN" = true ]; then
    cleanup_environment
fi

# 启动测试环境
setup_environment() {
    if [ "$NO_DOCKER" = true ]; then
        log_info "跳过Docker环境启动"
        return 0
    fi
    
    log_info "启动测试环境..."
    
    # 检查Docker是否运行
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker未运行，请启动Docker"
        exit 1
    fi
    
    # 检查Docker Compose文件
    if [ ! -f "test/docker-compose.test.yml" ]; then
        log_error "Docker Compose文件不存在: test/docker-compose.test.yml"
        exit 1
    fi
    
    # 启动服务
    local compose_args=""
    if [ "$DEBUG" = true ]; then
        compose_args="--profile debug"
    fi
    
    docker-compose -f test/docker-compose.test.yml up -d $compose_args
    
    # 等待服务启动
    log_info "等待服务启动..."
    sleep 10
    
    # 检查服务健康状态
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        log_info "检查服务状态 (尝试 $attempt/$max_attempts)..."
        
        if docker-compose -f test/docker-compose.test.yml ps | grep -q "healthy"; then
            log_success "测试环境启动成功"
            return 0
        fi
        
        sleep 5
        ((attempt++))
    done
    
    log_error "测试环境启动超时"
    docker-compose -f test/docker-compose.test.yml logs
    exit 1
}

# 运行测试
run_tests() {
    local test_type=$1
    
    log_info "运行 $test_type 测试..."
    
    case $test_type in
        unit)
            go test -v -race -coverprofile=coverage.out ./internal/... ./cmd/...
            ;;
        integration)
            if [ "$NO_DOCKER" = true ]; then
                log_warning "集成测试需要Docker环境，跳过"
                return 0
            fi
            go test -v -race -tags=integration ./test/integration/...
            ;;
        e2e)
            if [ "$NO_DOCKER" = true ]; then
                log_warning "E2E测试需要Docker环境，跳过"
                return 0
            fi
            go test -v -race -tags=e2e ./test/e2e/...
            ;;
        performance)
            if [ "$NO_DOCKER" = true ]; then
                log_warning "性能测试需要Docker环境，跳过"
                return 0
            fi
            go test -v -race -tags=performance ./test/e2e/performance/...
            ;;
        all)
            run_tests unit
            if [ "$NO_DOCKER" = false ]; then
                run_tests integration
                run_tests e2e
            fi
            ;;
        *)
            log_error "未知测试类型: $test_type"
            exit 1
            ;;
    esac
    
    if [ $? -eq 0 ]; then
        log_success "$test_type 测试通过"
    else
        log_error "$test_type 测试失败"
        exit 1
    fi
}

# 生成测试报告
generate_report() {
    if [ -f "coverage.out" ]; then
        log_info "生成测试覆盖率报告..."
        go tool cover -html=coverage.out -o coverage.html
        go tool cover -func=coverage.out
        log_success "覆盖率报告已生成: coverage.html"
    fi
}

# 显示环境信息
show_environment_info() {
    if [ "$NO_DOCKER" = false ]; then
        log_info "测试环境信息:"
        echo "  Gateway URL: http://localhost:8081"
        echo "  WebSocket URL: ws://localhost:8081/ocpp"
        echo "  Metrics: http://localhost:9091/metrics"
        echo "  Health Check: http://localhost:8083/health"
        
        if [ "$DEBUG" = true ]; then
            echo "  Kafka UI: http://localhost:8082"
            echo "  Redis Commander: http://localhost:8084"
        fi
    fi
}

# 主执行流程
main() {
    log_info "开始执行测试..."
    log_info "测试类型: $TEST_TYPE"
    log_info "项目目录: $PROJECT_DIR"
    
    # 启动环境
    setup_environment
    
    # 显示环境信息
    show_environment_info
    
    # 如果只是启动环境
    if [ "$SETUP_ONLY" = true ]; then
        log_success "测试环境已启动，使用 '$0 -d' 停止环境"
        exit 0
    fi
    
    # 运行测试
    run_tests "$TEST_TYPE"
    
    # 生成报告
    generate_report
    
    log_success "所有测试完成"
}

# 捕获退出信号，确保清理
trap 'log_warning "收到中断信号，正在清理..."; cleanup_environment; exit 1' INT TERM

# 执行主函数
main
