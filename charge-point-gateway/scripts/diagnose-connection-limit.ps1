# 连接限制诊断脚本
# 用于诊断为什么服务器主动拒绝连接

param(
    [string]$GatewayURL = "http://localhost:8081",
    [int]$SampleInterval = 5
)

Write-Host "连接限制诊断脚本" -ForegroundColor Green
Write-Host "目标网关: $GatewayURL" -ForegroundColor Cyan
Write-Host "采样间隔: $SampleInterval 秒" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Green

# 1. 检查网关健康状态
Write-Host "`n1. 检查网关健康状态..." -ForegroundColor Yellow

try {
    $healthResponse = Invoke-WebRequest -Uri "$GatewayURL/health" -Method GET -TimeoutSec 10
    if ($healthResponse.StatusCode -eq 200 -and $healthResponse.Content -eq "OK") {
        Write-Host "网关状态: healthy" -ForegroundColor Green

        # 通过TCP连接统计估算连接数
        $port = ($GatewayURL -split ':')[-1]
        if ($port -match '/') {
            $port = ($port -split '/')[0]
        }
        $tcpConnections = (netstat -an | findstr ":$port.*ESTABLISHED").Count
        Write-Host "当前TCP连接数: $tcpConnections" -ForegroundColor Green
        Write-Host "健康检查: 正常" -ForegroundColor Green
    } else {
        Write-Host "网关状态: 异常" -ForegroundColor Red
    }
} catch {
    Write-Host "无法连接到网关健康检查端点: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "请确认网关正在运行且端口正确" -ForegroundColor Yellow
    exit 1
}

# 2. 检查系统连接状态
Write-Host "`n2. 检查系统连接状态..." -ForegroundColor Yellow

$port = ($GatewayURL -split ':')[-1]
if ($port -match '/') {
    $port = ($port -split '/')[0]
}

Write-Host "监控端口: $port" -ForegroundColor Cyan

# 统计各种连接状态
$established = (netstat -an | findstr ":$port.*ESTABLISHED").Count
$timeWait = (netstat -an | findstr ":$port.*TIME_WAIT").Count
$listening = (netstat -an | findstr ":$port.*LISTENING").Count
$closeWait = (netstat -an | findstr ":$port.*CLOSE_WAIT").Count

Write-Host "TCP 连接状态统计:" -ForegroundColor Cyan
Write-Host "  ESTABLISHED: $established" -ForegroundColor Green
Write-Host "  TIME_WAIT: $timeWait" -ForegroundColor Yellow
Write-Host "  LISTENING: $listening" -ForegroundColor Green
Write-Host "  CLOSE_WAIT: $closeWait" -ForegroundColor $(if($closeWait -gt 0) {"Red"} else {"Green"})

# 3. 检查进程资源使用
Write-Host "`n3. 检查进程资源使用..." -ForegroundColor Yellow

$gatewayProcess = Get-Process -Name "gateway*" -ErrorAction SilentlyContinue
if ($gatewayProcess) {
    Write-Host "网关进程信息:" -ForegroundColor Cyan
    Write-Host "  PID: $($gatewayProcess.Id)" -ForegroundColor Green
    Write-Host "  CPU 使用率: $($gatewayProcess.CPU)" -ForegroundColor Green
    Write-Host "  内存使用: $([math]::Round($gatewayProcess.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor Green
    Write-Host "  句柄数: $($gatewayProcess.HandleCount)" -ForegroundColor Green
    Write-Host "  线程数: $($gatewayProcess.Threads.Count)" -ForegroundColor Green
} else {
    Write-Host "未找到网关进程" -ForegroundColor Red
}

# 4. 持续监控模式
Write-Host "`n4. 开始持续监控..." -ForegroundColor Yellow
Write-Host "按 Ctrl+C 停止监控" -ForegroundColor Gray

$monitorCount = 0
$maxConnectionsSeen = 0
$rejectionStarted = $false

