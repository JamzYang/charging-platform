# **充电站运营平台 (Station Service) V1.0 开发计划**

**版本：1.0**  
**日期：2025-07-25**  
**基于文档：**
- 《产品需求文档 (PRD)：充电站运营平台 V1.0》
- 《场站域服务 (Station Service) 详细开发设计文档 V1.0》

---

## 1. 项目概述

本项目旨在开发**充电站运营平台 (Station Service)** 的V1.0版本。此版本聚焦于构建平台的**核心数据底座和后端服务能力**，目标是管理5000个充电站和10万个充电桩的静态信息与动态状态。核心任务包括：打通与充电桩网关的数据链路，实现资产的数字化管理、充电桩状态的实时同步，并为上层业务（如订单服务）提供稳定、可靠的资产查询和设备远程控制API。

**主要技术栈:** Java 21, Spring Boot 3, DDD, PostgreSQL, Redis, Kafka。

---

## 2. 核心里程碑 (Key Milestones)

| 里程碑 | 计划完成时间 | 关键目标 |
| :--- | :--- | :--- |
| **M1** | 项目启动后第 1 周结束 | **核心框架与资产管理API**：项目基础框架搭建完毕，充电站和充电桩的CRUD API开发完成，可供前端或测试团队联调。 |
| **M2** | 项目启动后第 3 周结束 | **上行数据链路打通**：成功消费网关上报的Kafka事件，实现充电桩状态的实时数据库同步和缓存更新。 |
| **M3** | 项目启动后第 5 周结束 | **下行指令链路打通**：实现远程启停充电等核心控制API，完成“查Redis->算分区->发Kafka”的完整指令下发流程。 |
| **M4** | 项目启动后第 6 周结束 | **系统稳定与预发布**：完成所有功能的集成测试、异常处理优化和性能基准测试，系统达到预发布状态。 |

---

## 3. 开发阶段与任务分解

### **阶段一：项目初始化与核心框架搭建 (预计 1 周)**

**目标：** 搭建一个稳定、规范、可扩展的DDD项目骨架，为后续功能开发奠定坚实基础。

| 子任务 ID | 子任务描述 | 负责人 | 预估工时 | 依赖项 | 完成标准 (Definition of Done) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1.1** | **项目脚手架创建** | 后端团队 | 0.5 PD | - | 1. 使用Gradle创建新的Spring Boot 3.3+项目。 <br> 2. 核心依赖（`spring-boot-starter-web`, `spring-boot-starter-data-jpa`, `lombok`）已添加。 <br> 3. 项目可成功编译并启动，`actuator/health`端点可访问。 |
| **1.2** | **搭建DDD分层结构** | 后端团队 | 1 PD | 1.1 | 1. `interfaces`, `application`, `domain`, `infrastructure` 四层包结构已按设计文档建立。 <br> 2. 在各层包下创建`package-info.java`，说明该层职责。 <br> 3. 共享模块`shared`已创建，用于存放`exception`, `util`, `constant`。 |
| **1.3** | **配置与集成 (DB, Redis, Kafka)** | 后端团队 | 1.5 PD | 1.1 | 1. **数据库:** `application.yml`中数据源(PostgreSQL)配置完成，应用启动时能成功连接。 <br> 2. **缓存:** Redis依赖和配置完成，`StringRedisTemplate` Bean可被注入和使用。 <br> 3. **消息队列:** Kafka依赖和配置完成，`KafkaTemplate` Bean可被注入。 |
| **1.4** | **实现全局通用组件** | 后端团队 | 1 PD | 1.2 | 1. 全局异常处理器`GlobalExceptionHandler`已实现，能捕获`BusinessException`和`Exception`。 <br> 2. 统一API响应对象`ApiResult`已定义并应用。 <br> 3. Logback日志配置完成，能区分`INFO`, `WARN`, `ERROR`级别输出。 <br> 4. 基础安全配置`SecurityConfig`已添加，暂时允许所有请求访问。 |
| **1.5** | **数据库初始化** | 后端团队 | 1 PD | 1.3 | 1. 使用Flyway或Liquibase管理数据库版本。 <br> 2. `stations`, `charge_points`, `connectors` 和 `outbox_events` 表的初始化SQL脚本已编写并提交。 <br> 3. 应用启动时，数据库表结构能自动创建或更新。 |

