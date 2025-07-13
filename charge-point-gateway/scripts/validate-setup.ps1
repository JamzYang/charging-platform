# 验证开发环境设置脚本 (PowerShell)

Write-Host "🔧 Validating development environment setup..." -ForegroundColor Green

# 进入项目目录
Set-Location (Split-Path $PSScriptRoot -Parent)

# 检查Go版本
Write-Host "Checking Go version..." -ForegroundColor Yellow
$goVersion = go version
Write-Host "Go version: $goVersion" -ForegroundColor Cyan

# 检查项目结构
Write-Host "Checking project structure..." -ForegroundColor Yellow
$requiredDirs = @("cmd/gateway", "internal/config", "configs", "scripts")
foreach ($dir in $requiredDirs) {
    if (Test-Path $dir) {
        Write-Host "✅ Directory $dir exists" -ForegroundColor Green
    } else {
        Write-Host "❌ Directory $dir missing" -ForegroundColor Red
        exit 1
    }
}

# 检查必要文件
Write-Host "Checking required files..." -ForegroundColor Yellow
$requiredFiles = @("go.mod", ".golangci.yml", ".editorconfig", "Makefile", "README.md")
foreach ($file in $requiredFiles) {
    if (Test-Path $file) {
        Write-Host "✅ File $file exists" -ForegroundColor Green
    } else {
        Write-Host "❌ File $file missing" -ForegroundColor Red
        exit 1
    }
}

# 检查依赖
Write-Host "Checking dependencies..." -ForegroundColor Yellow
$verifyResult = go mod verify
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Dependencies verified" -ForegroundColor Green
} else {
    Write-Host "❌ Dependencies verification failed" -ForegroundColor Red
    exit 1
}

# 检查代码编译
Write-Host "Checking code compilation..." -ForegroundColor Yellow
$buildResult = go build ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Code compiles successfully" -ForegroundColor Green
} else {
    Write-Host "❌ Code compilation failed" -ForegroundColor Red
    exit 1
}

# 运行测试
Write-Host "Running tests..." -ForegroundColor Yellow
$testResult = go test ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ All tests pass" -ForegroundColor Green
} else {
    Write-Host "❌ Tests failed" -ForegroundColor Red
    exit 1
}

# 检查代码格式
Write-Host "Checking code format..." -ForegroundColor Yellow
$formatIssues = go fmt ./...
if ([string]::IsNullOrEmpty($formatIssues)) {
    Write-Host "✅ Code is properly formatted" -ForegroundColor Green
} else {
    Write-Host "❌ Code formatting issues found:" -ForegroundColor Red
    Write-Host $formatIssues -ForegroundColor Red
    exit 1
}

Write-Host "🎉 Development environment setup validation completed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" -ForegroundColor White
Write-Host "2. Run linter: golangci-lint run ./..." -ForegroundColor White
Write-Host "3. Start development: make dev" -ForegroundColor White
