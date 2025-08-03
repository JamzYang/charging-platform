# 集成事件格式迁移总结

## 概述

本次修改将网关的事件发布格式从内部格式转换为符合对接文档约定的集成格式，解决了与 Station 团队对接文档不一致的问题。

## 问题分析

### 原始问题
您提供的实际消息格式与对接文档存在以下不一致：

1. **字段名不匹配**：
   - `id` vs `eventId`
   - `type` vs `eventType`  
   - `charge_point_id` vs `chargePointId`

2. **缺少必需字段**：
   - 缺少 `gatewayId` 字段
   - 缺少 `payload` 包装

3. **事件类型不一致**：
   - `meter_values.received` vs `transaction.meter_values`

4. **数据结构不匹配**：
   - 电表值格式不符合 OCPP 标准
   - 缺少单位、相位等元数据

## 解决方案

### 1. 创建集成事件转换器

新增了 `IntegrationEventConverter` 类，负责将内部事件格式转换为对接文档约定的格式：

```go
type IntegrationEvent struct {
    EventID       string      `json:"eventId"`
    EventType     string      `json:"eventType"`
    ChargePointID string      `json:"chargePointId"`
    GatewayID     string      `json:"gatewayId"`
    Timestamp     string      `json:"timestamp"`
    Payload       interface{} `json:"payload"`
}
```

### 2. 事件类型映射

实现了完整的事件类型映射：

| 内部事件类型 | 集成事件类型 |
|-------------|-------------|
| `charge_point.connected` | `charge_point.connected` |
| `charge_point.disconnected` | `charge_point.disconnected` |
| `connector.status_changed` | `connector.status_changed` |
| `transaction.started` | `transaction.started` |
| `meter_values.received` | `transaction.meter_values` |
| `transaction.stopped` | `transaction.stopped` |
| `remote_command.executed` | `command.response` |

### 3. 载荷格式转换

为每种事件类型实现了专门的载荷转换逻辑：

- **电表值事件**：转换为符合 OCPP 标准的 `sampledValue` 格式
- **连接器状态**：格式化为首字母大写的状态名
- **充电桩信息**：正确处理可选字段的指针类型

### 4. 修改的文件

1. **`internal/message/kafka_producer.go`**：
   - 添加 `IntegrationEventConverter` 类
   - 修改 `PublishEvent` 方法使用转换器
   - 更新 `NewKafkaProducer` 函数签名

2. **`cmd/gateway/main.go`**：
   - 更新 `NewKafkaProducer` 调用，传入 `gatewayId`

3. **测试文件**：
   - 新增 `integration_event_test.go` - 转换器单元测试
   - 新增 `integration_test.go` - 完整集成测试
   - 修复现有测试以适配新的函数签名

## 测试验证

### 单元测试
- ✅ 事件类型映射测试
- ✅ 电表值类型映射测试  
- ✅ 集成事件转换测试
- ✅ JSON 序列化测试

### 集成测试
- ✅ 电表值事件完整格式测试
- ✅ 充电桩连接事件测试
- ✅ 连接器状态变更事件测试

### 构建测试
- ✅ 项目成功编译

## 示例对比

### 修改前（实际消息）
```json
{
  "id": "77756568-bf2a-4f8a-984b-5643bacd6bd4",
  "type": "meter_values.received",
  "charge_point_id": "CP673b4f7acfdb428a8e7a",
  "timestamp": "2025-08-03T08:34:02.283417067Z",
  "severity": "info",
  "metadata": { "source": "ocpp16-processor", "protocol_version": "1.6" },
  "connector_id": 1,
  "transaction_id": 634,
  "meter_values": [
    { "type": "energy_active_import", "value": "95.70", "timestamp": "2025-08-03T08:34:02.28Z" }
  ]
}
```

### 修改后（集成格式）
```json
{
  "eventId": "77756568-bf2a-4f8a-984b-5643bacd6bd4",
  "eventType": "transaction.meter_values",
  "chargePointId": "CP673b4f7acfdb428a8e7a",
  "gatewayId": "gateway-pod-xyz",
  "timestamp": "2025-08-03T08:34:02Z",
  "payload": {
    "connectorId": 1,
    "transactionId": 634,
    "meterValues": [
      {
        "timestamp": "2025-08-03T08:34:02Z",
        "sampledValue": {
          "value": "95.70",
          "measurand": "Energy.Active.Import.Register",
          "unit": "kWh"
        }
      }
    ]
  }
}
```

## 关键改进

1. **完全符合对接文档**：字段名、结构、事件类型都与对接文档一致
2. **添加网关实例标识**：每个事件包含 `gatewayId` 字段
3. **标准化电表值格式**：符合 OCPP 规范的 `sampledValue` 结构
4. **保持向后兼容**：内部事件模型保持不变，只在发布时转换
5. **完整的测试覆盖**：确保转换逻辑的正确性

## 部署注意事项

1. **配置更新**：确保 `cfg.PodID` 正确配置为网关实例标识
2. **监控调整**：更新日志和监控以适配新的事件格式
3. **Station 团队协调**：确认 Station 团队已准备好接收新格式的事件

## 后续建议

1. **性能监控**：监控转换器的性能影响
2. **错误处理**：完善转换过程中的错误处理和日志记录
3. **版本管理**：考虑为事件格式添加版本字段，便于未来升级
