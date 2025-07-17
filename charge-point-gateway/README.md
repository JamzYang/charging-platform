# Charge Point Gateway

高可用充电桩网关系统，基于Kubernetes原生架构设计，支持OCPP协议的大规模充电桩连接管理。

## 功能特性

- 🚀 **高性能**: 支持10万+并发WebSocket连接
- 🔄 **协议支持**: 完整支持OCPP 1.6J协议，预留OCPP 2.0.1扩展
- 📊 **消息处理**: 基于Kafka的异步消息处理，支持10万消息/秒吞吐量
- 💾 **状态管理**: Redis分布式缓存 + 进程内智能缓存
- 🛡️ **高可用**: 无状态设计，支持水平扩展和故障自愈
- 📈 **可观测性**: 完整的监控、日志和分布式追踪

## 架构设计

### 核心组件

- **WebSocket服务器**: 管理充电桩长连接
- **消息分发器**: 根据OCPP版本路由消息
- **协议处理器**: OCPP协议解析和业务转换
- **缓存管理器**: 多级缓存和状态管理
- **消息队列**: Kafka生产者和消费者

### 技术栈

- **语言**: Go 1.21+
- **WebSocket**: gorilla/websocket
- **消息队列**: Apache Kafka (Shopify/sarama)
- **缓存**: Redis (go-redis/redis)
- **配置**: Viper
- **日志**: Zerolog
- **监控**: Prometheus + OpenTelemetry
- **容器**: Docker + Kubernetes

## 快速开始

### 环境要求

- Go 1.21+
- Redis 6.0+
- Apache Kafka 2.8+
- Docker (可选)
- Kubernetes (生产环境)

### 本地开发

1. 克隆项目
```bash
git clone <repository-url>
cd charge-point-gateway
```

2. 安装依赖
```bash
go mod download
```

3. 启动依赖服务
```bash
# 使用Docker Compose启动Redis和Kafka
docker-compose up -d redis kafka
```

4. 配置应用
```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml
# 根据需要修改配置
```

5. 运行应用
```bash
go run cmd/gateway/main.go
```

### WebSocket连接测试

网关启动后，可以通过以下方式测试WebSocket连接：

```bash
# 使用wscat测试连接
wscat -c ws://localhost:8080/ocpp/CP-001 -s ocpp1.6

# 发送BootNotification消息
[2,"msg001","BootNotification",{"chargePointVendor":"TestVendor","chargePointModel":"TestModel"}]

# 检查健康状态
curl http://localhost:8080/health

# 查看连接状态
curl http://localhost:8080/connections
```

### 配置说明

主要配置项说明：

- `server.port`: WebSocket服务端口 (默认: 8080)
- `redis.addr`: Redis服务地址
- `kafka.brokers`: Kafka集群地址
- `cache.max_size`: 进程内缓存最大条目数
- `ocpp.supported_versions`: 支持的OCPP版本

详细配置请参考 `configs/config.yaml`

## 部署

### Docker部署

```bash
# 构建镜像
docker build -t charge-point-gateway:latest .

# 运行容器
docker run -d \
  --name gateway \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/configs:/app/configs \
  charge-point-gateway:latest
```

### Kubernetes部署

```bash
# 应用部署配置
kubectl apply -f deployments/k8s/
```

## 监控

- **健康检查**: `GET /health` (端口: 8081)
- **Prometheus指标**: `GET /metrics` (端口: 9090)
- **性能分析**: `GET /debug/pprof/` (开发环境)

## 开发

### 项目结构

```
charge-point-gateway/
├── cmd/gateway/           # 应用入口
├── internal/              # 内部包
│   ├── config/           # 配置管理
│   ├── domain/           # 领域模型
│   ├── gateway/          # WebSocket服务
│   ├── handler/          # 协议处理器
│   ├── message/          # 消息队列
│   └── storage/          # 存储层
├── pkg/                  # 公共包
├── api/                  # API定义
├── configs/              # 配置文件
├── deployments/          # 部署配置
├── scripts/              # 构建脚本
└── docs/                 # 文档
```

### 代码规范

- 遵循Go官方代码规范
- 使用golangci-lint进行静态检查
- 单元测试覆盖率 > 80%
- 所有公共接口必须有文档注释

## 许可证

[MIT License](LICENSE)
