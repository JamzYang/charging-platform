# 性能测试优化脚本
# 用于在运行高并发连接测试前优化系统设置

param(
    [switch]$SkipTcpOptimization,
    [switch]$SkipRedisOptimization,
    [switch]$ShowCurrentSettings
)

Write-Host "充电桩网关性能测试优化脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

if ($ShowCurrentSettings) {
    Write-Host "当前系统设置:" -ForegroundColor Cyan
    
    Write-Host "`n动态端口范围:" -ForegroundColor Yellow
    netsh int ipv4 show dynamicport tcp
    
    Write-Host "`nTCP全局设置:" -ForegroundColor Yellow
    netsh int tcp show global
    
    Write-Host "`n当前连接统计:" -ForegroundColor Yellow
    netstat -an | findstr ":8080" | measure-object | select-object Count
    netstat -an | findstr ":6379" | measure-object | select-object Count
    
    exit 0
}

# 1. TCP 优化
if (-not $SkipTcpOptimization) {
    Write-Host "`n1. 执行 TCP 连接优化..." -ForegroundColor Cyan
    
    # 检查管理员权限
    if (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
        Write-Host "警告：需要管理员权限进行 TCP 优化，跳过此步骤" -ForegroundColor Yellow
        Write-Host "请以管理员身份运行: .\scripts\windows-tcp-optimization.ps1" -ForegroundColor Yellow
    } else {
        & ".\scripts\windows-tcp-optimization.ps1"
    }
} else {
    Write-Host "跳过 TCP 优化" -ForegroundColor Yellow
}

# 2. Redis 优化检查
if (-not $SkipRedisOptimization) {
    Write-Host "`n2. 检查 Redis 配置..." -ForegroundColor Cyan
    
    $configFile = ".\configs\application-dev.yaml"
    if (Test-Path $configFile) {
        $content = Get-Content $configFile -Raw
        
        # 检查 Redis 连接池配置
        if ($content -match "pool_size:\s*(\d+)") {
            $poolSize = $matches[1]
            Write-Host "Redis 连接池大小: $poolSize" -ForegroundColor Green
            
            if ([int]$poolSize -lt 500) {
                Write-Host "建议：增加 Redis 连接池大小到 500+" -ForegroundColor Yellow
            }
        } else {
            Write-Host "警告：未找到 Redis 连接池配置" -ForegroundColor Red
            Write-Host "建议添加以下配置到 $configFile:" -ForegroundColor Yellow
            Write-Host @"
redis:
  pool_size: 500
  min_idle_conns: 50
  dial_timeout: "5s"
  read_timeout: "3s"
  write_timeout: "3s"
"@ -ForegroundColor Gray
        }
    } else {
        Write-Host "警告：未找到配置文件 $configFile" -ForegroundColor Red
    }
} else {
    Write-Host "跳过 Redis 优化检查" -ForegroundColor Yellow
}

# 3. 系统资源检查
Write-Host "`n3. 检查系统资源..." -ForegroundColor Cyan

# 检查可用内存
$memory = Get-WmiObject -Class Win32_OperatingSystem
$freeMemoryGB = [math]::Round($memory.FreePhysicalMemory / 1MB, 2)
$totalMemoryGB = [math]::Round($memory.TotalVisibleMemorySize / 1MB, 2)

Write-Host "可用内存: $freeMemoryGB GB / $totalMemoryGB GB" -ForegroundColor Green

if ($freeMemoryGB -lt 4) {
    Write-Host "警告：可用内存不足 4GB，可能影响高并发测试" -ForegroundColor Yellow
}

# 检查 CPU 使用率
$cpu = Get-WmiObject -Class Win32_Processor | Measure-Object -Property LoadPercentage -Average
$cpuUsage = $cpu.Average

Write-Host "当前 CPU 使用率: $cpuUsage%" -ForegroundColor Green

if ($cpuUsage -gt 50) {
    Write-Host "警告：当前 CPU 使用率较高，建议关闭不必要的程序" -ForegroundColor Yellow
}

# 4. 进程检查
Write-Host "`n4. 检查相关进程..." -ForegroundColor Cyan

# 检查 Redis 进程
$redisProcess = Get-Process -Name "redis-server" -ErrorAction SilentlyContinue
if ($redisProcess) {
    Write-Host "Redis 服务运行中 (PID: $($redisProcess.Id))" -ForegroundColor Green
} else {
    Write-Host "警告：Redis 服务未运行" -ForegroundColor Yellow
    Write-Host "请启动 Redis 服务或使用 Docker: docker run -d -p 6379:6379 redis:7-alpine" -ForegroundColor Gray
}

# 检查网关进程
$gatewayProcess = Get-Process -Name "gateway*" -ErrorAction SilentlyContinue
if ($gatewayProcess) {
    Write-Host "网关服务运行中 (PID: $($gatewayProcess.Id))" -ForegroundColor Green
} else {
    Write-Host "网关服务未运行，这是正常的（测试时会启动）" -ForegroundColor Gray
}

# 5. 网络连接检查
Write-Host "`n5. 检查网络连接..." -ForegroundColor Cyan

# 检查端口占用
$ports = @(8080, 8081, 6379, 9090, 9091)
foreach ($port in $ports) {
    $connection = netstat -an | findstr ":$port "
    if ($connection) {
        Write-Host "端口 $port 已被占用" -ForegroundColor Yellow
    } else {
        Write-Host "端口 $port 可用" -ForegroundColor Green
    }
}

# 6. 测试建议
Write-Host "`n6. 性能测试建议..." -ForegroundColor Cyan

Write-Host @"
建议的测试步骤：

1. 分阶段测试：
   - 1000 连接：验证基本功能
   - 5000 连接：验证中等负载  
   - 10000 连接：验证高负载
   - 13000+ 连接：验证极限负载

2. 监控指标：
   - 启动监控：docker-compose -f test/docker-compose.test.yml --profile monitoring up -d
   - 访问 Grafana：http://localhost:3000 (admin/admin)
   - 查看 Redis 监控面板

3. 运行测试：
   cd test/e2e/performance
   go test -v -run TestTC_E2E_04_ConcurrentConnections -timeout 30m

4. 故障排查：
   - 检查应用日志
   - 监控 Redis 连接数
   - 查看系统资源使用
   - 分析网络连接状态

"@ -ForegroundColor Gray

Write-Host "`n优化完成！" -ForegroundColor Green
Write-Host "如果进行了 TCP 优化，建议重启系统后再运行测试" -ForegroundColor Yellow
