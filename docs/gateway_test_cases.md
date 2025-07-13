# 高可用网关 - 集成与端到端测试用例

本文档旨在为高可用充电桩网关项目提供一套全面的测试用例，用于指导集成测试和端到端（E2E）测试的开发。

## 1. 测试目标

*   验证网关核心功能的正确性，包括上行数据处理、下行指令路由。
*   确保系统在 Kubernetes 环境下的高可用性，特别是节点故障时的自动恢复能力。
*   检验系统在不同负载下的性能和稳定性。
*   覆盖架构设计中的关键流程和风险缓解措施。

## 2. 测试环境与依赖

在执行测试前，需要搭建一个包含以下组件的、与生产环境高度相似的测试环境：

*   **Kubernetes 集群**: 用于部署网关、API 服务及其他依赖。
*   **Kafka 集群**: 用于消息传递。
*   **Redis 集群**: 用于存储连接映射。
*   **模拟充电桩 (Charge Point Simulator)**: 一个可以模拟 OCPP 协议行为的工具，能够：
    *   建立 WebSocket 连接。
    *   发送各类 OCPP 消息 (`BootNotification`, `MeterValues` 等)。
    *   接收并响应下行指令。
    *   模拟连接断开和重连。
*   **模拟后端服务 (Backend Simulator)**: 一个可以与 Kafka 和 Redis 交互的工具，能够：
    *   消费上行事件主题 (`ocpp-events-up`) 并验证事件内容。
    *   向 Redis 查询连接映射。
    *   向下行指令主题 (`commands-down`) 发布指令。

## 3. 集成测试用例

集成测试主要验证 Gateway Pod 内部各模块以及与外部依赖（Kafka, Redis）的交互是否正确。

### 场景 1: 上行数据流 (Happy Path)

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-INT-01** | **BootNotification**: 验证桩上线流程 | 1. 网关、Kafka、Redis 正常运行。 <br> 2. 后端模拟器已订阅 `ocpp-events-up` 主题。 | 1. 模拟充电桩 (ID: `CP-001`, OCPP 1.6J) 连接到网关。 <br> 2. 桩发送 `BootNotification` 请求。 | 1. 桩收到 `BootNotification.conf` 响应，`status` 为 `Accepted`。 <br> 2. Redis 中存在键 `conn:CP-001`，其值为当前 Gateway Pod 的 ID。 <br> 3. 后端模拟器在 Kafka 中消费到一条 `DeviceOnlineEvent` 事件，内容与 `BootNotification` 请求匹配。 |
| **TC-INT-02** | **MeterValues**: 验证计量数据上报 | 1. `TC-INT-01` 已成功。 | 1. 模拟充电桩 `CP-001` 发送 `MeterValues` 请求。 | 1. 桩收到 `MeterValues.conf` 响应。 <br> 2. 后端模拟器在 Kafka 中消费到一条 `MeterValuesEvent` 事件，内容与请求匹配。 |
| **TC-INT-03** | **StatusNotification**: 验证状态变更上报 | 1. `TC-INT-01` 已成功。 | 1. 模拟充电桩 `CP-001` 发送 `StatusNotification` 请求 (e.g., `connectorStatus` 变为 `Preparing`)。 | 1. 桩收到 `StatusNotification.conf` 响应。 <br> 2. 后端模拟器在 Kafka 中消费到一条 `DeviceStatusEvent` 事件，内容与请求匹配。 |

### 场景 2: 下行指令流 (Happy Path)

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-INT-04** | **RemoteStartTransaction**: 验证远程启动指令 | 1. `TC-INT-01` 已成功。 | 1. 后端模拟器查询 Redis，获取 `CP-001` 所在的 Pod ID。 <br> 2. 后端模拟器计算分区，并向 `commands-down` 主题的对应分区发送一条 `RemoteStartTransaction` 指令。 | 1. 模拟充电桩 `CP-001` 收到 `RemoteStartTransaction` 请求。 <br> 2. 桩可以正确解析请求内容。 |

