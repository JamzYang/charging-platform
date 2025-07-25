# 网关与场站服务集成协作手册 V1.0

**版本：1.0**  
**日期：2025-07-25**  
**制定方：充电桩网关 (Charge Point Gateway) 团队**  
**面向方：场站域服务 (Station Service) 团队**

---

## 1. 文档目的

本手册旨在为 **充电桩网关 (Gateway)** 与 **场站域服务 (Station)** 之间的系统集成、开发协作和长期维护提供一套清晰、全面的技术规范和流程协议。其核心目标是：

-   **建立统一契约**: 明确双方交互的接口、数据模型和通信协议。
-   **降低集成成本**: 提供清晰的对接指南，减少联调阶段的不确定性。
-   **提升协作效率**: 规范化变更、排障和沟通流程。
-   **保障系统稳定性**: 定义服务等级、监控和应急响应机制。

本文档是两个团队之间技术协作的 **“单一事实来源”**。

---

## 2. 核心交互接口与数据契约

Gateway 与 Station Service 之间的所有交互均通过 **Kafka 消息队列** 进行，以实现异步解耦和高可用性。

### 2.1 上行事件 (Gateway -> Station)

Station Service 作为消费者，从以下 Topic 接收由 Gateway 产生的标准化业务事件。

-   **Kafka Topic**: `ocpp-events-up`
-   **分区策略**: 事件以 `chargePointId` 作为消息的 Key，确保同一充电桩的所有事件严格有序，并被同一个消费者实例处理。
-   **数据格式**: 所有事件均为 JSON 格式，包含统一的事件头和具体的事件体。

#### 2.1.1 标准事件数据模型

所有上行事件都遵循以下基本结构：

```json
{
  "eventId": "uuid-string",         // 事件唯一ID
  "eventType": "charge_point.connected", // 事件类型 (关键)
  "chargePointId": "CP-001",        // 充电桩ID
  "gatewayId": "gateway-pod-xyz",   // 处理该事件的网关实例ID
  "timestamp": "2025-07-25T10:30:00Z", // 事件发生时间 (ISO 8601)
  "payload": { ... }                // 事件具体载荷
}
```

#### 2.1.2 关键业务事件列表

| 事件类型 (`eventType`)         | 描述                               | `payload` 示例 (JSON)                                                                                                                                                                                          |
| ------------------------------ | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `charge_point.connected`       | 充电桩上线并成功完成启动通知       | `{ "model": "Model-X", "vendor": "Vendor-A", "firmwareVersion": "v1.2.3" }`                                                                                                                                     |
| `charge_point.disconnected`    | 充电桩连接断开                     | `{ "reason": "tcp_connection_closed" }`                                                                                                                                                                        |
| `connector.status_changed`     | 充电枪状态变更                     | `{ "connectorId": 1, "status": "Charging", "previousStatus": "Preparing", "errorCode": "NoError" }`                                                                                                             |
| `transaction.started`          | 交易（充电）开始                   | `{ "connectorId": 1, "transactionId": "TXN12345", "idTag": "RFID123", "meterStartWh": 10500 }`                                                                                                                  |
| `transaction.meter_values`     | 充电过程中的电度计量值上报         | `{ "connectorId": 1, "transactionId": "TXN12345", "meterValues": [{ "timestamp": "...", "sampledValue": { "value": 11200, "measurand": "Energy.Active.Import.Register", "unit": "Wh" } }] }`                      |
| `transaction.stopped`          | 交易（充电）结束                   | `{ "transactionId": "TXN12345", "reason": "Remote", "meterStopWh": 15800, "stopTimestamp": "..." }`                                                                                                             |
| `command.response`             | 对下行指令的最终响应               | `{ "commandId": "cmd-uuid-123", "commandName": "RemoteStartTransaction", "status": "Accepted", "details": {} }`                                                                                                |

---

### 2.2 下行指令 (Station -> Gateway)

Station Service 作为生产者，向下行 Topic 发送控制指令。

-   **Kafka Topic**: `commands-down` ( **注意**: 是一个共享的、统一的 Topic)
-   **数据格式**: JSON

#### 2.2.1 **核心：指令路由机制**

为避免“主题爆炸”并实现动态伸缩，Gateway 采用 **“共享主题 + 分区路由”** 模式。Station Service **必须** 遵循以下流程发送指令：

1.  **查询连接映射**: 从 **Redis** 中根据 `chargePointId` 查询其当前连接的 Gateway Pod ID。
    -   **Redis Key**: `conn:<chargePointId>` (e.g., `conn:CP-001`)
    -   **Redis Value**: `<gatewayPodId>` (e.g., `gateway-pod-xyz`)

2.  **计算目标分区**: 使用稳定的哈希算法（如 FNV-1a）计算 Gateway Pod ID 的哈希值，并对预设的分区总数取模，得到目标分区号。
    -   `partition_id = hash("<gatewayPodId>") % 128` (假设总分区数为128)

3.  **发送到指定分区**: 使用 Kafka 生产者客户端，将指令消息 **精确地发送到计算出的 `partition_id`**。

#### 2.2.2 指令数据模型

```json
{
  "commandId": "uuid-string-from-station", // 指令唯一ID，用于追踪
  "commandName": "RemoteStartTransaction",   // 指令名称
  "chargePointId": "CP-001",                 // 目标充电桩ID
  "payload": { ... },                        // 指令具体载荷
  "traceId": "trace-uuid-from-platform"      // 分布式追踪ID (强烈建议)
}
```

#### 2.2.3 关键指令列表

