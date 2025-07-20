# 充电桩网关测试环境

这个目录包含了充电桩网关的完整测试环境，支持可选的监控服务。

## 🚀 快速开始

### Windows (PowerShell)

```powershell
# 启动基础测试环境
.\start-test-env.ps1

# 启动测试环境 + 监控服务
.\start-test-env.ps1 -WithMonitoring

# 重新构建并启动
.\start-test-env.ps1 -WithMonitoring -Build

# 查看状态
.\start-test-env.ps1 -Status

# 停止环境
.\start-test-env.ps1 -Stop

# 重启环境
.\start-test-env.ps1 -Restart -WithMonitoring
```

### Linux/macOS (Bash)

```bash
# 给脚本执行权限
chmod +x start-test-env.sh

# 启动基础测试环境
./start-test-env.sh

# 启动测试环境 + 监控服务
./start-test-env.sh --with-monitoring

# 重新构建并启动
./start-test-env.sh --with-monitoring --build

# 查看状态
./start-test-env.sh --status

# 停止环境
./start-test-env.sh --stop

# 重启环境
./start-test-env.sh --restart --with-monitoring
```

## 🏗️ 服务架构

### 基础服务 (默认启动)
- **gateway-test**: 充电桩网关服务
- **redis-test**: Redis缓存服务
- **kafka-test**: Kafka消息队列
- **zookeeper-test**: Zookeeper (Kafka依赖)

### 监控服务 (可选启动)
- **prometheus**: 指标收集和存储
- **grafana**: 数据可视化和仪表板
- **alertmanager**: 告警管理
- **node-exporter**: 系统指标导出器
- **cadvisor**: 容器指标导出器
- **redis-exporter**: Redis指标导出器
- **kafka-exporter**: Kafka指标导出器

### 调试工具 (可选启动)
```bash
# 启动Kafka UI和Redis Commander
docker-compose -f docker-compose.test.yml --profile debug up -d
```

## 🌐 服务访问地址

### 网关服务
- **WebSocket连接**: `ws://localhost:8081/ocpp/{charge_point_id}`
- **健康检查**: http://localhost:8081/health
- **Metrics指标**: http://localhost:9091/metrics

### 基础设施
- **Redis**: localhost:6379
- **Kafka**: localhost:9092
- **Zookeeper**: localhost:2182

### 监控服务 (使用 -WithMonitoring 启动时)
- **Grafana**: http://localhost:3000 (admin/admin123)
- **Prometheus**: http://localhost:9090
- **AlertManager**: http://localhost:9093
- **Node Exporter**: http://localhost:9100
- **cAdvisor**: http://localhost:8080
- **Redis Metrics**: http://localhost:9121
- **Kafka Metrics**: http://localhost:9308

### 调试工具 (使用 --profile debug 启动时)
- **Kafka UI**: http://localhost:8082
- **Redis Commander**: http://localhost:8084

## 📊 Docker Compose Profiles

这个配置使用了Docker Compose的profiles功能来组织服务：

- **默认**: 基础测试环境 (gateway, redis, kafka, zookeeper)
- **monitoring**: 监控服务 (prometheus, grafana, exporters等)
- **debug**: 调试工具 (kafka-ui, redis-commander)

## 🔧 手动操作

如果你想手动控制服务，可以直接使用docker-compose命令：

```bash
# 启动基础环境
docker-compose -f docker-compose.test.yml up -d

# 启动基础环境 + 监控
docker-compose -f docker-compose.test.yml --profile monitoring up -d

# 启动所有服务（包括调试工具）
docker-compose -f docker-compose.test.yml --profile monitoring --profile debug up -d

# 查看服务状态
docker-compose -f docker-compose.test.yml --profile monitoring ps

# 查看日志
docker-compose -f docker-compose.test.yml --profile monitoring logs -f

# 停止所有服务
docker-compose -f docker-compose.test.yml --profile monitoring --profile debug down
```

## 🧪 测试用例

启动环境后，你可以运行各种测试：

```bash
# 单元测试
go test ./...

# 集成测试
go test -tags=integration ./test/...

# E2E测试
go test -tags=e2e ./test/...

# 性能测试
go test -tags=performance ./test/...
```

## 📝 注意事项

1. **端口冲突**: 确保相关端口没有被其他服务占用
2. **资源要求**: 监控服务会消耗额外的CPU和内存资源
3. **数据持久化**: 使用Docker volumes保存数据，停止服务不会丢失数据
4. **网络隔离**: 所有服务运行在独立的Docker网络中

## 🔍 故障排除

### 服务启动失败
```bash
# 查看服务日志
docker-compose -f docker-compose.test.yml logs [service-name]

# 检查端口占用
netstat -an | grep [port-number]
```

### 健康检查失败
```bash
# 手动测试健康检查
curl http://localhost:8081/health

# 检查容器内部
docker exec gateway-test curl http://localhost:8080/health
```

### 监控数据不显示
1. 确认Prometheus能访问目标服务
2. 检查Grafana数据源配置
3. 验证网络连通性
