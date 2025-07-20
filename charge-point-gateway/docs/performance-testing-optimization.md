# 性能测试优化指南

## 问题分析

### 错误现象
```
dial tcp [::1]:8081: bind: An operation on a socket could not be performed because the system lacked sufficient buffer space or because a queue was full.
```

### 根本原因
1. **Windows系统TCP连接限制**：默认动态端口范围约16k
2. **IPv6连接问题**：使用IPv6可能导致额外的连接开销
3. **连接风暴**：同时建立大量连接导致系统资源耗尽
4. **TCP缓冲区不足**：系统级TCP缓冲区配置不当

## 解决方案

### 1. 系统级优化（Windows）

#### 运行TCP优化脚本
```powershell
# 以管理员身份运行
.\scripts\windows-tcp-optimization.ps1
```

#### 手动优化步骤
```powershell
# 1. 增加动态端口范围
netsh int ipv4 set dynamicport tcp start=1024 num=60000
netsh int ipv6 set dynamicport tcp start=1024 num=60000

# 2. 调整TCP参数
netsh int tcp set global autotuninglevel=normal
netsh int tcp set global chimney=enabled
netsh int tcp set global rss=enabled

# 3. 查看当前设置
netsh int ipv4 show dynamicport tcp
netsh int tcp show global
```

### 2. 应用层优化

#### WebSocket客户端优化
- **强制使用IPv4**：避免IPv6连接开销
- **TCP参数优化**：启用NoDelay、KeepAlive
- **连接池管理**：合理设置缓冲区大小

#### 批次连接策略
- **批次大小**：100个连接/批次
- **批次延迟**：50ms批次间延迟
- **进度监控**：每1000个连接记录进度

### 3. 测试参数调优

#### 推荐配置
```go
// 并发连接测试
connectionCount := 10000
batchSize := 100
batchDelay := 50 * time.Millisecond
testDuration := 20 * time.Minute

// WebSocket配置
ReadBufferSize:   4096
WriteBufferSize:  4096
HandshakeTimeout: 10 * time.Second
```

#### 监控指标
- 成功连接率：>80%
- 消息成功率：>90%
- 连接建立时间：<10s
- 内存使用：<2GB

### 4. 故障排除

#### 常见错误及解决方案

1. **端口耗尽**
   ```
   dial tcp: bind: An operation on a socket could not be performed...
   ```
   - 解决：增加动态端口范围
   - 验证：`netsh int ipv4 show dynamicport tcp`

2. **连接超时**
   ```
   dial tcp: i/o timeout
   ```
   - 解决：增加HandshakeTimeout
   - 优化：使用批次连接策略

3. **内存不足**
   ```
   runtime: out of memory
   ```
   - 解决：减少并发连接数
   - 优化：启用连接复用

#### 性能基准
| 连接数 | 预期成功率 | 内存使用 | 建立时间 |
|--------|------------|----------|----------|
| 1,000  | >95%       | <500MB   | <5s      |
| 5,000  | >90%       | <1GB     | <15s     |
| 10,000 | >80%       | <2GB     | <30s     |

### 5. 验证步骤

#### 1. 系统优化验证
```powershell
# 检查动态端口范围
netsh int ipv4 show dynamicport tcp

# 检查TCP设置
netsh int tcp show global
```

#### 2. 运行性能测试
```bash
# 运行并发连接测试
go test -v ./test/e2e/performance/concurrent_connections_test.go -timeout 30m

# 监控系统资源
# 任务管理器 -> 性能 -> 网络/内存
```

#### 3. 结果分析
- 检查连接成功率
- 监控系统资源使用
- 分析错误日志模式

### 6. 最佳实践

#### 开发环境
- 连接数：1,000-5,000
- 批次大小：50-100
- 测试时长：5-10分钟

#### 生产环境模拟
- 连接数：10,000-50,000
- 批次大小：100-200
- 测试时长：20-60分钟

#### 监控建议
- 使用Prometheus监控连接数
- 配置Grafana仪表板
- 设置告警阈值

## 注意事项

1. **重启要求**：系统级优化后建议重启
2. **权限要求**：TCP优化需要管理员权限
3. **环境差异**：不同Windows版本可能有差异
4. **资源监控**：持续监控CPU、内存、网络使用率
