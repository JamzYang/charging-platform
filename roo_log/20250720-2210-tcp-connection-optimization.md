# TCP 连接优化分析和解决方案

## 问题分析

### 当前测试结果
- **总连接数**: 13000
- **成功连接**: 10839 (83.38%)
- **失败连接**: 2161 (16.62%)
- **主要错误**: `dial tcp4 127.0.0.1:8081: connectex: A connection attempt failed because the connected party did not properly respond after a period of time`

### 根本原因分析

#### 1. TCP 监听队列溢出
- **问题**: Go 默认使用系统的 TCP 监听队列大小（通常 128-512）
- **影响**: 当大量连接同时建立时，监听队列溢出导致连接被拒绝
- **表现**: `connectex` 超时错误

#### 2. 批次处理过于激进
- **当前设置**: 100 连接/批次，100ms 间隔
- **问题**: 瞬时连接压力过大，超过系统处理能力
- **计算**: 100 连接 × 10 批次/秒 = 1000 连接/秒的瞬时压力

#### 3. 系统资源限制
- **Windows 默认限制**: 动态端口范围约 16k
- **连接状态**: TIME_WAIT 状态占用端口资源
- **内存压力**: 13000 个 Goroutine + WebSocket 连接

## 已实施的优化

### 1. 测试参数优化
```go
// 修改前
batchSize := 100                     // 100 连接/批次
batchDelay := 100 * time.Millisecond // 100ms 间隔

// 修改后
batchSize := 50                      // 50 连接/批次
batchDelay := 200 * time.Millisecond // 200ms 间隔
```

**效果预期**: 
- 瞬时连接压力从 1000/秒 降低到 250/秒
- 给系统更多时间处理连接队列

### 2. 服务器优化
```go
// 添加了优化的 HTTP 服务器配置
server := &http.Server{
    Handler:        mainMux,
    ReadTimeout:    cfg.Server.ReadTimeout,
    WriteTimeout:   cfg.Server.WriteTimeout,
    IdleTimeout:    120 * time.Second,  // 增加空闲超时
    MaxHeaderBytes: 1 << 20,            // 1MB 头部限制
}
```

### 3. Redis 连接池优化
```yaml
redis:
  pool_size: 500          # 增加到 500
  min_idle_conns: 50      # 最小空闲连接
  dial_timeout: "5s"
  read_timeout: "3s"
  write_timeout: "3s"
```

## 进一步优化建议

### 1. 系统级优化（必须）

#### Windows TCP 优化
```powershell
# 运行管理员权限的 PowerShell
.\scripts\windows-tcp-optimization.ps1
```

**关键设置**:
- 动态端口范围: 1024-65535 (64k 端口)
- TCP 监听队列: 增加到 1000+
- TIME_WAIT 超时: 减少到 30 秒

#### 验证优化效果
```powershell
# 检查动态端口范围
netsh int ipv4 show dynamicport tcp

# 检查 TCP 设置
netsh int tcp show global

# 监控连接状态
netstat -an | findstr ":8081" | measure-object
```

### 2. 应用层优化

#### 分阶段测试策略
```go
// 建议的测试阶梯
var testStages = []struct {
    connections int
    batchSize   int
    batchDelay  time.Duration
}{
    {1000,  25,  100 * time.Millisecond}, // 热身阶段
    {5000,  50,  150 * time.Millisecond}, // 中等负载
    {10000, 50,  200 * time.Millisecond}, // 高负载
    {13000, 40,  250 * time.Millisecond}, // 极限负载
}
```

#### 连接重试机制
```go
func connectWithRetry(gatewayURL, chargePointID string, maxRetries int) (*WebSocketClient, error) {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        client, err := utils.NewWebSocketClient(gatewayURL, chargePointID)
        if err == nil {
            return client, nil
        }
        lastErr = err
        
        // 指数退避
        backoff := time.Duration(i+1) * 100 * time.Millisecond
        time.Sleep(backoff)
    }
    return nil, lastErr
}
```

### 3. 监控和诊断

#### 关键监控指标
- **TCP 连接状态分布**: ESTABLISHED, TIME_WAIT, SYN_SENT
- **Redis 连接池使用率**: 活跃连接数 / 总连接数
- **Goroutine 数量**: 避免 Goroutine 泄漏
- **内存使用**: 监控内存增长趋势

#### 实时监控命令
```powershell
# 监控 TCP 连接状态
while ($true) {
    $established = (netstat -an | findstr ":8081.*ESTABLISHED").Count
    $timeWait = (netstat -an | findstr ":8081.*TIME_WAIT").Count
    Write-Host "$(Get-Date): ESTABLISHED=$established, TIME_WAIT=$timeWait"
    Start-Sleep 5
}
```

### 4. 硬件资源优化

#### 内存要求
- **基础内存**: 2GB
- **每 1000 连接**: 额外 200-300MB
- **13000 连接**: 建议 8GB+ 内存

#### CPU 要求
- **最低**: 4 核心
- **推荐**: 8 核心（支持更高并发）

## 预期优化效果

### 优化前 vs 优化后

| 指标 | 优化前 | 优化后（预期） |
|------|--------|----------------|
| 连接成功率 | 83.38% | 95%+ |
| 失败连接数 | 2161 | <650 |
| 瞬时连接压力 | 1000/秒 | 250/秒 |
| Redis 超时错误 | 频繁 | 极少 |

### 测试验证步骤

1. **运行系统优化**
   ```powershell
   .\scripts\optimize-for-performance-test.ps1
   ```

2. **重启系统**（应用 TCP 优化）

3. **分阶段测试**
   ```bash
   # 测试 5000 连接
   go test -v -run TestTC_E2E_04_ConcurrentConnections -timeout 20m
   ```

4. **监控关键指标**
   - 访问 Grafana: http://localhost:3000
   - 查看 Redis 监控面板
   - 监控系统资源使用

## 故障排查指南

### 常见问题和解决方案

#### 1. 连接超时持续发生
- 检查防火墙设置
- 验证端口可用性
- 确认服务器正在监听正确端口

#### 2. Redis 连接池超时
- 增加 Redis 连接池大小
- 检查 Redis 服务器性能
- 优化 Redis 配置

#### 3. 内存不足
- 监控 Goroutine 泄漏
- 实现连接清理机制
- 增加系统内存

#### 4. CPU 使用率过高
- 减少批次大小
- 增加批次间延迟
- 优化消息处理逻辑

## 总结

通过系统级 TCP 优化、应用层参数调优和分阶段测试策略，预期能够将连接成功率从 83.38% 提升到 95% 以上，支持 13000+ 并发连接的稳定运行。

关键成功因素：
1. **系统级 TCP 优化**（最重要）
2. **合理的批次处理策略**
3. **充足的硬件资源**
4. **实时监控和调优**