### 场景 3: 异常与边界情况

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-INT-05** | **格式错误的消息**: 验证网关对畸形报文的处理 | 1. 模拟桩已连接。 | 1. 模拟桩发送一个 JSON 格式错误的 OCPP 报文。 | 1. 网关连接不中断。 <br> 2. 网关返回一个 OCPP Error 消息给充电桩，指明错误原因（如 `FormatViolation`）。 |
| **TC-INT-06** | **不支持的 Action**: 验证对未知消息的处理 | 1. 模拟桩已连接。 | 1. 模拟桩发送一个在对应 OCPP 版本中不存在的 Action，如 `FakeAction`。 | 1. 网关连接不中断。 <br> 2. 网关返回一个 OCPP Error 消息，指明错误原因（如 `NotSupported`）。 |
| **TC-INT-07** | **Kafka/Redis 临时不可用**: 验证网关的容错与恢复 | 1. 网关正在运行。 | 1. 临时中断网关与 Kafka 或 Redis 的网络连接。 <br> 2. 恢复网络连接。 | 1. 在中断期间，网关应记录错误日志，但进程保持运行。 <br> 2. 恢复后，网关应能自动重连到 Kafka/Redis 并恢复正常工作。 |

## 4. 端到端 (E2E) 测试用例

E2E 测试验证在真实部署环境下，从设备到后端的完整业务流程。

### 场景 4: 高可用性与故障转移

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-E2E-01** | **节点故障转移**: 验证网关自愈能力 | 1. 部署 2 个 Gateway Pod (`pod-a`, `pod-b`)。 <br> 2. 模拟桩 `CP-007` 已连接到 `pod-a`。 <br> 3. Redis 中 `conn:CP-007` 的值为 `pod-a`。 | 1. 手动删除 (kill) `pod-a`。 <br> 2. 等待模拟桩自动重连。 | 1. 模拟桩的 TCP 连接断开。 <br> 2. 模拟桩在短暂延时后成功重新连接到 `pod-b`。 <br> 3. 模拟桩在新连接上重新发送 `BootNotification`。 <br> 4. Redis 中 `conn:CP-007` 的值被更新为 `pod-b`。 <br> 5. K8s 会自动拉起一个新的 Pod 替代 `pod-a`。 |
| **TC-E2E-02** | **故障转移期间的指令路由**: 验证指令不丢失 | 1. `TC-E2E-01` 的场景正在发生。 | 1. 在 `pod-a` 被删除后，但在 `CP-007` 重连到 `pod-b` 之前，后端模拟器持续向 `CP-007` 发送指令。 <br> 2. 在 `CP-007` 重连到 `pod-b` 之后，后端模拟器再次发送指令。 | 1. 步骤 1 中发送的指令可能会失败（取决于后端的重试策略），但不会被错误的 Pod 消费。 <br> 2. 步骤 2 中发送的指令必须成功被 `pod-b` 接收，并最终下发给 `CP-007`。 |

### 场景 5: 完整业务流程

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-E2E-03** | **完整充电流程**: 模拟一次完整的充电会话 | 1. `TC-INT-01` 已成功。 | 1. **远程启动**: 后端发送 `RemoteStartTransaction`。 <br> 2. **启动确认**: 桩回复 `RemoteStartTransaction.conf` 并发送 `StatusNotification` (Charging)。 <br> 3. **上报计量**: 桩定时发送 `MeterValues`。 <br> 4. **远程停止**: 后端发送 `RemoteStopTransaction`。 <br> 5. **停止确认**: 桩回复 `RemoteStopTransaction.conf` 并发送 `StatusNotification` (Finishing)。 <br> 6. **交易数据**: 桩发送 `StopTransaction`。 | 1. 模拟桩和后端模拟器在每一步都收到预期的消息。 <br> 2. Kafka 中的事件流与流程完全对应。 <br> 3. 整个流程无错误发生。 |

### 场景 6: 性能与压力

| 用例 ID | 测试描述 | 前置条件 | 测试步骤 | 预期结果 |
| :--- | :--- | :--- | :--- | :--- |
| **TC-E2E-04** | **并发连接**: 模拟大量桩同时在线 | 1. 网关正常部署。 | 1. 启动 1000 个模拟充电桩，并让它们在短时间内全部连接到网关。 <br> 2. 保持连接 10 分钟。 | 1. 所有桩都能成功连接并保持在线。 <br> 2. Gateway Pod 的 CPU 和内存使用率在一个合理的、稳定的范围内。 <br> 3. 无连接异常断开。 |
| **TC-E2E-05** | **高吞吐量**: 模拟消息洪峰 | 1. `TC-E2E-04` 已成功。 | 1. 让 1000 个在线的桩同时高频发送 `MeterValues` 消息。 | 1. 消息处理的端到端延迟（从桩发出到后端收到）在一个可接受的阈值内。 <br>2. Kafka Lag 保持在较低水平。 <br> 3. 系统无崩溃或明显性能下降。 |