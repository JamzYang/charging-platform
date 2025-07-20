# 连接监控脚本
# 实时监控网关连接状态，帮助诊断连接拒绝问题

param(
    [string]$HealthURL = "http://localhost:8081/health",
    [int]$Interval = 2
)

Write-Host "网关连接监控脚本" -ForegroundColor Green
Write-Host "健康检查URL: $HealthURL" -ForegroundColor Cyan
Write-Host "监控间隔: $Interval 秒" -ForegroundColor Cyan
Write-Host "按 Ctrl+C 停止监控" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Green

$maxConnections = 0
$rejectionDetected = $false
$startTime = Get-Date

try {
    while ($true) {
        $currentTime = Get-Date
        $elapsed = $currentTime - $startTime

        try {
            # 检查健康状态（简单的OK响应）
            $healthResponse = Invoke-WebRequest -Uri $HealthURL -Method GET -TimeoutSec 3
            $isHealthy = $healthResponse.StatusCode -eq 200 -and $healthResponse.Content -eq "OK"

            # 通过TCP连接数估算应用连接数（因为健康检查不返回连接数）
            $netstatOutput = netstat -an | findstr ":8081"
            $connections = ($netstatOutput | findstr "ESTABLISHED").Count

            # 更新最大连接数
            if ($connections -gt $maxConnections) {
                $maxConnections = $connections
            }

            # 获取TCP连接统计
            $tcpStats = @{}
            $netstatOutput = netstat -an | findstr ":8081"
            $tcpStats.ESTABLISHED = ($netstatOutput | findstr "ESTABLISHED").Count
            $tcpStats.TIME_WAIT = ($netstatOutput | findstr "TIME_WAIT").Count
            $tcpStats.CLOSE_WAIT = ($netstatOutput | findstr "CLOSE_WAIT").Count
            $tcpStats.LISTENING = ($netstatOutput | findstr "LISTENING").Count

            # 检测连接拒绝的临界点
            $status = "NORMAL"
            $color = "Green"

            if ($connections -gt 12000) {
                $status = "HIGH"
                $color = "Yellow"
            }

            if ($connections -gt 12500) {
                $status = "CRITICAL"
                $color = "Red"
                if (-not $rejectionDetected) {
                    $rejectionDetected = $true
                    Write-Host "`n!!! 连接数达到临界值，可能开始拒绝连接 !!!" -ForegroundColor Red
                }
            }

            # 显示监控信息
            $timeStr = $currentTime.ToString("HH:mm:ss")
            $elapsedStr = "{0:mm\:ss}" -f $elapsed
            $healthStatus = if ($isHealthy) { "✓" } else { "✗" }

            Write-Host "[$timeStr] [$status] $healthStatus TCP连接: $connections (峰值: $maxConnections) | EST=$($tcpStats.ESTABLISHED) TW=$($tcpStats.TIME_WAIT) CW=$($tcpStats.CLOSE_WAIT) | 运行: $elapsedStr" -ForegroundColor $color

            # 检测异常情况
            if ($tcpStats.CLOSE_WAIT -gt 50) {
                Write-Host "  ⚠️  CLOSE_WAIT 连接过多: $($tcpStats.CLOSE_WAIT)" -ForegroundColor Red
            }

            if ($connections -gt 0 -and $tcpStats.ESTABLISHED -lt ($connections * 0.7)) {
                Write-Host "  ⚠️  TCP连接数异常低: 应用=$connections vs TCP=$($tcpStats.ESTABLISHED)" -ForegroundColor Red
            }

            # 连接数下降检测
            if ($maxConnections -gt 12000 -and $connections -lt ($maxConnections * 0.9)) {
                Write-Host "  📉 连接数显著下降: 从 $maxConnections 降至 $connections" -ForegroundColor Yellow
            }

        } catch {
            Write-Host "[$($currentTime.ToString('HH:mm:ss'))] ❌ 健康检查失败: $($_.Exception.Message)" -ForegroundColor Red
        }

        Start-Sleep $Interval
    }
} catch {
    Write-Host "`n监控被中断" -ForegroundColor Yellow
}

Write-Host "`n监控结束" -ForegroundColor Green
Write-Host "最大连接数: $maxConnections" -ForegroundColor Green
Write-Host "是否检测到拒绝: $rejectionDetected" -ForegroundColor $(if($rejectionDetected) {"Red"} else {"Green"})
