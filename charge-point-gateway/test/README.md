# 测试目录结构

本目录包含充电桩网关的集成测试和端到端测试。

## 目录结构

```
test/
├── README.md                    # 本文件
├── integration/                 # 集成测试
│   ├── setup/                  # 测试环境设置
│   ├── upstream/               # 上行数据流测试
│   ├── downstream/             # 下行指令流测试
│   └── error_handling/         # 异常处理测试
├── e2e/                        # 端到端测试
│   ├── high_availability/      # 高可用性测试
│   ├── performance/            # 性能测试
│   └── business_flow/          # 完整业务流程测试
├── simulators/                 # 模拟器
│   ├── charge_point/           # 充电桩模拟器
│   └── backend/                # 后端服务模拟器
├── fixtures/                   # 测试数据
│   ├── ocpp_messages/          # OCPP消息样本
│   └── configs/                # 测试配置文件
└── utils/                      # 测试工具
    ├── test_helpers.go         # 测试辅助函数
    ├── docker_compose.go       # Docker环境管理
    └── assertions.go           # 自定义断言
```

## 测试分类

### 集成测试 (Integration Tests)
- 验证Gateway Pod内部各模块的交互
- 验证与外部依赖（Kafka, Redis）的集成
- 测试用例ID: TC-INT-01 到 TC-INT-07

### 端到端测试 (E2E Tests)  
- 验证完整的业务流程
- 测试高可用性和故障转移
- 性能和压力测试
- 测试用例ID: TC-E2E-01 到 TC-E2E-05

## 运行测试

### 前置条件
- Docker 和 Docker Compose
- Go 1.21+
- 可用的Redis和Kafka实例

### 快速开始

#### 1. 使用测试脚本（推荐）
```bash
# 运行所有测试
./test/run_tests.sh

# 只运行单元测试（不需要Docker）
./test/run_tests.sh --no-docker unit

# 运行集成测试
./test/run_tests.sh integration

# 运行端到端测试
./test/run_tests.sh e2e

# 运行性能测试
./test/run_tests.sh performance

# 启用调试工具运行测试
./test/run_tests.sh --debug e2e

# 清理环境后运行测试
./test/run_tests.sh -c all

# 只启动测试环境（不运行测试）
./test/run_tests.sh -s

# 停止测试环境
./test/run_tests.sh -d
```

#### 2. 使用Makefile
```bash
# 运行单元测试
make test

# 运行集成测试
make test-integration

# 运行端到端测试
make test-e2e

# 运行性能测试
make test-performance

# 运行所有测试
make test-all

# 生成详细测试报告
make test-report
```

#### 3. 手动运行
```bash
# 启动测试环境
docker-compose -f test/docker-compose.test.yml up -d

# 等待服务启动
sleep 30

# 运行特定测试
go test -v ./test/integration/upstream/...
go test -v ./test/e2e/high_availability/...

# 停止测试环境
docker-compose -f test/docker-compose.test.yml down -v
```

## 测试环境

### Docker服务
测试使用Docker Compose启动以下服务：

| 服务 | 端口 | 描述 |
|------|------|------|
| Redis | 6380 | 缓存和连接映射存储 |
| Kafka | 9093 | 消息队列 |
| Zookeeper | 2182 | Kafka依赖 |
| Gateway | 8081 | 网关服务 |
| Metrics | 9091 | Prometheus指标 |
| Health Check | 8083 | 健康检查接口 |

### 调试工具（可选）
启用 `--debug` 标志时可用：

| 工具 | 端口 | 描述 |
|------|------|------|
| Kafka UI | 8082 | Kafka管理界面 |
| Redis Commander | 8084 | Redis管理界面 |

### 配置文件
- `test/fixtures/configs/test_config.yaml` - 基础测试配置
- `test/fixtures/configs/integration_test_config.yaml` - 集成测试配置
- `test/docker-compose.test.yml` - Docker Compose配置

### 测试数据
测试数据文件位于 `test/fixtures/ocpp_messages/` 目录：
- `boot_notification.json` - BootNotification消息样本
- `meter_values.json` - MeterValues消息样本
- `status_notification.json` - StatusNotification消息样本
- `remote_start_transaction.json` - RemoteStartTransaction消息样本
