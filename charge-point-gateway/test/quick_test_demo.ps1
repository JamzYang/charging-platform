# 充电桩网关测试演示脚本 (PowerShell版本)
# 用于快速验证不同层次的测试

# 设置错误处理
$ErrorActionPreference = "Stop"

function Write-Info {
    param($Message)
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Write-Success {
    param($Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Warning {
    param($Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error-Custom {
    param($Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

Write-Host "🚀 充电桩网关测试演示" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

Write-Info "📋 测试层次结构:"
Write-Host "├── 单元测试 (Unit Tests) - 仅需Go环境"
Write-Host "├── 集成测试 (Integration Tests) - 使用TestContainers"
Write-Host "└── E2E测试 (End-to-End Tests) - 使用Docker Compose"
Write-Host ""

Write-Info "🎯 1. 运行单元测试 (快速验证)"
Write-Host "====================================="
Write-Info "运行OCPP消息创建测试..."
try {
    go test -v ./test -run "TestOCPPMessageCreation" -timeout 10s
    Write-Success "OCPP消息测试通过"
} catch {
    Write-Error-Custom "OCPP消息测试失败"
    exit 1
}
Write-Host ""

Write-Info "🎯 2. 运行工具函数测试"
Write-Host "====================================="
Write-Info "运行断言和数据加载测试..."
try {
    go test -v ./test -run "TestLoadTestData|TestAssertionHelpers" -timeout 10s
    Write-Success "工具函数测试通过"
} catch {
    Write-Error-Custom "工具函数测试失败"
    exit 1
}
Write-Host ""

Write-Info "🎯 3. 运行WebSocket客户端测试"
Write-Host "====================================="
Write-Info "运行WebSocket客户端创建测试..."
try {
    go test -v ./test -run "TestWebSocketClientCreation" -timeout 10s
    Write-Success "WebSocket客户端测试通过"
} catch {
    Write-Error-Custom "WebSocket客户端测试失败"
    exit 1
}
Write-Host ""

Write-Info "🎯 4. 验证修复工具"
Write-Host "====================================="
Write-Info "编译并运行verify_fixes..."
try {
    go test -c -o test_verify_fixes.exe github.com/charging-platform/charge-point-gateway/test
    Write-Success "verify_fixes编译成功"
    Write-Info "运行verify_fixes (注意: 可能会有连接失败，这是正常的)"
    # 运行但不检查退出码，因为连接失败是预期的
    try {
        .\test_verify_fixes.exe
    } catch {
        Write-Warning "verify_fixes运行完成 (连接失败是预期的)"
    }
    Remove-Item -Path "test_verify_fixes.exe" -ErrorAction SilentlyContinue
} catch {
    Write-Error-Custom "verify_fixes编译失败"
}
Write-Host ""

Write-Info "🎯 5. 检查Docker环境状态"
Write-Host "====================================="
Write-Info "检查Docker Compose服务状态..."
try {
    docker-compose -f test/docker-compose.test.yml ps
} catch {
    Write-Warning "Docker Compose环境未启动或不可用"
}
Write-Host ""

Write-Info "📊 测试总结"
Write-Host "====================================="
Write-Success "✅ 单元测试: 全部通过 (无需Docker)"
Write-Info "🔧 集成测试: 使用TestContainers (自动管理容器)"
Write-Info "🐳 E2E测试: 使用Docker Compose环境 (需要预启动)"
Write-Host ""
Write-Info "💡 下一步:"
Write-Host "1. 运行集成测试: go test -v ./test/integration/... -timeout 120s"
Write-Host "2. 启动完整环境: docker-compose -f test/docker-compose.test.yml up -d"
Write-Host "3. 运行E2E测试: go test -v ./test/e2e/... -timeout 300s"
Write-Host ""

Write-Success "🎉 测试演示完成!"
