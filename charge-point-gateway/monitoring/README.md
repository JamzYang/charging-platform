# 充电桩网关监控栈

这个监控栈为充电桩网关提供完整的可观测性解决方案，包括指标收集、可视化和告警。

## 🏗️ 架构组件

### 核心监控服务
- **Prometheus** (端口: 9090) - 指标收集和存储
- **Grafana** (端口: 3000) - 数据可视化和仪表板
- **AlertManager** (端口: 9093) - 告警管理

### 指标导出器
- **Node Exporter** (端口: 9100) - 系统指标
- **cAdvisor** (端口: 8080) - 容器指标
- **Redis Exporter** (端口: 9121) - Redis指标
- **Kafka Exporter** (端口: 9308) - Kafka指标

## 🚀 快速开始

### 1. 启动监控栈

**Windows (PowerShell):**
```powershell
cd monitoring
.\start-monitoring.ps1
```

**Linux/macOS:**
```bash
cd monitoring
./start-monitoring.sh start
```

### 2. 访问监控界面

启动成功后，可以通过以下地址访问各个服务：

| 服务 | 地址 | 用户名/密码 | 描述 |
|------|------|-------------|------|
| 📊 Grafana | http://localhost:3000 | admin/admin123 | 主要监控仪表板 |
| 📈 Prometheus | http://localhost:9090 | - | 指标查询和规则管理 |
| 🚨 AlertManager | http://localhost:9093 | - | 告警管理 |
| 💻 Node Exporter | http://localhost:9100 | - | 系统指标 |
| 🐳 cAdvisor | http://localhost:8080 | - | 容器指标 |

### 3. 启动网关应用

确保网关应用正在运行并暴露指标：

```bash
# 在网关项目根目录
go run cmd/gateway/main.go
```

网关指标将在 http://localhost:9090/metrics 暴露。

## 📊 监控指标

### 网关核心指标
- `gateway_active_connections` - 活跃WebSocket连接数
- `gateway_messages_received_total` - 接收消息总数（按OCPP版本和消息类型分类）
- `gateway_events_published_total` - 发布事件总数（按状态分类）
- `gateway_message_processing_duration_seconds` - 消息处理延迟分布

### 系统指标
- CPU使用率
- 内存使用率
- 磁盘使用率
- 网络I/O

### 基础设施指标
- Redis连接状态和性能
- Kafka集群状态和吞吐量
- 容器资源使用情况

## 🚨 告警规则

监控栈包含以下预配置告警：

### 网关告警
- **高连接数告警** - 连接数超过80,000时触发
- **严重连接数告警** - 连接数超过95,000时触发
- **高延迟告警** - 95%分位数处理延迟超过1秒
- **高错误率告警** - 错误率超过5%
- **事件发布失败** - 5分钟内失败超过10次
- **服务不可用** - 网关服务停止响应

### 基础设施告警
- **Redis/Kafka服务不可用**
- **高CPU/内存使用率**
- **磁盘空间不足**

## 🎛️ 管理命令

### Windows (PowerShell)
```powershell
# 启动监控栈
.\start-monitoring.ps1

# 停止监控栈
.\start-monitoring.ps1 -Stop

# 重启监控栈
.\start-monitoring.ps1 -Restart

# 查看服务状态
.\start-monitoring.ps1 -Status

# 查看服务日志
.\start-monitoring.ps1 -Logs
```

### Linux/macOS
```bash
# 启动监控栈
./start-monitoring.sh start

# 停止监控栈
./start-monitoring.sh stop

# 重启监控栈
./start-monitoring.sh restart

# 查看服务状态
./start-monitoring.sh status

# 查看服务日志
./start-monitoring.sh logs
```

## 📈 性能测试监控

在进行性能测试时，建议关注以下关键指标：

### 1. 连接性能
- 活跃连接数趋势
- 连接建立/断开速率
- WebSocket连接稳定性

### 2. 消息处理性能
- 消息接收速率 (TPS)
- 消息处理延迟分布
- 错误率和超时率

### 3. 系统资源
- CPU使用率
- 内存使用率和增长趋势
- 网络带宽使用

### 4. 基础设施性能
- Redis响应时间
- Kafka生产/消费延迟
- 磁盘I/O性能

## 🔧 自定义配置

### 修改Prometheus配置
编辑 `prometheus/prometheus.yml` 来调整：
- 抓取间隔
- 目标服务地址
- 告警规则

### 自定义Grafana仪表板
1. 访问 Grafana (http://localhost:3000)
2. 使用 admin/admin123 登录
3. 导入或创建新的仪表板
4. 配置数据源和查询

### 配置告警通知
编辑 `alertmanager/alertmanager.yml` 来配置：
- 邮件通知
- Webhook集成
- 告警路由规则

## 🐛 故障排除

### 常见问题

1. **服务无法启动**
   - 检查Docker是否运行
   - 确认端口未被占用
   - 查看服务日志

2. **指标数据缺失**
   - 确认网关应用正在运行
   - 检查网关指标端点 (http://localhost:9090/metrics)
   - 验证Prometheus配置

3. **Grafana无法连接Prometheus**
   - 检查数据源配置
   - 确认Prometheus服务正常运行
   - 验证网络连接

### 日志查看
```bash
# 查看所有服务日志
docker-compose -f docker-compose.monitoring.yml logs

# 查看特定服务日志
docker-compose -f docker-compose.monitoring.yml logs prometheus
docker-compose -f docker-compose.monitoring.yml logs grafana
```

## 📝 注意事项

1. **首次启动** - 服务完全启动可能需要1-2分钟
2. **数据持久化** - 监控数据存储在Docker卷中，停止服务不会丢失数据
3. **资源使用** - 监控栈会消耗一定的系统资源，建议在测试环境中预留足够资源
4. **网络配置** - 确保网关应用和监控栈能够相互访问

## 🔗 相关链接

- [Prometheus文档](https://prometheus.io/docs/)
- [Grafana文档](https://grafana.com/docs/)
- [充电桩网关项目文档](../README.md)