| 指令名称 (`commandName`)     | 描述         | `payload` 示例 (JSON)                                    |
| ---------------------------- | ------------ | -------------------------------------------------------- |
| `RemoteStartTransaction`     | 远程启动充电 | `{ "idTag": "RFID123", "connectorId": 1 }`               |
| `RemoteStopTransaction`      | 远程停止充电 | `{ "transactionId": "TXN12345" }`                        |
| `TriggerMessage`             | 触发消息上报 | `{ "requestedMessage": "StatusNotification" }`           |
| `ChangeConfiguration`        | 修改设备配置 | `{ "key": "HeartbeatInterval", "value": "120" }`         |

---

## 3. 技术规范与指南

### 3.1 错误码与异常处理

Gateway 层面产生的、需要 Station Service 感知的错误会通过 `command.response` 事件的 `status` 字段（值为 `Rejected`）和 `details` 载荷来传递。

| 错误场景                 | `details` 示例                                     | 建议处理方式                               |
| ------------------------ | -------------------------------------------------- | ------------------------------------------ |
| 设备不在线               | `{ "reason": "DEVICE_OFFLINE" }`                   | 更新设备状态，标记指令失败，通知上层业务。 |
| 指令格式无效             | `{ "reason": "INVALID_COMMAND_PAYLOAD" }`          | 检查指令构造逻辑，修正后可重试。           |
| 设备拒绝执行             | `{ "reason": "DEVICE_REJECTED", "deviceStatus": "Faulted" }` | 根据设备状态判断，可能需要人工介入。       |

### 3.2 鉴权与安全

-   **设备接入层**: Gateway 负责与充电桩之间的 TLS 握手和基础认证，确保只有合法的设备可以接入。
-   **服务间通信**: Gateway 与 Kafka/Redis 之间的通信安全由平台基础设施层（如 Kafka ACL、Redis 密码）统一保障。Station Service 无需关心设备级的安全细节。

### 3.3 部署与配置

-   Gateway 作为 K8s Deployment 部署，可动态扩缩容。
-   Station Service 需要的配置信息（Kafka Brokers, Redis 地址, Topic名称等）将通过平台统一的配置中心（如 K8s ConfigMap）提供。请勿在代码中硬编码。

### 3.4 监控与日志

为便于联合排障，双方需遵循以下可观测性约定：

-   **Gateway 核心指标**: Gateway 会通过 Prometheus 暴露以下核心指标，Station 团队可按需在 Grafana 中创建监控面板：
    -   `gateway_connected_devices_total`: 当前总连接数。
    -   `gateway_events_produced_total{type="event_type"}`: 按类型统计的上行事件生产数。
    -   `gateway_commands_consumed_total{name="command_name"}`: 按类型统计的下行指令消费数。
    -   `gateway_command_routing_latency_seconds`: 指令路由延迟。
-   **分布式追踪 (TraceID)**: Station Service 在创建下行指令时，**必须** 生成一个 `TraceID` 并放入指令消息中。Gateway 在处理该指令及后续产生的所有相关事件时，会全程透传此 `TraceID`，以便在 Jaeger/Skywalking 中追踪完整的调用链路。

---

## 4. 协作流程与协议

### 4.1 版本管理与变更流程

-   **版本策略**: 本手册及所有接口契约（事件、指令模型）遵循 **语义化版本（Semantic Versioning 2.0.0）**。
-   **变更通知**:
    -   **非破坏性变更** (如新增事件、在 payload 中增加可选字段): Gateway 团队将在发布后通过协作渠道通知。
    -   **破坏性变更** (如修改现有字段、删除事件): Gateway 团队 **至少提前两周** 在协作渠道中发出正式通知，提供详细的变更说明、迁移指南和新的契约文档版本，并与 Station 团队共同制定升级计划。

### 4.2 故障排查指南 (Playbook)

| 故障现象                   | Station 团队排查步骤                                                                                             | Gateway 团队排查步骤                                                                                             |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| **远程控制指令无响应**     | 1. 确认指令已成功发送到 `commands-down` Topic 的 **正确分区**。<br>2. 检查 Redis 中 `conn:<CPID>` 是否存在且值正确。<br>3. 检查日志中是否有 Kafka 生产错误。<br>4. 提供 `TraceID`。 | 1. 根据 `TraceID` 检查日志，确认是否消费到指令。<br>2. 检查 Gateway 与设备间的 WebSocket 连接是否正常。<br>3. 检查设备是否返回了响应或错误。 |
| **设备状态长时间未更新**   | 1. 确认 Kafka 消费者是否正常运行，没有消费延迟。<br>2. 检查日志中是否有事件解析错误。<br>3. 确认 `ocpp-events-up` Topic 是否有新消息。 | 1. 检查 `gateway_events_produced_total` 指标是否增长。<br>2. 检查对应设备的 WebSocket 连接是否活跃。<br>3. 检查 Gateway 日志中是否有 Kafka 生产错误。 |

### 4.3 服务等级协议 (SLA)

-   **Gateway 可用性**: `99.95%` (月度)
-   **上行事件处理延迟**: P99 < `500ms` (从 Gateway 收到设备消息到发布至 Kafka)
-   **下行指令消费延迟**: P99 < `500ms` (从 Kafka 获取到指令到发送给设备)

### 4.4 沟通与支持机制

-   **日常沟通**:
    -   Slack/Teams 频道: `#team-station-gateway-integration`
-   **正式请求**:
    -   Bug 报告 & 功能需求: JIRA 项目 `INTEGRATION`
-   **紧急事件**:
    -   On-call 流程: 通过 PagerDuty 触发，仅限 P0/P1 级生产故障。
-   **定期会议**:
    -   **双周同步会**: 审阅进行中的集成任务、讨论即将到来的变更、解决阻塞问题。
