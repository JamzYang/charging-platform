# 性能瓶颈排查报告：解决并发连接数1k上限问题

## 1. 问题背景

在性能测试中，尝试建立10,000个并发WebSocket连接，但监控显示实际活动连接数始终被精确地限制在1,000（1k）左右，无法进一步提升。

## 2. 初步排查总结 (由开发团队完成)

在深入调试前，开发团队已经完成了大量出色的初步排查工作，为后续定位问题奠定了坚实的基础。主要包括：

*   **应用层排查**:
    *   确认测试代码 (`concurrent_connections_test.go`) 逻辑无误。
    *   确认应用配置 (`application.yaml`) 中 `max_connections` 设置充足。
    *   移除了测试客户端的大量日志以减轻I/O瓶颈。
*   **环境与架构排查**:
    *   修复了因 `/metrics` 路由重复注册导致的网关启动 `panic`。
    *   通过 `docker-compose.test.yml` 提高了网关容器的文件描述符限制 (`ulimit`)。
    *   **关键决策**：将测试客户端容器化 (`Dockerfile.testclient`)，并与被测服务置于同一Docker网络中，极大地排除了宿主机和网络NAT层的干扰。

尽管做了以上努力，连接数瓶颈依然存在，问题指向了更底层的环境或配置。

## 3. 深入调试与根因定位 (由故障排查专家执行)

基于已有信息，我们进行了一系列假设驱动的系统性排查。

### [2025-07-18 12:42] 第一轮诊断：定位客户端`ulimit`瓶颈

*   **分析**: 审查 `docker-compose.test.yml` 和 `Dockerfile.testclient`。
*   **发现**: `gateway-test` 服务已设置高 `ulimit`，但 `test-client` 服务**没有**进行相关配置。在多数Docker环境中，容器默认的文件描述符限制为1024。
*   **假设**: 瓶颈在于测试客户端容器的文件描述符上限。
*   **修复**: 在 `docker-compose.test.yml` 中为 `test-client` 服务添加与 `gateway-test` 相同的 `ulimits` 配置。
    ```yaml
    test-client:
      # ...
      ulimits:
        nofile:
          soft: 120000
          hard: 120000
    ```

### [2025-07-18 12:45] 第二轮诊断：定位服务启动时序问题

*   **症状**: 解除客户端 `ulimit` 限制后，测试结果从“限制在1k”恶化为“0连接成功”，所有连接尝试均报 `websocket: bad handshake` 错误。
*   **分析**: 状况的恶化表明解除一个限制后触发了新的、更严重的问题。`bad handshake` 错误在高并发场景下通常意味着服务器不堪重负或尚未就绪。
*   **假设**: **服务未就绪**。`docker-compose` 的 `depends_on` 只保证容器启动顺序，不保证容器内应用已完成初始化。`test-client` 启动过快，在 `gateway-test` 应用准备好接受连接前就发起了请求。
*   **修复**: 修改 `docker-compose.test.yml`，为 `test-client` 添加一个更健壮的启动命令，利用 `curl` 轮询 `gateway-test` 的 `/health` 端点，确保服务完全就绪后再执行测试。同时，修正了健康检查错用 `8081` 端口的问题，应为 `8080`。
    ```yaml
    test-client:
      # ...
      command: >
        sh -c "echo 'Waiting for gateway to be fully ready...' && 
               apk add --no-cache curl &&
               until curl -f http://gateway-test:8080/health; do echo '...'; sleep 2; done &&
               echo 'Gateway is ready, starting test.' &&
               /app/performance.test -test.v -test.run TestTC_E2E_04_ConcurrentConnections"
    ```

### [2025-07-18 01:03] 第三轮诊断：定位服务器端硬编码限制

*   **症状**: 解决了时序问题后，测试日志显示前1000个连接成功，但从第1001个连接开始，所有连接均报 `websocket: bad handshake` 错误。这精确地复现了最初的1k限制，但现在是在服务器端。
*   **分析**: 这种精确的限制强烈暗示应用程序内部存在一个硬性上限。
最终发现 manager.go 中的 `MaxConnections` 设置为1000，导致了这个限制。