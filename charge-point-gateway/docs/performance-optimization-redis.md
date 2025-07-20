# Redis 连接池优化和高并发性能调优

## 问题分析

在 13000 并发连接压测中发现以下问题：

### 1. Redis 连接池超时
```
{"level":"error","time":"2025-07-20T12:12:46Z","message":"Failed to set connection mapping in Redis for charge point CP-5106: redis: connection pool timeout"}
```

### 2. TCP 连接超时
```
concurrent_connections_test.go:72: Failed to connect CP-5948 (total failed: 1): read tcp4 127.0.0.1:28534->127.0.0.1:8081: i/o timeout
```

## 已实施的优化

### 1. Redis 连接池配置优化

**修改文件**: `configs/application-dev.yaml`
```yaml
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 500          # 增加连接池大小以支持高并发
  min_idle_conns: 50      # 最小空闲连接数
  dial_timeout: "5s"      # 连接超时
  read_timeout: "3s"      # 读取超时
  write_timeout: "3s"     # 写入超时
```

**修改文件**: `internal/storage/redis_storage.go`
- 添加了完整的 Redis 客户端配置参数
- 使用配置文件中的连接池设置

### 2. Grafana 监控增强

**修改文件**: `monitoring/grafana/dashboards/charge-point-gateway.json`
- 添加了 Redis 连接状态监控面板
- 添加了 Redis 性能指标监控面板
- 包含连接数、阻塞连接数、命令执行速率、缓存命中率等关键指标

## 建议的进一步优化

### 1. 系统级 TCP 优化

**Windows 系统优化**:
```powershell
# 增加 TCP 连接数限制
netsh int ipv4 set dynamicport tcp start=1024 num=64511

# 优化 TCP 参数
netsh int tcp set global autotuninglevel=normal
netsh int tcp set global chimney=enabled
netsh int tcp set global rss=enabled
```

**Linux 系统优化**:
```bash
# 增加文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 优化 TCP 参数
echo "net.core.somaxconn = 65536" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 65536" >> /etc/sysctl.conf
echo "net.core.netdev_max_backlog = 5000" >> /etc/sysctl.conf
sysctl -p
```

### 2. 压测参数调优

**建议修改**: `test/e2e/performance/concurrent_connections_test.go`
```go
// 更保守的批次设置
batchSize := 50                      // 减小批次大小
batchDelay := 200 * time.Millisecond // 增加批次间延迟

// 分阶段压测
connectionCount := 10000  // 先测试 10k 连接
```

### 3. Redis 服务器优化

**Redis 配置优化**:
```conf
# redis.conf
maxclients 10000
tcp-keepalive 300
timeout 0
tcp-backlog 511
```

### 4. 应用层优化

**WebSocket 连接管理**:
- 实现连接池复用
- 优化心跳机制
- 添加连接重试逻辑

**内存管理**:
- 监控 Goroutine 数量
- 实现连接清理机制
- 优化消息缓冲区

## 监控指标

### Redis 关键指标
- `redis_connected_clients`: 当前连接数
- `redis_blocked_clients`: 阻塞连接数
- `redis_commands_total`: 命令执行总数
- `redis_keyspace_hits_total`: 缓存命中数
- `redis_keyspace_misses_total`: 缓存未命中数

### 应用指标
- `websocket_connections_total`: WebSocket 连接总数
- `message_processing_duration`: 消息处理延迟
- `go_goroutines`: Goroutine 数量
- `go_memstats_alloc_bytes`: 内存使用量

## 测试建议

### 1. 分阶段压测
1. 1000 连接 - 验证基本功能
2. 5000 连接 - 验证中等负载
3. 10000 连接 - 验证高负载
4. 15000+ 连接 - 验证极限负载

### 2. 监控重点
- Redis 连接池使用率
- TCP 连接状态
- 内存使用趋势
- CPU 使用率
- 网络 I/O

### 3. 故障排查
- 检查 Redis 日志
- 监控系统资源
- 分析网络连接状态
- 查看应用日志

## 预期效果

通过以上优化，预期能够：
- 支持 10000+ 并发连接
- Redis 连接池超时错误减少 90%+
- TCP 连接成功率提升至 95%+
- 整体系统稳定性显著提升
