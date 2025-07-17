# 配置修复验证脚本
# 用于验证配置管理修复是否成功

param(
    [string]$Profile = "dev"
)

Write-Host "=== 配置修复验证脚本 ===" -ForegroundColor Green
Write-Host "测试环境: $Profile" -ForegroundColor Yellow

# 进入项目目录
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectDir = Split-Path -Parent $scriptDir
Set-Location $projectDir

Write-Host "`n--- 1. 检查配置文件结构 ---" -ForegroundColor Cyan
if (Test-Path "configs/application.yaml") {
    Write-Host "✅ application.yaml 存在" -ForegroundColor Green
} else {
    Write-Host "❌ application.yaml 不存在" -ForegroundColor Red
}

if (Test-Path "configs/application-$Profile.yaml") {
    Write-Host "✅ application-$Profile.yaml 存在" -ForegroundColor Green
} else {
    Write-Host "❌ application-$Profile.yaml 不存在" -ForegroundColor Red
}

Write-Host "`n--- 2. 构建配置测试工具 ---" -ForegroundColor Cyan
try {
    go build -o bin/config-test.exe ./cmd/config-test/main.go
    Write-Host "✅ 配置测试工具构建成功" -ForegroundColor Green
} catch {
    Write-Host "❌ 配置测试工具构建失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n--- 3. 测试默认配置加载 ---" -ForegroundColor Cyan
$env:APP_PROFILE = $Profile
./bin/config-test.exe

Write-Host "`n--- 4. 测试环境变量覆盖 ---" -ForegroundColor Cyan
$env:APP_PROFILE = $Profile
$env:REDIS_ADDR = "test-redis:6379"
$env:KAFKA_BROKERS = "test-kafka:9092"
$env:LOG_LEVEL = "debug"
$env:SERVER_PORT = "8888"

Write-Host "设置测试环境变量:" -ForegroundColor Yellow
Write-Host "  REDIS_ADDR = $env:REDIS_ADDR"
Write-Host "  KAFKA_BROKERS = $env:KAFKA_BROKERS"
Write-Host "  LOG_LEVEL = $env:LOG_LEVEL"
Write-Host "  SERVER_PORT = $env:SERVER_PORT"

./bin/config-test.exe

# 清理环境变量
Remove-Item Env:REDIS_ADDR -ErrorAction SilentlyContinue
Remove-Item Env:KAFKA_BROKERS -ErrorAction SilentlyContinue
Remove-Item Env:LOG_LEVEL -ErrorAction SilentlyContinue
Remove-Item Env:SERVER_PORT -ErrorAction SilentlyContinue

Write-Host "`n--- 5. 测试Docker环境配置 ---" -ForegroundColor Cyan
$env:APP_PROFILE = "test"
$env:REDIS_ADDR = "redis-test:6379"
$env:KAFKA_BROKERS = "kafka-test:9092"

Write-Host "模拟Docker环境变量:" -ForegroundColor Yellow
Write-Host "  APP_PROFILE = $env:APP_PROFILE"
Write-Host "  REDIS_ADDR = $env:REDIS_ADDR"
Write-Host "  KAFKA_BROKERS = $env:KAFKA_BROKERS"

./bin/config-test.exe

# 清理环境变量
Remove-Item Env:APP_PROFILE -ErrorAction SilentlyContinue
Remove-Item Env:REDIS_ADDR -ErrorAction SilentlyContinue
Remove-Item Env:KAFKA_BROKERS -ErrorAction SilentlyContinue

Write-Host "`n=== 配置验证完成 ===" -ForegroundColor Green
Write-Host "如果所有测试都显示正确的配置值，说明修复成功！" -ForegroundColor Yellow
