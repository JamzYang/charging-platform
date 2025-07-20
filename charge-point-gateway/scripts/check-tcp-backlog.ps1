# TCP 监听队列诊断脚本
# 检查 TCP backlog 和连接状态

param(
    [int]$Port = 8081
)

Write-Host "TCP 监听队列诊断脚本" -ForegroundColor Green
Write-Host "检查端口: $Port" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Green

# 1. 检查端口监听状态
Write-Host "`n1. 检查端口监听状态..." -ForegroundColor Yellow

$listening = netstat -an | findstr ":$Port.*LISTENING"
if ($listening) {
    Write-Host "端口 $Port 正在监听: $listening" -ForegroundColor Green
} else {
    Write-Host "端口 $Port 没有在监听" -ForegroundColor Red
    exit 1
}

# 2. 统计各种连接状态
Write-Host "`n2. TCP 连接状态统计..." -ForegroundColor Yellow

$netstatOutput = netstat -an | findstr ":$Port"
$stats = @{
    LISTENING = ($netstatOutput | findstr "LISTENING").Count
    ESTABLISHED = ($netstatOutput | findstr "ESTABLISHED").Count
    SYN_SENT = ($netstatOutput | findstr "SYN_SENT").Count
    SYN_RECEIVED = ($netstatOutput | findstr "SYN_RECEIVED").Count
    TIME_WAIT = ($netstatOutput | findstr "TIME_WAIT").Count
    CLOSE_WAIT = ($netstatOutput | findstr "CLOSE_WAIT").Count
    FIN_WAIT1 = ($netstatOutput | findstr "FIN_WAIT1").Count
    FIN_WAIT2 = ($netstatOutput | findstr "FIN_WAIT2").Count
    CLOSING = ($netstatOutput | findstr "CLOSING").Count
}

foreach ($state in $stats.Keys | Sort-Object) {
    $count = $stats[$state]
    $color = switch ($state) {
        "ESTABLISHED" { "Green" }
        "TIME_WAIT" { if ($count -gt 1000) { "Yellow" } else { "Green" } }
        "CLOSE_WAIT" { if ($count -gt 50) { "Red" } else { "Green" } }
        "SYN_SENT" { if ($count -gt 100) { "Red" } else { "Yellow" } }
        "SYN_RECEIVED" { if ($count -gt 100) { "Red" } else { "Yellow" } }
        default { "Gray" }
    }
    Write-Host "  $state`: $count" -ForegroundColor $color
}

# 3. 检查系统 TCP 设置
Write-Host "`n3. 检查系统 TCP 设置..." -ForegroundColor Yellow

Write-Host "动态端口范围:" -ForegroundColor Cyan
try {
    $portRange = netsh int ipv4 show dynamicport tcp
    $portRange | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
} catch {
    Write-Host "  无法获取动态端口范围" -ForegroundColor Red
}

Write-Host "`nTCP 全局设置:" -ForegroundColor Cyan
try {
    $tcpSettings = netsh int tcp show global
    $tcpSettings | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
} catch {
    Write-Host "  无法获取 TCP 全局设置" -ForegroundColor Red
}

# 4. 检查进程资源使用
Write-Host "`n4. 检查网关进程资源..." -ForegroundColor Yellow

$gatewayProcess = Get-Process -Name "gateway*" -ErrorAction SilentlyContinue
if ($gatewayProcess) {
    Write-Host "网关进程信息:" -ForegroundColor Cyan
    Write-Host "  PID: $($gatewayProcess.Id)" -ForegroundColor Green
    Write-Host "  CPU: $($gatewayProcess.CPU)" -ForegroundColor Green
    Write-Host "  内存: $([math]::Round($gatewayProcess.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor Green
    Write-Host "  句柄数: $($gatewayProcess.HandleCount)" -ForegroundColor Green
    Write-Host "  线程数: $($gatewayProcess.Threads.Count)" -ForegroundColor Green
    
    # 检查句柄数是否过高
    if ($gatewayProcess.HandleCount -gt 10000) {
        Write-Host "  ⚠️  句柄数过高，可能存在资源泄漏" -ForegroundColor Red
    }
    
    # 检查内存使用
    $memoryMB = [math]::Round($gatewayProcess.WorkingSet64 / 1MB, 2)
    if ($memoryMB -gt 2048) {
        Write-Host "  ⚠️  内存使用过高: $memoryMB MB" -ForegroundColor Red
    }
} else {
    Write-Host "未找到网关进程" -ForegroundColor Red
}

