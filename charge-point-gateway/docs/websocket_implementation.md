# WebSocket实现文档

## 概述

本文档描述了充电桩网关中WebSocket功能的实现，包括架构设计、API接口、使用方法和测试指南。

## 架构设计

### 核心组件

1. **WebSocket管理器 (Manager)**
   - 负责WebSocket连接的生命周期管理
   - 处理连接升级、消息路由和连接清理
   - 提供HTTP服务器和路由处理

2. **连接包装器 (ConnectionWrapper)**
   - 封装单个WebSocket连接
   - 处理消息发送/接收和连接元数据
   - 管理连接状态和活动时间

3. **事件系统 (ConnectionEvent)**
   - 提供连接事件通知机制
   - 支持连接、断开、消息和错误事件
   - 与主应用程序解耦

### 数据流

```
充电桩 <--WebSocket--> 网关 <--Kafka--> 后端系统
                        |
                        v
                     Redis缓存
```

## API接口

### WebSocket端点

#### 连接端点
```
ws://host:port/ocpp/{charge_point_id}
```

**参数:**
- `charge_point_id`: 充电桩唯一标识符

**示例:**
```
ws://localhost:8080/ocpp/CP-001
```

#### 子协议支持
- `ocpp1.6`: OCPP 1.6协议支持

### HTTP端点

#### 健康检查
```
GET /health
```

**响应:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-14T10:00:00Z",
  "connections": 42,
  "uptime": "2h30m15s"
}
```

#### 连接状态
```
GET /connections
```

**响应:**
```json
{
  "total_connections": 2,
  "connections": {
    "CP-001": {
      "last_activity": "2024-01-14T10:00:00Z",
      "connected_at": "2024-01-14T09:30:00Z",
      "remote_addr": "192.168.1.100:54321",
      "subprotocol": "ocpp1.6"
    },
    "CP-002": {
      "last_activity": "2024-01-14T10:00:05Z",
      "connected_at": "2024-01-14T09:45:00Z",
      "remote_addr": "192.168.1.101:54322",
      "subprotocol": "ocpp1.6"
    }
  },
  "timestamp": "2024-01-14T10:00:10Z"
}
```

## 配置

### WebSocket配置
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  websocket_path: "/ocpp"
  read_timeout: "30s"
  write_timeout: "30s"
  max_connections: 1000
```

### 环境变量
```bash
# 服务器配置
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_WEBSOCKET_PATH=/ocpp

# 超时配置
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s

# 连接限制
MAX_CONNECTIONS=1000
```

## 使用方法

### 启动网关
```bash
# 编译
go build -o bin/gateway ./cmd/gateway/

# 启动
./bin/gateway
```

### 客户端连接示例

#### 使用wscat (Node.js)
```bash
# 安装wscat
npm install -g wscat

# 连接到网关
wscat -c ws://localhost:8080/ocpp/CP-001 -s ocpp1.6

# 发送BootNotification消息
[2,"msg001","BootNotification",{"chargePointVendor":"TestVendor","chargePointModel":"TestModel"}]
```

#### 使用Python
```python
import asyncio
import websockets
import json

async def test_connection():
    uri = "ws://localhost:8080/ocpp/CP-001"
    
    async with websockets.connect(uri, subprotocols=["ocpp1.6"]) as websocket:
        # 发送BootNotification
        boot_notification = [
            2,
            "msg001", 
            "BootNotification",
            {
                "chargePointVendor": "TestVendor",
                "chargePointModel": "TestModel"
            }
        ]
        
        await websocket.send(json.dumps(boot_notification))
        response = await websocket.recv()
        print(f"Received: {response}")

asyncio.run(test_connection())
```

#### 使用Go客户端
```go
package main

import (
    "encoding/json"
    "log"
    "github.com/gorilla/websocket"
)

func main() {
    url := "ws://localhost:8080/ocpp/CP-001"
    headers := make(map[string][]string)
    headers["Sec-WebSocket-Protocol"] = []string{"ocpp1.6"}
    
    conn, _, err := websocket.DefaultDialer.Dial(url, headers)
    if err != nil {
        log.Fatal("dial:", err)
    }
    defer conn.Close()
    
    // 发送BootNotification
    bootNotification := []interface{}{
        2,
        "msg001",
        "BootNotification",
        map[string]interface{}{
            "chargePointVendor": "TestVendor",
            "chargePointModel":  "TestModel",
        },
    }
    
    if err := conn.WriteJSON(bootNotification); err != nil {
        log.Fatal("write:", err)
    }
    
    // 读取响应
    var response []interface{}
    if err := conn.ReadJSON(&response); err != nil {
        log.Fatal("read:", err)
    }
    
    log.Printf("Received: %v", response)
}
```

## 测试

### 运行WebSocket测试
```bash
# 运行WebSocket集成测试
go test -v ./test/integration/websocket_integration_test.go

# 使用测试脚本
./scripts/test_websocket.sh

# 运行所有集成测试
make test-integration
```

### 手动测试步骤

1. **启动测试环境**
   ```bash
   docker-compose -f test/docker-compose.test.yml up -d
   ```

2. **启动网关**
   ```bash
   go run ./cmd/gateway/
   ```

3. **测试连接**
   ```bash
   # 健康检查
   curl http://localhost:8080/health
   
   # 连接状态
   curl http://localhost:8080/connections
   
   # WebSocket连接
   wscat -c ws://localhost:8080/ocpp/CP-001
   ```

### 性能测试

#### 并发连接测试
```bash
# 使用测试工具
go test -v ./test/e2e/performance/concurrent_connections_test.go

# 或使用自定义脚本
for i in {1..100}; do
    wscat -c ws://localhost:8080/ocpp/CP-$(printf "%03d" $i) &
done
```

## 故障排除

### 常见问题

1. **连接被拒绝**
   - 检查网关是否启动
   - 验证端口是否正确
   - 确认防火墙设置

2. **消息无响应**
   - 检查OCPP消息格式
   - 验证消息ID唯一性
   - 查看网关日志

3. **连接断开**
   - 检查网络稳定性
   - 验证心跳机制
   - 查看错误日志

### 日志分析

#### 启用调试日志
```bash
export LOG_LEVEL=debug
./bin/gateway
```

#### 关键日志信息
```
# 连接建立
INFO WebSocket connection established for CP-001

# 消息接收
DEBUG Message received from CP-001: [2,"msg001","Heartbeat",{}]

# 连接断开
INFO Charge point CP-001 disconnected

# 错误信息
ERROR WebSocket error for CP-001: connection reset by peer
```

## 监控和指标

### Prometheus指标
- `websocket_connections_total`: 当前连接数
- `websocket_messages_total`: 消息总数
- `websocket_errors_total`: 错误总数
- `websocket_connection_duration`: 连接持续时间

### 健康检查
网关提供健康检查端点用于监控：
```bash
curl http://localhost:8080/health
```

## 安全考虑

1. **认证**: 当前版本未实现认证，生产环境需要添加
2. **授权**: 需要验证充电桩ID的合法性
3. **加密**: 生产环境建议使用WSS (WebSocket Secure)
4. **限流**: 实现连接数和消息频率限制

## 扩展性

### 水平扩展
- 使用Redis存储连接映射
- 支持多个网关实例
- Kafka消息分区策略

### 垂直扩展
- 调整连接池大小
- 优化消息缓冲区
- 增加工作协程数量

## 版本历史

- **v1.0.0**: 基础WebSocket功能实现
- **v1.1.0**: 添加HTTP管理接口
- **v1.2.0**: 集成事件系统和错误处理
- **v1.3.0**: 性能优化和监控支持