---

### **阶段二：充电资产管理 (CRUD) (预计 1 周)**

**目标：** 实现充电站和充电桩的完整生命周期管理，为系统提供基础数据。

| 子任务 ID | 子任务描述 | 负责人 | 预估工时 | 依赖项 | 完成标准 (Definition of Done) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **2.1** | **领域模型与仓储 (Station & ChargePoint)** | Dev-A | 2 PD | 1.2, 1.5 | 1. `Station`和`ChargePoint`聚合根、值对象（如`Location`, `DeviceStatus`）已按设计文档定义。 <br> 2. `StationRepository`和`ChargePointRepository`接口及其JPA实现已完成。 <br> 3. 单元测试覆盖聚合根的核心业务规则（如状态变更逻辑）。 |
| **2.2** | **应用服务 (Station & ChargePoint)** | Dev-A | 1 PD | 2.1 | 1. `StationApplicationService`和`ChargePointApplicationService`已创建。 <br> 2. 实现了资产的创建、修改、查询的业务流程编排。 <br> 3. 事务注解`@Transactional`已在公开方法上正确使用。 |
| **2.3** | **接口层 (Controller & DTO)** | Dev-A | 1.5 PD | 2.2 | 1. `StationController`和`ChargePointController`已创建。 <br> 2. `CreateStationRequest`, `StationResponse`等DTO对象已定义，并使用`Record`和`@Valid`注解。 <br> 3. `StationAssembler`等DTO转换器已实现。 <br> 4. API已通过Postman或单元测试验证，符合API设计文档。 |
| **2.4** | **缓存实现 (Cache-Aside)** | Dev-A | 0.5 PD | 2.3 | 1. 查询充电站/桩详情时，优先从Redis获取。 <br> 2. 缓存未命中则查询数据库，并将结果写入缓存。 <br> 3. 资产信息更新或删除时，能正确清除（evict）相关缓存。 |

---

### **阶段三：上行数据链路 (状态同步) (预计 2 周)**

**目标：** 实时消费网关上报的业务事件，将物理世界的设备状态变更精确同步到平台。

| 子任务 ID | 子任务描述 | 负责人 | 预估工时 | 依赖项 | 完成标准 (Definition of Done) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **3.1** | **事务性发件箱模式实现** | Dev-B | 2 PD | 1.5, 2.1 | 1. `DomainEventPublisher`和`OutboxEventRelay`已按设计文档实现。 <br> 2. 业务操作（如`chargePoint.updateStatus`）与`OutboxEvent`的创建在同一数据库事务中。 <br> 3. `OutboxEventRelay`能通过`@Scheduled`任务轮询并发布事件到Kafka，并处理并发（使用分布式锁或数据库锁）。 |
| **3.2** | **Kafka消费者与事件处理器** | Dev-B | 2 PD | 2.1 | 1. `DeviceEventConsumer`已创建，监听`ocpp-events-up` Topic。 <br> 2. `DeviceStatusEventHandler`已实现，能正确解析网关上报的`StatusNotificationEvent`等事件。 <br> 3. 消费者具备幂等性，重复消费同一消息不会导致数据状态错误（可通过数据库唯一约束或业务逻辑检查实现）。 |
| **3.3** | **状态更新与领域事件发布** | Dev-B | 1.5 PD | 3.1, 3.2 | 1. `DeviceStatusEventHandler`在处理事件后，能调用`ChargePoint`聚合根的`updateStatus`方法。 <br> 2. 状态变更后，`ChargePoint`聚合根能生成`ChargePointStatusChangedEvent`领域事件。 <br> 3. 该领域事件通过发件箱模式被可靠地发布出去（用于未来可能的订阅者）。 |
| **3.4** | **集成测试：上行链路** | QA/Dev-B | 2 PD | 3.3 | 1. 编写集成测试，模拟向`ocpp-events-up` Topic发送消息。 <br> 2. **验证：** 数据库中充电桩的状态、最后心跳时间等字段被正确更新。 <br> 3. **验证：** Redis中的充电桩状态缓存被清除或更新。 <br> 4. **验证：** `outbox_events`表中产生了对应的`ChargePointStatusChangedEvent`记录并被成功发布。 |