# 5. 检查系统资源限制
Write-Host "`n5. 检查系统资源限制..." -ForegroundColor Yellow

# 检查可用内存
$memory = Get-WmiObject -Class Win32_OperatingSystem
$freeMemoryMB = [math]::Round($memory.FreePhysicalMemory / 1024, 2)
$totalMemoryMB = [math]::Round($memory.TotalVisibleMemorySize / 1024, 2)
$memoryUsagePercent = [math]::Round(($totalMemoryMB - $freeMemoryMB) / $totalMemoryMB * 100, 2)

Write-Host "系统内存:" -ForegroundColor Cyan
Write-Host "  总内存: $totalMemoryMB MB" -ForegroundColor Green
Write-Host "  可用内存: $freeMemoryMB MB" -ForegroundColor Green
Write-Host "  使用率: $memoryUsagePercent%" -ForegroundColor $(if($memoryUsagePercent -gt 90) {"Red"} elseif($memoryUsagePercent -gt 80) {"Yellow"} else {"Green"})

# 6. 诊断建议
Write-Host "`n6. 诊断建议..." -ForegroundColor Yellow

$issues = @()
$suggestions = @()

# 检查连接状态异常
if ($stats.SYN_SENT -gt 100) {
    $issues += "SYN_SENT 连接过多 ($($stats.SYN_SENT))，可能是连接超时"
    $suggestions += "检查网络连接和服务器响应能力"
}

if ($stats.CLOSE_WAIT -gt 50) {
    $issues += "CLOSE_WAIT 连接过多 ($($stats.CLOSE_WAIT))，可能存在连接泄漏"
    $suggestions += "检查应用程序的连接关闭逻辑"
}

if ($stats.TIME_WAIT -gt 2000) {
    $issues += "TIME_WAIT 连接过多 ($($stats.TIME_WAIT))，端口资源紧张"
    $suggestions += "调整 TCP TIME_WAIT 超时时间"
}

if ($stats.ESTABLISHED -gt 10000) {
    $issues += "ESTABLISHED 连接数很高 ($($stats.ESTABLISHED))"
    $suggestions += "这可能接近系统或应用程序的连接限制"
}

if ($memoryUsagePercent -gt 90) {
    $issues += "系统内存使用率过高 ($memoryUsagePercent%)"
    $suggestions += "考虑增加系统内存或优化应用程序内存使用"
}

if ($issues.Count -eq 0) {
    Write-Host "✅ 未发现明显问题" -ForegroundColor Green
} else {
    Write-Host "⚠️  发现的问题:" -ForegroundColor Red
    foreach ($issue in $issues) {
        Write-Host "  - $issue" -ForegroundColor Red
    }
    
    Write-Host "`n💡 建议的解决方案:" -ForegroundColor Yellow
    foreach ($suggestion in $suggestions) {
        Write-Host "  - $suggestion" -ForegroundColor Yellow
    }
}

# 7. TCP Backlog 相关建议
Write-Host "`n7. TCP Backlog 优化建议..." -ForegroundColor Yellow

Write-Host "当前问题分析:" -ForegroundColor Cyan
Write-Host "  - 客户端连接超时但服务端无日志" -ForegroundColor Gray
Write-Host "  - 这通常表示连接请求在内核层被丢弃" -ForegroundColor Gray
Write-Host "  - 最可能的原因是 TCP 监听队列(backlog)已满" -ForegroundColor Gray

Write-Host "`n建议的解决方案:" -ForegroundColor Cyan
Write-Host "  1. 增加应用程序的监听队列大小" -ForegroundColor Yellow
Write-Host "  2. 优化系统 TCP 参数" -ForegroundColor Yellow
Write-Host "  3. 减少压测的并发连接速度" -ForegroundColor Yellow

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "诊断完成" -ForegroundColor Green
