# 监控系统 "No Data" 问题排查与解决报告

**[2025-07-17 18:06]**

## 1. 初始状态与核心问题

*   **初始症状**: 在启动了 `test` 环境和 `monitoring` profile 后，登录 Grafana (`http://localhost:3000`)，所有监控面板（包括业务指标和系统指标）均显示 "No data"。
*   **已完成的修复**: 在本次排查会话开始前，一系列应用层和配置层的问题已经被修复，包括：
    *   `gateway-test` 容器的健康检查端口错误。
    *   Prometheus 抓取目标的地址错误。
    *   应用主服务 (`main.go`) 从未启动的核心代码逻辑缺陷。
*   **核心矛盾**: 尽管所有服务看似都已健康运行，数据链路理论上通畅，但最终的监控数据依然缺失。

## 2. 系统化排查过程

我们的排查过程遵循“数据从源头到终端”的思路，逐步验证数据链路的每一环。

*   **阶段一：验证数据源 (Gateway App)**
    *   **[2025-07-17 16:15]** **假设**: 网关应用是否真的在产生 Prometheus 指标？
    *   **验证**: 通过 `curl http://localhost:9091/metrics` 直接访问应用的 metrics 端点。
    *   **发现**: **成功**。应用确实在暴露指标，但关键业务指标 `gateway_active_connections` 的值为 `0`。
    *   **结论**: 问题焦点从“数据链路中断”转移到“数据源没有活动”。

*   **阶段二：生成业务数据**
    *   **[2025-07-17 16:17]** **假设**: 只要有客户端连接到网关，业务指标就会产生。
    *   **验证**: 尝试通过运行测试脚本来模拟客户端连接。
        *   **[2025-07-17 16:22]** **尝试1 (失败)**: `wscat` 命令因未安装而失败。
        *   **[2025-07-17 16:22]** **尝试2 (失败)**: `go test` 命令因在 Windows PowerShell 中使用了不兼容的环境变量设置语法 (`VAR=value ...`) 而失败。
        *   **[2025-07-17 16:23]** **尝试3 (失败)**: 修正语法后，`go test` 命令又因执行目录错误（未在 Go module 根目录下）而失败。
        *   **[2025-07-17 16:27]** **尝试4 (成功)**: 在正确的目录 (`charge-point-gateway`) 下使用正确的 PowerShell 语法，我们成功运行了集成测试，**证明了可以向网关建立连接**。

*   **阶段三：发现终极矛盾**
    *   **[2025-07-17 17:46]** **新症状**: 即便我们成功生成了连接（`gateway_active_connections` 的值变为 `30`），Grafana 依然显示 "No data"。
    *   **验证**: 我们直接在 Prometheus UI (`http://localhost:9090`) 中执行了 Grafana 面板的查询语句 `gateway_active_connections{job="charge-point-gateway"}`。
    *   **[2025-07-17 17:47]** **决定性发现**: **Prometheus UI 中成功返回了值为 `30` 的数据！**
    *   **结论**: 问题被精确锁定在 **Grafana 与 Prometheus 之间**。

*   **阶段四：定位并修复根本原因**
    *   **[2025-07-17 17:47]** **假设**: Grafana 无法从 Prometheus 获取数据，是数据源配置问题。
    *   **验证**:
        1.  检查 Grafana 仪表盘的 JSON 配置，发现它要求数据源的 `uid` 必须是 `"prometheus"`。
        2.  检查 Grafana 的数据源配置文件 (`datasources/prometheus.yml`)，发现其中**没有定义 `uid` 字段**。
    *   **根本原因**: 当通过文件配置 Grafana 数据源时，若不指定 `uid`，Grafana 会为其生成一个随机 `uid`。这导致仪表盘想找的 `uid: "prometheus"` 与实际创建的随机 `uid` 不匹配，从而找不到数据源。
    *   **[2025-07-17 17:48]** **修复**: 修改了 `charge-point-gateway/monitoring/grafana/provisioning/datasources/prometheus.yml` 文件，添加了 `uid: 'prometheus'` 这一行。