---

### **阶段四：下行指令链路 (设备控制) (预计 2 周)**

**目标：** 为上层服务提供稳定、异步的设备控制API，并确保指令能被精确路由到目标网关。

| 子任务 ID | 子任务描述 | 负责人 | 预估工时 | 依赖项 | 阶段二, 1.3 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **4.1** | **实现`GatewayCommandRouter`** | Dev-A | 2 PD | 1.3 | 1. `GatewayCommandRouter`类已创建。 <br> 2. `sendCommand`方法实现了完整的“**查Redis->算分区->发Kafka**”逻辑。 <br> 3. Redis Key (`conn:{chargePointId}`) 和分区计算逻辑与网关团队约定一致。 <br> 4. 单元测试覆盖分区计算的正确性和设备离线时的异常处理。 |
| **4.2** | **设备控制应用服务** | Dev-A | 1.5 PD | 4.1 | 1. `DeviceControlApplicationService`已创建。 <br> 2. `startCharging`, `stopCharging`等方法已实现。 <br> 3. 在发送指令前，对充电桩状态进行前置检查（如启动充电前检查桩是否`Available`）。 <br> 4. 调用`GatewayCommandRouter`发送指令，并同步返回“指令已接收”响应。 |
| **4.3** | **设备控制API接口** | Dev-A | 1 PD | 4.2 | 1. `DeviceControlController`已创建，并定义`POST /api/v1/piles/{pileId}/commands/start-transaction`等API端点。 <br> 2. API能正确处理设备离线或状态不符的场景，返回4xx错误码。 <br> 3. API调用是异步的，立即返回`CommandAcceptedResponse`。 |
| **4.4** | **指令响应处理 (可选, V1.1)** | Dev-B | 1.5 PD | 3.2 | 1. 在`DeviceEventConsumer`中增加对`command.response`事件的处理逻辑。 <br> 2. 创建`CommandResponseEventHandler`，用于更新指令的最终执行状态（如持久化到`command_log`表）。 <br> 3. （可选）当指令被拒绝时，通过发件箱发布集成事件，通知相关方。 |
| **4.5** | **集成测试：下行链路** | QA/Dev-A | 2 PD | 4.3 | 1. 编写集成测试，调用设备控制API。 <br> 2. **验证：** `GatewayCommandRouter`能正确从模拟的Redis中读取连接信息。 <br> 3. **验证：** 使用嵌入式Kafka (Embedded Kafka) 验证指令消息被发送到了**正确的分区**和**正确的Topic** (`commands-down`)。 <br> 4. **验证：** API在各种前置条件下（桩离线、状态错误）的行为符合预期。 |

---

### **阶段五：系统集成、测试与交付 (预计 1 周)**

**目标：** 确保系统整体功能的正确性、稳定性和性能，准备交付。

| 子任务 ID | 子任务描述 | 负责人 | 预估工时 | 依赖项 | 完成标准 (Definition of Done) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **5.1** | **完善API文档** | 后端团队 | 1 PD | 阶段四 | 1. 使用SpringDoc或Swagger为所有公开API生成OpenAPI 3.0文档。 <br> 2. 文档中包含清晰的请求/响应示例和错误码说明。 |
| **5.2** | **端到端(E2E)测试** | QA/全体 | 2 PD | 阶段四 | 1. 与网关、订单服务等进行联调测试。 <br> 2. 覆盖核心业务场景：新桩入网 -> 状态上报 -> 远程启动充电 -> 状态变更 -> 远程停止充电。 |
| **5.3** | **性能基准测试** | 后端团队 | 1 PD | 阶段四 | 1. 对核心API（查桩、启停）进行压力测试。 <br> 2. 测试上行事件处理的延迟，确保P95延迟在PRD要求的2秒以内。 |
| **5.4** | **部署与交付** | 运维/后端 | 1 PD | 5.1, 5.2, 5.3 | 1. 编写Dockerfile和Kubernetes部署文件（Deployment, Service, ConfigMap）。 <br> 2. 在测试环境中成功部署并验证。 <br> 3. 交付完整的部署手册和API文档。 |
