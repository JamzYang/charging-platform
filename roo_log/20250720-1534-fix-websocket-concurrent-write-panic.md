# 20250720-1534 修复WebSocket并发写入导致Gateway服务崩溃问题

## 问题描述

在进行12000并发连接压测时，Gateway服务在运行约51秒后自动停止，出现以下错误：

```
panic: concurrent write to websocket connection

goroutine 98893 [running]:
github.com/gorilla/websocket.(*messageWriter).flushFrame(0xc001d92ec0, 0x1, {0xc01ad90369?, 0xc002a6ab60?, 0xc002a6ab60?})
    /go/pkg/mod/github.com/gorilla/websocket@v1.5.3/conn.go:617 +0x4b8
github.com/gorilla/websocket.(*Conn).WriteMessage(0xc0041a29a0, 0x3d45ac0309?, {0xc01ad90320, 0x49, 0x50})
    /go/pkg/mod/github.com/gorilla/websocket@v1.5.3/conn.go:770 +0x126
github.com/charging-platform/charge-point-gateway/internal/transport/websocket.(*ConnectionWrapper).sendRoutine(0xc002880bd0)
    /app/internal/transport/websocket/manager.go:628 +0x169
```

同时伴随大量事件通道满载警告：
```
Event channel full, dropping event
Event channel full, dropping message event
```

## 根本原因分析

### 1. WebSocket并发写入冲突

**核心问题**: `sendRoutine()` 和 `pingRoutine()` 两个协程同时直接调用 `w.conn.WriteMessage()`，违反了WebSocket连接的线程安全要求。

```go
// sendRoutine 发送协程
func (w *ConnectionWrapper) sendRoutine() {
    // 直接写入WebSocket连接
    if err := w.conn.WriteMessage(websocket.TextMessage, message); err != nil {
        // ...
    }
}

// pingRoutine ping协程  
func (w *ConnectionWrapper) pingRoutine() {
    // 同时直接写入WebSocket连接 - 导致并发冲突！
    if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
        // ...
    }
}
```

**违反架构原则**: 根据架构设计文档，所有WebSocket写入操作应该通过统一的 `sendRoutine()` 处理，确保单一写入协程原则。

### 2. 事件通道容量不足

**配置问题**: 
- 事件通道容量：100（严重不足）
- 发送通道容量：100（每个连接，不足以应对突发流量）

在12000个并发连接下，这些容量配置完全无法满足高并发需求。

### 3. 错误的ping消息处理逻辑

```go
// 错误的ping处理
select {
case w.sendChan <- nil: // 错误：发送nil到消息通道
default:
}
```

ping消息不应该发送到普通消息通道，而且发送 `nil` 到 `[]byte` 通道是错误的。

## 解决方案

### 1. 统一WebSocket写入机制

引入 `WebSocketMessage` 类型，确保所有写入操作都通过 `sendRoutine()` 处理：

```go
// MessageType 消息类型枚举
type MessageType int

const (
    MessageTypeText MessageType = iota
    MessageTypePing
    MessageTypePong
)

// WebSocketMessage WebSocket消息结构
type WebSocketMessage struct {
    Type MessageType
    Data []byte
}

// 修改连接包装器
type ConnectionWrapper struct {
    sendChan chan WebSocketMessage  // 统一消息通道
    // ...
}
```

### 2. 修复sendRoutine统一处理

```go
// sendRoutine 发送协程 - 统一处理所有WebSocket写入操作
func (w *ConnectionWrapper) sendRoutine() {
    for {
        select {
        case wsMessage := <-w.sendChan:
            // 根据消息类型选择对应的WebSocket消息类型
            var err error
            switch wsMessage.Type {
            case MessageTypeText:
                err = w.conn.WriteMessage(websocket.TextMessage, wsMessage.Data)
            case MessageTypePing:
                err = w.conn.WriteMessage(websocket.PingMessage, wsMessage.Data)
            case MessageTypePong:
                err = w.conn.WriteMessage(websocket.PongMessage, wsMessage.Data)
            }
            
            if err != nil {
                w.logger.Errorf("Failed to send message: %v", err)
                return
            }
        }
    }
}
```

### 3. 修复pingRoutine

```go
// pingRoutine ping协程 - 通过sendRoutine统一发送ping消息
func (w *ConnectionWrapper) pingRoutine() {
    ticker := time.NewTicker(w.config.PingInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // 通过sendChan发送ping消息
            pingMsg := WebSocketMessage{
                Type: MessageTypePing,
                Data: nil,
            }
            
            select {
            case w.sendChan <- pingMsg:
                // ping消息已发送到队列
            default:
                // 如果发送队列满了，记录警告但不阻塞
                w.logger.Warnf("Failed to send ping: send channel full")
            }
        }
    }
}
```

### 4. 增加通道容量

```go
// 增加事件通道容量
eventChan: make(chan ConnectionEvent, 10000),  // 从100增加到10000

// 增加发送通道容量  
sendChan: make(chan WebSocketMessage, 1000),   // 从100增加到1000
```

## 修复效果

### 修复前：
- **连接成功率**: 77.47% (9296/12000)
- **消息成功率**: 78.89% (34158/43300)
- **测试时长**: 51.7秒 (提前崩溃)
- **崩溃原因**: `panic: concurrent write to websocket connection`

### 修复后：
- **连接成功率**: 99.08% (11890/12000) ✅
- **消息成功率**: 98.36% (627184/637652) ✅  
- **测试时长**: 10分钟 (完整运行) ✅
- **无崩溃**: 系统稳定运行 ✅

## 架构符合性

此修复完全符合架构设计文档中的要求：

1. **单一写入协程原则**: 所有WebSocket写入都通过 `sendRoutine()` 处理
2. **消息驱动架构**: 使用通道进行协程间通信
3. **错误隔离**: 各组件独立处理错误，避免级联失败
4. **高并发支持**: 通过增加通道容量支持更高的并发量

## 经验教训

1. **严格遵循架构设计**: WebSocket连接的线程安全要求必须严格遵守
2. **容量规划**: 高并发场景下的通道容量需要根据实际负载进行合理配置
3. **统一写入原则**: 所有写入操作必须通过统一入口，避免并发冲突
4. **错误处理**: 在高并发场景下，错误处理机制需要更加健壮

## 相关文件

- `internal/transport/websocket/manager.go`: 主要修复文件
- `internal/transport/websocket/manager_test.go`: 相应的测试更新
- `test/e2e/performance/concurrent_connections_test.go`: 压测验证

## 状态

✅ **已解决** - Gateway服务现在可以稳定支持12000个并发连接，无崩溃运行10分钟。
