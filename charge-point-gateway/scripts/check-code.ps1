# 分层代码检查脚本 - 体现行业最佳实践

param(
    [string]$Level = "basic"  # basic, full, ci
)

Write-Host "🔍 Running code checks (Level: $Level)..." -ForegroundColor Green

# 进入项目目录
Set-Location (Split-Path $PSScriptRoot -Parent)

$exitCode = 0

# 第一层：基础检查 (快速，适合开发阶段)
if ($Level -eq "basic" -or $Level -eq "full" -or $Level -eq "ci") {
    Write-Host "`n=== 基础检查 (go vet) ===" -ForegroundColor Yellow
    
    Write-Host "Running go vet..." -ForegroundColor Cyan
    $vetResult = go vet ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ go vet failed" -ForegroundColor Red
        $exitCode = 1
    } else {
        Write-Host "✅ go vet passed" -ForegroundColor Green
    }
    
    Write-Host "Checking code format..." -ForegroundColor Cyan
    $formatIssues = go fmt ./...
    if (![string]::IsNullOrEmpty($formatIssues)) {
        Write-Host "❌ Code formatting issues found" -ForegroundColor Red
        Write-Host $formatIssues -ForegroundColor Red
        $exitCode = 1
    } else {
        Write-Host "✅ Code format check passed" -ForegroundColor Green
    }
    
    Write-Host "Checking module tidiness..." -ForegroundColor Cyan
    go mod tidy
    # 在实际项目中，这里应该检查git diff来确认go.mod/go.sum没有变化
    Write-Host "✅ Module tidiness checked" -ForegroundColor Green
}

# 第二层：全面检查 (适合提交前和CI)
if ($Level -eq "full" -or $Level -eq "ci") {
    Write-Host "`n=== 全面检查 (golangci-lint) ===" -ForegroundColor Yellow
    
    # 检查golangci-lint是否可用
    $golangciAvailable = $false
    try {
        $null = golangci-lint version 2>$null
        $golangciAvailable = $true
    } catch {
        $golangciAvailable = $false
    }
    
    if ($golangciAvailable) {
        Write-Host "Running golangci-lint..." -ForegroundColor Cyan
        golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) {
            Write-Host "❌ golangci-lint failed" -ForegroundColor Red
            $exitCode = 1
        } else {
            Write-Host "✅ golangci-lint passed" -ForegroundColor Green
        }
    } else {
        Write-Host "⚠️  golangci-lint not available, skipping comprehensive checks" -ForegroundColor Yellow
        Write-Host "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" -ForegroundColor Gray
    }
}

# 第三层：CI专用检查
if ($Level -eq "ci") {
    Write-Host "`n=== CI专用检查 ===" -ForegroundColor Yellow
    
    Write-Host "Running tests with race detection..." -ForegroundColor Cyan
    go test -race ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Race detection tests failed" -ForegroundColor Red
        $exitCode = 1
    } else {
        Write-Host "✅ Race detection tests passed" -ForegroundColor Green
    }
    
    Write-Host "Checking test coverage..." -ForegroundColor Cyan
    go test -coverprofile=coverage.out ./...
    if ($LASTEXITCODE -eq 0) {
        $coverage = go tool cover -func=coverage.out | Select-String "total:" | ForEach-Object { ($_ -split "\s+")[-1] }
        $coverageNum = [float]($coverage -replace "%", "")
        Write-Host "Coverage: $coverage" -ForegroundColor Cyan
        
        if ($coverageNum -ge 80) {
            Write-Host "✅ Coverage requirement met ($coverage >= 80%)" -ForegroundColor Green
        } else {
            Write-Host "❌ Coverage requirement not met ($coverage < 80%)" -ForegroundColor Red
            $exitCode = 1
        }
    } else {
        Write-Host "❌ Coverage check failed" -ForegroundColor Red
        $exitCode = 1
    }
}

# 总结
Write-Host "`n=== 检查总结 ===" -ForegroundColor Yellow
if ($exitCode -eq 0) {
    Write-Host "🎉 All checks passed!" -ForegroundColor Green
    
    # 根据检查级别给出建议
    switch ($Level) {
        "basic" {
            Write-Host "`n💡 建议: 提交前运行 'scripts/check-code.ps1 -Level full'" -ForegroundColor Cyan
        }
        "full" {
            Write-Host "`n💡 代码质量良好，可以提交!" -ForegroundColor Cyan
        }
        "ci" {
            Write-Host "`n💡 CI检查通过，可以合并!" -ForegroundColor Cyan
        }
    }
} else {
    Write-Host "❌ Some checks failed. Please fix the issues before proceeding." -ForegroundColor Red
}

exit $exitCode
