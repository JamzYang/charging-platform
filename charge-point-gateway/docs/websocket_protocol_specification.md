# 充电桩网关 WebSocket 通信协议规范

## 文档信息
- **版本**: v1.0.0
- **协议**: OCPP 1.6J
- **更新日期**: 2024-01-14
- **适用范围**: 模拟充电桩应用开发

## 目录
1. [WebSocket连接建立](#1-websocket连接建立)
2. [OCPP 1.6 消息格式规范](#2-ocpp-16-消息格式规范)
3. [插枪/拔枪消息格式](#3-插枪拔枪消息格式)
4. [充电状态推送消息格式](#4-充电状态推送消息格式)
5. [错误消息格式](#5-错误消息格式)
6. [数据字典](#6-数据字典)
7. [环境配置](#7-环境配置)
8. [前端需要处理的OCPP消息类型](#8-前端需要处理的ocpp消息类型)
9. [开发建议](#9-开发建议)

---

## 1. WebSocket连接建立

### 1.1 连接地址
```
ws://host:port/ocpp/{charge_point_id}
```

**参数说明：**
- `host`: 网关服务器地址
- `port`: 网关服务器端口（默认8080）
- `charge_point_id`: 充电桩唯一标识符

**环境配置：**
- **开发环境**: `ws://localhost:8080/ocpp/{charge_point_id}`
- **测试环境**: `ws://localhost:8081/ocpp/{charge_point_id}`
- **生产环境**: `wss://gateway.example.com:8080/ocpp/{charge_point_id}` (启用TLS)

### 1.2 认证方式
- **子协议**: 推荐指定 `ocpp1.6`（如未指定，系统将使用默认版本）
- **握手头**: `Sec-WebSocket-Protocol: ocpp1.6`
- **容错机制**: 系统支持在未协商到有效子协议时自动使用默认版本
- **生产环境**: 支持TLS客户端证书认证

**连接示例：**
```javascript
// JavaScript
const ws = new WebSocket('ws://localhost:8080/ocpp/CP-001', ['ocpp1.6']);

// Python
import websockets
uri = "ws://localhost:8080/ocpp/CP-001"
websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])

// Go
headers := make(map[string][]string)
headers["Sec-WebSocket-Protocol"] = []string{"ocpp1.6"}
conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ocpp/CP-001", headers)
```

---

## 2. OCPP 1.6 消息格式规范

### 2.1 基础消息结构
所有OCPP消息都采用JSON数组格式：

**请求消息 (Call):**
```json
[2, "messageId", "action", payload]
```

**响应消息 (CallResult):**
```json
[3, "messageId", payload]
```

**错误消息 (CallError):**
```json
[4, "messageId", "errorCode", "errorDescription", errorDetails]
```

### 2.2 消息类型说明
- **MessageType 2 (Call)**: 请求消息，需要响应
- **MessageType 3 (CallResult)**: 成功响应消息
- **MessageType 4 (CallError)**: 错误响应消息

### 2.3 启动通知示例 (BootNotification)
```json
[2, "msg001", "BootNotification", {
  "chargePointVendor": "TestVendor",
  "chargePointModel": "TestModel",
  "chargePointSerialNumber": "SN123456",
  "firmwareVersion": "1.0.0",
  "iccid": "89860000000000000000",
  "imsi": "123456789012345",
  "meterType": "TestMeter",
  "meterSerialNumber": "MSN123456"
}]
```

**响应：**
```json
[3, "msg001", {
  "status": "Accepted",
  "currentTime": "2024-01-14T10:00:00Z",
  "interval": 300
}]
```

---

## 3. 插枪/拔枪消息格式

### 3.1 插枪消息 (StatusNotification)

**插枪状态通知：**
```json
[2, "msg001", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "NoError", 
  "status": "Preparing",
  "timestamp": "2024-01-14T10:00:00Z",
  "info": "Cable connected, preparing for charging"
}]
```

**响应确认：**
```json
[3, "msg001", {}]
```

### 3.2 拔枪消息 (StatusNotification)
```json
[2, "msg002", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "NoError",
  "status": "Available", 
  "timestamp": "2024-01-14T10:30:00Z",
  "info": "Cable disconnected, connector available"
}]
```

### 3.3 状态变化流程
```
Available → Preparing → Charging → Finishing → Available
    ↑                                              ↓
    ←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←←
```

---

## 4. 充电状态推送消息格式

### 4.1 启动充电确认 (StartTransaction)

**启动充电消息：**
```json
[2, "msg003", "StartTransaction", {
  "connectorId": 1,
  "idTag": "RFID123456",
  "meterStart": 1000,
  "timestamp": "2024-01-14T10:05:00Z"
}]
```

**启动充电响应：**
```json
[3, "msg003", {
  "idTagInfo": {
    "status": "Accepted"
  },
  "transactionId": 12345
}]
```

### 4.2 充电量实时数据 (MeterValues)

**充电数据消息：**
```json
[2, "msg004", "MeterValues", {
  "connectorId": 1,
  "transactionId": 12345,
  "meterValue": [{
    "timestamp": "2024-01-14T10:10:00Z",
    "sampledValue": [
      {
        "value": "1234.56",
        "measurand": "Energy.Active.Import.Register",
        "unit": "kWh"
      },
      {
        "value": "7200",
        "measurand": "Power.Active.Import", 
        "unit": "W"
      },
      {
        "value": "230.5",
        "measurand": "Voltage",
        "phase": "L1",
        "unit": "V"
      },
      {
        "value": "31.3",
        "measurand": "Current.Import",
        "phase": "L1", 
        "unit": "A"
      }
    ]
  }]
}]
```

**响应：**
```json
[3, "msg004", {}]
```

### 4.3 停止充电确认 (StopTransaction)
```json
[2, "msg005", "StopTransaction", {
  "idTag": "RFID123456",
  "meterStop": 1500,
  "timestamp": "2024-01-14T10:30:00Z",
  "transactionId": 12345,
  "reason": "Local",
  "transactionData": [
    {
      "timestamp": "2024-01-14T10:30:00Z",
      "sampledValue": [
        {
          "value": "1500.00",
          "measurand": "Energy.Active.Import.Register",
          "unit": "kWh"
        }
      ]
    }
  ]
}]
```

**停止充电响应：**
```json
[3, "msg005", {
  "idTagInfo": {
    "status": "Accepted"
  }
}]
```

---

## 5. 错误消息格式

### 5.1 错误码定义

| 错误码 | 描述 | 使用场景 |
|--------|------|----------|
| `NoError` | 无错误 | 正常状态 |
| `ConnectorLockFailure` | 连接器锁定失败 | 插枪锁定异常 |
| `EVCommunicationError` | 电动车通信错误 | 与车辆通信异常 |
| `GroundFailure` | 接地故障 | 安全检测异常 |
| `HighTemperature` | 高温故障 | 温度过高 |
| `InternalError` | 内部错误 | 系统内部异常 |
| `LocalListConflict` | 本地列表冲突 | 授权列表冲突 |
| `OtherError` | 其他错误 | 未分类错误 |
| `OverCurrentFailure` | 过流故障 | 电流超限 |
| `OverVoltage` | 过压故障 | 电压超限 |
| `PowerMeterFailure` | 电表故障 | 计量异常 |
| `PowerSwitchFailure` | 电源开关故障 | 开关控制异常 |
| `ReaderFailure` | 读卡器故障 | RFID读取异常 |
| `ResetFailure` | 重启失败 | 设备重启异常 |
| `UnderVoltage` | 欠压故障 | 电压不足 |
| `WeakSignal` | 信号弱 | 通信信号不良 |

### 5.2 错误消息示例
```json
[4, "msg006", "InternalError", "Internal system error occurred", {
  "details": "Database connection failed",
  "timestamp": "2024-01-14T10:15:00Z",
  "errorCode": "DB_CONNECTION_FAILED"
}]
```

### 5.3 状态通知错误示例
```json
[2, "msg007", "StatusNotification", {
  "connectorId": 1,
  "errorCode": "HighTemperature",
  "status": "Faulted",
  "timestamp": "2024-01-14T10:20:00Z",
  "info": "Connector temperature exceeded 80°C",
  "vendorErrorCode": "TEMP_001"
}]
```

---

## 6. 数据字典

### 6.1 充电桩状态枚举 (ChargePointStatus)

| 状态值 | 描述 | 使用场景 |
|--------|------|----------|
| `Available` | 可用 | 空闲状态，可以开始充电 |
| `Preparing` | 准备中 | 插枪后，准备开始充电 |
| `Charging` | 充电中 | 正在充电 |
| `SuspendedEVSE` | 设备暂停 | 充电桩主动暂停 |
| `SuspendedEV` | 车辆暂停 | 车辆主动暂停 |
| `Finishing` | 结束中 | 充电结束，准备拔枪 |
| `Reserved` | 已预约 | 被预约占用 |
| `Unavailable` | 不可用 | 维护或故障 |
| `Faulted` | 故障 | 设备故障 |

### 6.2 OCPP动作类型 (Action)

**核心功能 (Core Profile):**
- `Authorize` - 授权请求
- `BootNotification` - 启动通知
- `ChangeAvailability` - 改变可用性
- `ChangeConfiguration` - 修改配置
- `ClearCache` - 清除缓存
- `DataTransfer` - 数据传输
- `GetConfiguration` - 获取配置
- `Heartbeat` - 心跳
- `MeterValues` - 电表值
- `RemoteStartTransaction` - 远程启动充电
- `RemoteStopTransaction` - 远程停止充电
- `Reset` - 重启
- `StartTransaction` - 开始交易
- `StatusNotification` - 状态通知
- `StopTransaction` - 停止交易
- `UnlockConnector` - 解锁连接器

### 6.3 计量单位 (UnitOfMeasure)

| 单位 | 描述 | 用途 |
|------|------|------|
| `kWh` | 千瓦时 | 电能 |
| `W` | 瓦特 | 功率 |
| `V` | 伏特 | 电压 |
| `A` | 安培 | 电流 |
| `Celsius` | 摄氏度 | 温度 |
| `Percent` | 百分比 | SOC等 |

### 6.4 停止原因 (Reason)

| 原因 | 描述 |
|------|------|
| `EmergencyStop` | 紧急停止 |
| `EVDisconnected` | 车辆断开 |
| `HardReset` | 硬重启 |
| `Local` | 本地停止 |
| `Other` | 其他原因 |
| `PowerLoss` | 断电 |
| `Remote` | 远程停止 |
| `DeAuthorized` | 取消授权 |

---

## 7. 环境配置

### 7.1 开发环境
```bash
# WebSocket连接地址
WS_URL=ws://localhost:8080/ocpp/{charge_point_id}

# API端点
HEALTH_CHECK_URL=http://localhost:8080/health
CONNECTIONS_URL=http://localhost:8080/connections

# 服务器配置
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_WEBSOCKET_PATH=/ocpp

# 超时配置
SERVER_READ_TIMEOUT=60s
SERVER_WRITE_TIMEOUT=60s
MAX_CONNECTIONS=100000
```

### 7.2 测试环境
```bash
# WebSocket连接地址
WS_URL=ws://localhost:8081/ocpp/{charge_point_id}

# API端点
HEALTH_CHECK_URL=http://localhost:8081/health
CONNECTIONS_URL=http://localhost:8081/connections

# 服务器配置
SERVER_PORT=8081
SERVER_READ_TIMEOUT=300s    # 延长读取超时到5分钟，匹配OCPP心跳间隔
SERVER_WRITE_TIMEOUT=60s
MAX_CONNECTIONS=25000       # 设置为25000以支持2万连接测试

# WebSocket配置
WEBSOCKET_PING_INTERVAL=30s
WEBSOCKET_PONG_TIMEOUT=10s
WEBSOCKET_IDLE_TIMEOUT=15m
```

### 7.3 生产环境
```bash
# WebSocket连接地址 (启用TLS)
WS_URL=wss://gateway.example.com:8080/ocpp/{charge_point_id}

# API端点
HEALTH_CHECK_URL=https://gateway.example.com:8080/health

# 安全配置
SECURITY_TLS_ENABLED=true
SECURITY_CERT_FILE=/app/certs/server.crt
SECURITY_KEY_FILE=/app/certs/server.key
SECURITY_CLIENT_AUTH=true
```

---

## 8. 前端需要处理的OCPP消息类型

### 8.1 上行消息（充电桩发送到网关）

| 消息类型 | 描述 | 发送时机 | 必需字段 |
|----------|------|----------|----------|
| `BootNotification` | 启动通知 | 设备启动时 | chargePointVendor, chargePointModel |
| `StatusNotification` | 状态通知 | 状态变化时 | connectorId, errorCode, status |
| `StartTransaction` | 开始充电 | 开始充电时 | connectorId, idTag, meterStart, timestamp |
| `StopTransaction` | 停止充电 | 停止充电时 | meterStop, timestamp, transactionId |
| `MeterValues` | 充电数据 | 定期上报 | connectorId, meterValue |
| `Heartbeat` | 心跳 | 定期发送 | 无 |
| `Authorize` | 授权请求 | 刷卡时 | idTag |

### 8.2 下行消息（网关发送到充电桩）

| 消息类型 | 描述 | 触发条件 | 必需字段 |
|----------|------|----------|----------|
| `RemoteStartTransaction` | 远程启动 | 用户远程启动 | idTag |
| `RemoteStopTransaction` | 远程停止 | 用户远程停止 | transactionId |
| `ChangeConfiguration` | 修改配置 | 配置更新 | key, value |
| `TriggerMessage` | 触发消息 | 主动查询 | requestedMessage |
| `Reset` | 重启设备 | 维护需要 | type |
| `UnlockConnector` | 解锁连接器 | 紧急解锁 | connectorId |

---

## 9. 开发建议

### 9.1 消息ID管理
```javascript
// 推荐的消息ID格式
const generateMessageId = () => {
  const timestamp = Date.now();
  const random = Math.floor(Math.random() * 1000);
  return `cp-${timestamp}-${random}`;
};
```

### 9.2 错误处理
```javascript
// WebSocket错误处理示例
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
  // 实现重连逻辑
  setTimeout(() => {
    reconnect();
  }, 5000);
};

ws.onclose = (event) => {
  if (event.code !== 1000) {
    console.log('Connection closed unexpectedly, reconnecting...');
    reconnect();
  }
};
```

### 9.3 心跳机制

#### OCPP层心跳（应用层）
```javascript
// OCPP Heartbeat消息发送示例
const sendHeartbeat = () => {
  const heartbeat = [2, generateMessageId(), "Heartbeat", {}];
  ws.send(JSON.stringify(heartbeat));
};

// 每300秒发送一次OCPP心跳（根据BootNotification响应的interval字段）
setInterval(sendHeartbeat, 300000);
```

#### WebSocket层心跳（传输层）
```javascript
// WebSocket Ping/Pong机制由网关自动处理
// 网关每30秒向所有连接发送WebSocket Ping帧
// 客户端应自动响应Pong帧以保持连接活跃
// 如果10秒内未收到Pong响应，连接将被关闭

// 客户端通常不需要手动处理，但可以监听事件
ws.addEventListener('ping', (event) => {
  console.log('Received WebSocket ping');
});

ws.addEventListener('pong', (event) => {
  console.log('Received WebSocket pong');
});
```

### 9.4 状态同步
```javascript
// 状态管理示例
class ChargePointState {
  constructor() {
    this.status = 'Available';
    this.connectorId = 1;
    this.currentTransaction = null;
  }
  
  updateStatus(newStatus) {
    if (this.status !== newStatus) {
      this.status = newStatus;
      this.sendStatusNotification();
    }
  }
  
  sendStatusNotification() {
    const message = [2, generateMessageId(), "StatusNotification", {
      connectorId: this.connectorId,
      errorCode: "NoError",
      status: this.status,
      timestamp: new Date().toISOString()
    }];
    ws.send(JSON.stringify(message));
  }
}
```

### 9.5 数据验证
```javascript
// 消息验证示例
const validateMessage = (message) => {
  if (!Array.isArray(message) || message.length < 3) {
    throw new Error('Invalid message format');
  }
  
  const [messageType, messageId, action, payload] = message;
  
  if (![2, 3, 4].includes(messageType)) {
    throw new Error('Invalid message type');
  }
  
  if (typeof messageId !== 'string' || messageId.length === 0) {
    throw new Error('Invalid message ID');
  }
  
  return true;
};
```

### 9.6 完整连接示例
```javascript
class ChargePointSimulator {
  constructor(chargePointId, serverUrl) {
    this.chargePointId = chargePointId;
    this.serverUrl = serverUrl;
    this.ws = null;
    this.state = new ChargePointState();
  }
  
  connect() {
    const url = `${this.serverUrl}/ocpp/${this.chargePointId}`;
    this.ws = new WebSocket(url, ['ocpp1.6']);
    
    this.ws.onopen = () => {
      console.log('Connected to gateway');
      this.sendBootNotification();
    };
    
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.ws.onclose = () => {
      console.log('Connection closed');
      // 实现重连逻辑
    };
  }
  
  sendBootNotification() {
    const message = [2, generateMessageId(), "BootNotification", {
      chargePointVendor: "TestVendor",
      chargePointModel: "TestModel",
      chargePointSerialNumber: "SN123456",
      firmwareVersion: "1.0.0"
    }];
    this.ws.send(JSON.stringify(message));
  }
  
  handleMessage(message) {
    const [messageType, messageId, actionOrPayload, payload] = message;
    
    if (messageType === 2) { // Call
      this.handleCall(messageId, actionOrPayload, payload);
    } else if (messageType === 3) { // CallResult
      this.handleCallResult(messageId, actionOrPayload);
    } else if (messageType === 4) { // CallError
      this.handleCallError(messageId, actionOrPayload, payload);
    }
  }
}
```

---

## 附录

### A. 常用测试工具
- **wscat**: `npm install -g wscat`
- **WebSocket King**: Chrome扩展
- **Postman**: 支持WebSocket测试

### B. 调试命令
```bash
# 使用wscat连接
wscat -c ws://localhost:8080/ocpp/CP-001 -s ocpp1.6

# 发送BootNotification
[2,"msg001","BootNotification",{"chargePointVendor":"TestVendor","chargePointModel":"TestModel"}]

# 健康检查
curl http://localhost:8080/health

# 查看连接状态
curl http://localhost:8080/connections
```

### C. 相关文档
- [OCPP 1.6 官方规范](https://www.openchargealliance.org/protocols/ocpp-16/)
- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455)
- [项目架构设计文档](./high_availability_gateway_arch_design.md)

---

**文档版本**: v1.1.0
**最后更新**: 2025-01-27
**维护者**: 充电桩网关开发团队

## 更新日志

### v1.1.0 (2025-01-27)
- 补充了缺失的错误码定义（LocalListConflict、OtherError、PowerSwitchFailure等）
- 修正了子协议验证机制的描述，增加了容错机制说明
- 更新了测试环境的实际配置参数
- 区分了OCPP层心跳和WebSocket层心跳机制
- 增加了WebSocket Ping/Pong机制的详细说明
