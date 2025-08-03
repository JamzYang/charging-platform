# 集成事件格式示例

本文档展示了按照对接文档修改后的集成事件格式示例。

## 1. 电表值事件 (transaction.meter_values)

**修改前的内部格式**：
```json
{
  "id": "77756568-bf2a-4f8a-984b-5643bacd6bd4",
  "type": "meter_values.received",
  "charge_point_id": "CP673b4f7acfdb428a8e7a",
  "timestamp": "2025-08-03T08:34:02.283417067Z",
  "severity": "info",
  "metadata": {
    "source": "ocpp16-processor",
    "protocol_version": "1.6"
  },
  "connector_id": 1,
  "transaction_id": 634,
  "meter_values": [
    {
      "type": "energy_active_import",
      "value": "95.70",
      "timestamp": "2025-08-03T08:34:02.28Z"
    }
  ]
}
```

**修改后的集成格式**：
```json
{
  "eventId": "77756568-bf2a-4f8a-984b-5643bacd6bd4",
  "eventType": "transaction.meter_values",
  "chargePointId": "CP673b4f7acfdb428a8e7a",
  "gatewayId": "gateway-pod-xyz",
  "timestamp": "1722672842283",
  "payload": {
    "connectorId": 1,
    "transactionId": 634,
    "meterValues": [
      {
        "timestamp": "1722672842283",
        "sampledValue": {
          "value": "95.70",
          "measurand": "Energy.Active.Import.Register",
          "unit": "kWh"
        }
      },
      {
        "timestamp": "1722672842283",
        "sampledValue": {
          "value": "7958",
          "measurand": "Power.Active.Import",
          "unit": "W"
        }
      },
      {
        "timestamp": "1722672842283",
        "sampledValue": {
          "value": "228.0",
          "measurand": "Voltage",
          "unit": "V"
        }
      }
    ]
  }
}
```

## 2. 充电桩连接事件 (charge_point.connected)

**集成格式**：
```json
{
  "eventId": "74fe221c-e8ad-4425-b570-de8156480a79",
  "eventType": "charge_point.connected",
  "chargePointId": "CP-001",
  "gatewayId": "gateway-pod-abc",
  "timestamp": "1722673799000",
  "payload": {
    "model": "Model-X",
    "vendor": "Vendor-A",
    "firmwareVersion": "v1.2.3"
  }
}
```

## 3. 连接器状态变更事件 (connector.status_changed)

**集成格式**：
```json
{
  "eventId": "3806e167-3aac-4659-9bf1-72791a810b22",
  "eventType": "connector.status_changed",
  "chargePointId": "CP-002",
  "gatewayId": "gateway-pod-def",
  "timestamp": "1722673799000",
  "payload": {
    "connectorId": 1,
    "status": "Charging",
    "previousStatus": "Preparing",
    "errorCode": "NoError"
  }
}
```

## 4. 充电桩断开连接事件 (charge_point.disconnected)

**集成格式**：
```json
{
  "eventId": "uuid-string",
  "eventType": "charge_point.disconnected",
  "chargePointId": "CP-003",
  "gatewayId": "gateway-pod-789",
  "timestamp": "1722673800000",
  "payload": {
    "reason": "tcp_connection_closed"
  }
}
```

## 5. 交易开始事件 (transaction.started)

**集成格式**：
```json
{
  "eventId": "uuid-string",
  "eventType": "transaction.started",
  "chargePointId": "CP-004",
  "gatewayId": "gateway-pod-123",
  "timestamp": "1722673800000",
  "payload": {
    "connectorId": 1,
    "transactionId": 12345,
    "idTag": "RFID123",
    "meterStartWh": 10500
  }
}
```

## 6. 交易结束事件 (transaction.stopped)

**集成格式**：
```json
{
  "eventId": "uuid-string",
  "eventType": "transaction.stopped",
  "chargePointId": "CP-005",
  "gatewayId": "gateway-pod-456",
  "timestamp": "1722673800000",
  "payload": {
    "transactionId": 12345,
    "reason": "Remote",
    "meterStopWh": 15800,
    "stopTimestamp": "1722673800000"
  }
}
```

## 7. 指令响应事件 (command.response)

**集成格式**：
```json
{
  "eventId": "uuid-string",
  "eventType": "command.response",
  "chargePointId": "CP-006",
  "gatewayId": "gateway-pod-789",
  "timestamp": "1722673800000",
  "payload": {
    "commandId": "cmd-uuid-123",
    "commandName": "RemoteStartTransaction",
    "status": "Accepted",
    "details": {}
  }
}
```

## 关键变更说明

1. **字段名映射**：
   - `id` → `eventId`
   - `type` → `eventType`
   - `charge_point_id` → `chargePointId`
   - 添加了 `gatewayId` 字段

2. **事件类型映射**：
   - `meter_values.received` → `transaction.meter_values`
   - `charge_point.connected` → `charge_point.connected` (保持不变)
   - `connector.status_changed` → `connector.status_changed` (保持不变)

3. **载荷包装**：
   - 所有业务数据都包装在 `payload` 字段中
   - 移除了 `severity` 和 `metadata` 字段

4. **数据格式优化**：
   - 电表值格式符合 OCPP 标准
   - 连接器状态使用首字母大写格式
   - 时间戳统一使用 Unix 毫秒时间戳字符串格式

5. **网关实例标识**：
   - 每个事件都包含 `gatewayId` 字段，标识处理该事件的网关实例
   - 便于分布式追踪和问题排查
