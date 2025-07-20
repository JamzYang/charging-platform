# TCP设置验证脚本
# 用于检查Windows TCP优化设置是否正确应用

Write-Host "=== Windows TCP设置验证 ===" -ForegroundColor Green

# 1. 检查动态端口范围
Write-Host "`n1. 动态端口范围检查" -ForegroundColor Cyan
Write-Host "IPv4 动态端口范围："
$ipv4Ports = netsh int ipv4 show dynamicport tcp
Write-Host $ipv4Ports

Write-Host "`nIPv6 动态端口范围："
$ipv6Ports = netsh int ipv6 show dynamicport tcp
Write-Host $ipv6Ports

# 解析端口范围
$ipv4StartPort = ($ipv4Ports | Select-String "起始端口\s*:\s*(\d+)" | ForEach-Object { $_.Matches[0].Groups[1].Value })
$ipv4PortCount = ($ipv4Ports | Select-String "端口数\s*:\s*(\d+)" | ForEach-Object { $_.Matches[0].Groups[1].Value })

if ($ipv4StartPort -and $ipv4PortCount) {
    $totalPorts = [int]$ipv4PortCount
    if ($totalPorts -ge 50000) {
        Write-Host "✓ IPv4端口范围充足: $totalPorts 个端口" -ForegroundColor Green
    } elseif ($totalPorts -ge 20000) {
        Write-Host "⚠ IPv4端口范围一般: $totalPorts 个端口 (建议≥50000)" -ForegroundColor Yellow
    } else {
        Write-Host "✗ IPv4端口范围不足: $totalPorts 个端口 (建议≥50000)" -ForegroundColor Red
    }
} else {
    Write-Host "⚠ 无法解析IPv4端口范围信息" -ForegroundColor Yellow
}

# 2. 检查TCP全局设置
Write-Host "`n2. TCP全局设置检查" -ForegroundColor Cyan
$tcpGlobal = netsh int tcp show global
Write-Host $tcpGlobal

# 3. 检查注册表设置
Write-Host "`n3. 注册表TCP设置检查" -ForegroundColor Cyan
$regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters"

try {
    $tcpNumConnections = Get-ItemProperty -Path $regPath -Name "TcpNumConnections" -ErrorAction SilentlyContinue
    if ($tcpNumConnections) {
        Write-Host "✓ TcpNumConnections: $($tcpNumConnections.TcpNumConnections)" -ForegroundColor Green
    } else {
        Write-Host "⚠ TcpNumConnections 未设置 (使用系统默认值)" -ForegroundColor Yellow
    }

    $tcpTimedWaitDelay = Get-ItemProperty -Path $regPath -Name "TcpTimedWaitDelay" -ErrorAction SilentlyContinue
    if ($tcpTimedWaitDelay) {
        Write-Host "✓ TcpTimedWaitDelay: $($tcpTimedWaitDelay.TcpTimedWaitDelay) 秒" -ForegroundColor Green
    } else {
        Write-Host "⚠ TcpTimedWaitDelay 未设置 (使用系统默认值)" -ForegroundColor Yellow
    }

    $maxUserPort = Get-ItemProperty -Path $regPath -Name "MaxUserPort" -ErrorAction SilentlyContinue
    if ($maxUserPort) {
        Write-Host "✓ MaxUserPort: $($maxUserPort.MaxUserPort)" -ForegroundColor Green
    } else {
        Write-Host "⚠ MaxUserPort 未设置 (使用系统默认值)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "✗ 无法读取注册表TCP设置: $($_.Exception.Message)" -ForegroundColor Red
}

# 4. 系统资源检查
Write-Host "`n4. 系统资源检查" -ForegroundColor Cyan

# 内存检查
$memory = Get-WmiObject -Class Win32_ComputerSystem
$totalMemoryGB = [math]::Round($memory.TotalPhysicalMemory / 1GB, 2)
Write-Host "总内存: $totalMemoryGB GB"

if ($totalMemoryGB -ge 8) {
    Write-Host "✓ 内存充足，支持高并发测试" -ForegroundColor Green
} elseif ($totalMemoryGB -ge 4) {
    Write-Host "⚠ 内存一般，建议降低并发连接数" -ForegroundColor Yellow
} else {
    Write-Host "✗ 内存不足，不建议进行高并发测试" -ForegroundColor Red
}

# CPU检查
$cpu = Get-WmiObject -Class Win32_Processor
$coreCount = $cpu.NumberOfCores
Write-Host "CPU核心数: $coreCount"

if ($coreCount -ge 4) {
    Write-Host "✓ CPU核心数充足" -ForegroundColor Green
} else {
    Write-Host "⚠ CPU核心数较少，可能影响性能" -ForegroundColor Yellow
}

# 5. 网络连接状态检查
Write-Host "`n5. 当前网络连接状态" -ForegroundColor Cyan
$connections = netstat -an | findstr ":8080\|:8081"
$connectionCount = ($connections | Measure-Object).Count
Write-Host "当前8080/8081端口连接数: $connectionCount"

if ($connectionCount -gt 1000) {
    Write-Host "⚠ 当前连接数较高，可能影响新测试" -ForegroundColor Yellow
} else {
    Write-Host "✓ 当前连接数正常" -ForegroundColor Green
}

# 6. 建议
Write-Host "`n=== 优化建议 ===" -ForegroundColor Green

if ($totalPorts -lt 50000) {
    Write-Host "• 运行 windows-tcp-optimization.ps1 脚本优化TCP设置" -ForegroundColor Yellow
}

if ($totalMemoryGB -lt 8) {
    Write-Host "• 考虑降低并发连接数到 5000 以下" -ForegroundColor Yellow
}

if ($connectionCount -gt 500) {
    Write-Host "• 建议重启应用或等待现有连接释放后再进行测试" -ForegroundColor Yellow
}

Write-Host "`n验证完成！" -ForegroundColor Green