try {
    while ($true) {
        Start-Sleep $SampleInterval
        $monitorCount++
        
        # 获取当前时间
        $timestamp = Get-Date -Format "HH:mm:ss"
        
        # 获取健康状态
        try {
            $healthCheck = Invoke-WebRequest -Uri "$GatewayURL/health" -Method GET -TimeoutSec 5
            $isHealthy = $healthCheck.StatusCode -eq 200 -and $healthCheck.Content -eq "OK"

            # 通过TCP连接数估算当前连接数
            $currentConnections = (netstat -an | findstr ":$port.*ESTABLISHED").Count
            
            # 更新最大连接数
            if ($currentConnections -gt $maxConnectionsSeen) {
                $maxConnectionsSeen = $currentConnections
            }
            
            # 检测连接拒绝开始的时机
            if (-not $rejectionStarted -and $currentConnections -gt 12000) {
                $rejectionStarted = $true
                Write-Host "`n[$timestamp] 检测到高连接数，开始重点监控..." -ForegroundColor Red
            }
            
        } catch {
            Write-Host "[$timestamp] 健康检查失败: $($_.Exception.Message)" -ForegroundColor Red
            continue
        }
        
        # 获取TCP连接统计
        $tcpEstablished = (netstat -an | findstr ":$port.*ESTABLISHED").Count
        $tcpTimeWait = (netstat -an | findstr ":$port.*TIME_WAIT").Count
        $tcpCloseWait = (netstat -an | findstr ":$port.*CLOSE_WAIT").Count
        
        # 显示监控信息
        $status = if ($currentConnections -eq $maxConnectionsSeen) { "PEAK" } else { "NORM" }
        $statusColor = if ($status -eq "PEAK") { "Yellow" } else { "Green" }
        $healthIcon = if ($isHealthy) { "✓" } else { "✗" }

        Write-Host "[$timestamp] [$status] $healthIcon TCP连接: $currentConnections | EST=$tcpEstablished, TW=$tcpTimeWait, CW=$tcpCloseWait | 峰值: $maxConnectionsSeen" -ForegroundColor $statusColor
        
        # 检测异常情况
        if ($tcpCloseWait -gt 100) {
            Write-Host "  警告: CLOSE_WAIT 连接过多 ($tcpCloseWait)，可能存在连接泄漏" -ForegroundColor Red
        }
        
        if ($currentConnections -gt 12500 -and $tcpEstablished -lt $currentConnections * 0.8) {
            Write-Host "  警告: 应用连接数与TCP连接数不匹配，可能存在计数错误" -ForegroundColor Red
        }
        
        # 每10次监控显示一次详细统计
        if ($monitorCount % 10 -eq 0) {
            Write-Host "`n--- 第 $monitorCount 次监控统计 ---" -ForegroundColor Cyan
            Write-Host "最大连接数: $maxConnectionsSeen" -ForegroundColor Green
            Write-Host "当前连接数: $currentConnections" -ForegroundColor Green
            Write-Host "连接利用率: $([math]::Round($currentConnections / 25000 * 100, 2))%" -ForegroundColor Green
            
            # 检查内存使用
            if ($gatewayProcess) {
                $gatewayProcess = Get-Process -Id $gatewayProcess.Id -ErrorAction SilentlyContinue
                if ($gatewayProcess) {
                    $memoryMB = [math]::Round($gatewayProcess.WorkingSet64 / 1MB, 2)
                    Write-Host "内存使用: $memoryMB MB" -ForegroundColor Green
                    
                    if ($memoryMB -gt 2048) {
                        Write-Host "  警告: 内存使用过高" -ForegroundColor Red
                    }
                }
            }
            Write-Host ""
        }
    }
} catch {
    Write-Host "`n监控被中断: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "`n监控结束" -ForegroundColor Green
Write-Host "最大连接数: $maxConnectionsSeen" -ForegroundColor Green
