# Windows TCP 连接优化脚本
# 用于解决高并发连接测试中的系统限制问题

Write-Host "Windows TCP连接优化脚本" -ForegroundColor Green
Write-Host "注意：需要管理员权限运行" -ForegroundColor Yellow

# 检查管理员权限
if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Host "错误：需要管理员权限运行此脚本" -ForegroundColor Red
    exit 1
}

Write-Host "1. 增加动态端口范围..." -ForegroundColor Cyan
# 增加动态端口范围（默认约16k，增加到60k）
netsh int ipv4 set dynamicport tcp start=1024 num=60000
netsh int ipv6 set dynamicport tcp start=1024 num=60000

Write-Host "2. 调整TCP参数..." -ForegroundColor Cyan
# 增加TCP连接数限制
netsh int ipv4 set global autotuninglevel=normal
netsh int ipv6 set global autotuninglevel=normal

# 调整TCP窗口大小
netsh int tcp set global autotuninglevel=normal
netsh int tcp set global chimney=enabled
netsh int tcp set global rss=enabled
netsh int tcp set global netdma=enabled

Write-Host "3. 调整注册表设置..." -ForegroundColor Cyan
# 增加TCP连接数限制
$regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters"

# 最大连接数
Set-ItemProperty -Path $regPath -Name "TcpNumConnections" -Value 65534 -Type DWord -Force

# TCP时间等待超时（减少TIME_WAIT状态持续时间）
Set-ItemProperty -Path $regPath -Name "TcpTimedWaitDelay" -Value 30 -Type DWord -Force

# 最大用户端口
Set-ItemProperty -Path $regPath -Name "MaxUserPort" -Value 65534 -Type DWord -Force

# TCP连接超时
Set-ItemProperty -Path $regPath -Name "TcpMaxConnectRetransmissions" -Value 2 -Type DWord -Force

# 增加TCP监听队列大小
Set-ItemProperty -Path $regPath -Name "TcpMaxHalfOpen" -Value 1000 -Type DWord -Force
Set-ItemProperty -Path $regPath -Name "TcpMaxHalfOpenRetried" -Value 500 -Type DWord -Force

# 优化TCP窗口缩放
Set-ItemProperty -Path $regPath -Name "Tcp1323Opts" -Value 3 -Type DWord -Force

# 启用TCP快速打开（如果支持）
Set-ItemProperty -Path $regPath -Name "TcpMaxDupAcks" -Value 2 -Type DWord -Force

Write-Host "4. 调整应用程序特定设置..." -ForegroundColor Cyan
# 增加应用程序可用的套接字缓冲区
$regPathWinsock = "HKLM:\SYSTEM\CurrentControlSet\Services\AFD\Parameters"

# 增加非分页池内存
Set-ItemProperty -Path $regPathWinsock -Name "LargeBufferSize" -Value 65536 -Type DWord -Force
Set-ItemProperty -Path $regPathWinsock -Name "MediumBufferSize" -Value 8192 -Type DWord -Force
Set-ItemProperty -Path $regPathWinsock -Name "SmallBufferSize" -Value 512 -Type DWord -Force

# 增加缓冲区数量
Set-ItemProperty -Path $regPathWinsock -Name "LargeBufferListDepth" -Value 256 -Type DWord -Force
Set-ItemProperty -Path $regPathWinsock -Name "MediumBufferListDepth" -Value 512 -Type DWord -Force
Set-ItemProperty -Path $regPathWinsock -Name "SmallBufferListDepth" -Value 1024 -Type DWord -Force

Write-Host "5. 显示当前设置..." -ForegroundColor Cyan
Write-Host "动态端口范围："
netsh int ipv4 show dynamicport tcp
netsh int ipv6 show dynamicport tcp

Write-Host "TCP全局设置："
netsh int tcp show global

Write-Host "优化完成！" -ForegroundColor Green
Write-Host "建议重启系统以确保所有设置生效" -ForegroundColor Yellow
Write-Host "重启后可以运行性能测试验证效果" -ForegroundColor Yellow
